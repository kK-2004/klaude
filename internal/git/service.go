package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kk-2004/klaude/internal/executor"
)

var ErrNotRepository = errors.New("git: not a repository")

// Service 封装对指定仓库的 git CLI 调用（经 executor），供分支/worktree UI 使用。
type Service struct {
	Root     string
	Executor executor.Local
}

type BranchInfo struct {
	Name    string `json:"name"`
	Remote  bool   `json:"remote"`
	Current bool   `json:"current"`
}

type BranchSnapshot struct {
	Current      string       `json:"current"`
	Branches     []BranchInfo `json:"branches"`
	WorktreeBase string       `json:"worktreeBase"`
}

func New(root string) Service { return Service{Root: root} }
func (s Service) Status(ctx context.Context) (string, error) {
	return s.run(ctx, "status", "--porcelain=v1", "-b")
}
func (s Service) Diff(ctx context.Context) (string, error) {
	return s.run(ctx, "diff", "--no-ext-diff", "--no-color")
}
func (s Service) Branch(ctx context.Context) (string, error) {
	return s.run(ctx, "branch", "--show-current")
}

// Branches 列出本地与远端分支，并附带建议的 worktree 父目录（仓库父路径）。
func (s Service) Branches(ctx context.Context) (BranchSnapshot, error) {
	output, err := s.run(ctx, "for-each-ref", "--sort=refname", "--format=%(refname)%09%(HEAD)%09%(symref)", "refs/heads", "refs/remotes")
	if err != nil {
		return BranchSnapshot{}, err
	}
	current, err := s.Branch(ctx)
	if err != nil {
		return BranchSnapshot{}, err
	}
	current = strings.TrimSpace(current)
	branches := make([]BranchInfo, 0)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 || strings.TrimSpace(fields[2]) != "" {
			continue
		}
		ref := strings.TrimSpace(fields[0])
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			name := strings.TrimPrefix(ref, "refs/heads/")
			branches = append(branches, BranchInfo{Name: name, Current: name == current || strings.TrimSpace(fields[1]) == "*"})
		case strings.HasPrefix(ref, "refs/remotes/"):
			name := strings.TrimPrefix(ref, "refs/remotes/")
			if !strings.HasSuffix(name, "/HEAD") {
				branches = append(branches, BranchInfo{Name: name, Remote: true})
			}
		}
	}
	if current != "" && !containsBranch(branches, current, false) {
		branches = append([]BranchInfo{{Name: current, Current: true}}, branches...)
	}
	return BranchSnapshot{Current: current, Branches: branches, WorktreeBase: filepath.Dir(s.Root)}, nil
}

// CheckoutBranch：本地直接 switch；远端若已有同名本地分支则切过去，否则 --track 新建跟踪分支。
func (s Service) CheckoutBranch(ctx context.Context, name string, remote bool) error {
	snapshot, err := s.Branches(ctx)
	if err != nil {
		return err
	}
	if !containsBranch(snapshot.Branches, name, remote) {
		return fmt.Errorf("git: branch %q was not found", name)
	}
	if !remote {
		_, err = s.run(ctx, "switch", name)
		return err
	}
	localName, ok := remoteLocalName(name)
	if !ok {
		return fmt.Errorf("git: invalid remote branch %q", name)
	}
	if containsBranch(snapshot.Branches, localName, false) {
		_, err = s.run(ctx, "switch", localName)
	} else {
		_, err = s.run(ctx, "switch", "--track", name)
	}
	return err
}

func (s Service) DeleteBranch(ctx context.Context, name string, remote bool) error {
	snapshot, err := s.Branches(ctx)
	if err != nil {
		return err
	}
	if !containsBranch(snapshot.Branches, name, remote) {
		return fmt.Errorf("git: branch %q was not found", name)
	}
	if !remote {
		_, err = s.run(ctx, "branch", "-d", name)
		return err
	}
	remoteName, branchName, ok := strings.Cut(name, "/")
	if !ok || remoteName == "" || branchName == "" {
		return fmt.Errorf("git: invalid remote branch %q", name)
	}
	_, err = s.run(ctx, "push", remoteName, "--delete", branchName)
	return err
}

// CreateWorktree：分支已存在则 worktree add；否则 -b 从 startRef 新建分支并检出到 target。
func (s Service) CreateWorktree(ctx context.Context, startRef, branchName, targetPath string) (string, error) {
	snapshot, err := s.Branches(ctx)
	if err != nil {
		return "", err
	}
	if !containsBranchName(snapshot.Branches, startRef) {
		return "", fmt.Errorf("git: start branch %q was not found", startRef)
	}
	if _, err := s.run(ctx, "check-ref-format", "--branch", branchName); err != nil {
		return "", fmt.Errorf("git: invalid worktree branch %q: %w", branchName, err)
	}
	if strings.TrimSpace(targetPath) == "" {
		return "", errors.New("git: worktree path is empty")
	}
	target, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("git: resolve worktree path: %w", err)
	}
	target = filepath.Clean(target)
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", fmt.Errorf("git: resolve repository path: %w", err)
	}
	if target == filepath.Clean(root) {
		return "", errors.New("git: worktree path must differ from the repository path")
	}
	if info, statErr := os.Stat(filepath.Dir(target)); statErr != nil || !info.IsDir() {
		return "", errors.New("git: worktree parent directory does not exist")
	}
	if containsBranch(snapshot.Branches, branchName, false) {
		_, err = s.run(ctx, "worktree", "add", target, branchName)
	} else {
		_, err = s.run(ctx, "worktree", "add", "-b", branchName, target, startRef)
	}
	if err != nil {
		return "", err
	}
	return target, nil
}

func containsBranch(branches []BranchInfo, name string, remote bool) bool {
	for _, branch := range branches {
		if branch.Name == name && branch.Remote == remote {
			return true
		}
	}
	return false
}

func containsBranchName(branches []BranchInfo, name string) bool {
	for _, branch := range branches {
		if branch.Name == name {
			return true
		}
	}
	return false
}

func remoteLocalName(name string) (string, bool) {
	_, branch, ok := strings.Cut(name, "/")
	return branch, ok && branch != ""
}

func (s Service) run(ctx context.Context, args ...string) (string, error) {
	if strings.TrimSpace(s.Root) == "" {
		return "", ErrNotRepository
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", err
	}
	result, err := s.Executor.Execute(ctx, executor.Request{Program: "git", Args: append([]string{"-C", s.Root}, args...), WorkingDir: s.Root, Timeout: 30 * time.Second, MaxOutput: 100_000})
	if err != nil {
		if strings.Contains(strings.ToLower(result.Stderr), "not a git repository") {
			return "", ErrNotRepository
		}
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			message = err.Error()
		}
		return result.Stderr, fmt.Errorf("git %s: %s", args[0], message)
	}
	return result.Stdout, nil
}
