package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStatusAndDiffUseExplicitGitCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX git shell semantics")
	}
	root := t.TempDir()
	for _, args := range [][]string{{"init", root}, {"-C", root, "config", "user.email", "test@example.com"}, {"-C", root, "config", "user.name", "Test"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New(root)
	if _, err := service.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Diff(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestBranchWorkflowUsesRealLocalAndRemoteRefs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX git shell semantics")
	}
	root := initRepository(t)
	runGit(t, "-C", root, "branch", "feature/local")
	runGit(t, "-C", root, "branch", "feature/remote")
	remote := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "clone", "--bare", root, remote)
	runGit(t, "-C", root, "remote", "add", "origin", remote)
	runGit(t, "-C", root, "fetch", "origin")

	service := New(root)
	snapshot, err := service.Branches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Current != "main" || !containsBranch(snapshot.Branches, "feature/local", false) || !containsBranch(snapshot.Branches, "origin/feature/remote", true) {
		t.Fatalf("unexpected branch snapshot: %+v", snapshot)
	}

	runGit(t, "-C", root, "branch", "-D", "feature/local")
	if err := service.CheckoutBranch(context.Background(), "origin/feature/local", true); err != nil {
		t.Fatal(err)
	}
	if current, err := service.Branch(context.Background()); err != nil || current != "feature/local\n" {
		t.Fatalf("current=%q err=%v", current, err)
	}
	runGit(t, "-C", root, "switch", "main")
	if err := service.DeleteBranch(context.Background(), "feature/local", false); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteBranch(context.Background(), "origin/feature/remote", true); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "--git-dir", remote, "show-ref", "--verify", "refs/heads/feature/remote").CombinedOutput(); err == nil {
		t.Fatalf("remote branch still exists: %s", output)
	}
}

func TestCreateWorktreeCreatesAndChecksOutNamedBranch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX git shell semantics")
	}
	root := initRepository(t)
	target := filepath.Join(t.TempDir(), "feature-worktree")
	created, err := New(root).CreateWorktree(context.Background(), "main", "feature/worktree", target)
	if err != nil {
		t.Fatal(err)
	}
	if created != target {
		t.Fatalf("created path = %q", created)
	}
	output := runGit(t, "-C", target, "branch", "--show-current")
	if output != "feature/worktree\n" {
		t.Fatalf("worktree branch = %q", output)
	}
}

func TestBranchesIncludesCurrentUnbornBranch(t *testing.T) {
	root := t.TempDir()
	runGit(t, "init", root)
	runGit(t, "-C", root, "branch", "-M", "main")
	snapshot, err := New(root).Branches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Current != "main" || !containsBranch(snapshot.Branches, "main", false) {
		t.Fatalf("unexpected unborn branch snapshot: %+v", snapshot)
	}
}

func initRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, "init", root)
	runGit(t, "-C", root, "config", "user.email", "test@example.com")
	runGit(t, "-C", root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, "-C", root, "add", "README.md")
	runGit(t, "-C", root, "commit", "-m", "initial")
	runGit(t, "-C", root, "branch", "-M", "main")
	return root
}

func runGit(t *testing.T, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, output)
	}
	return string(output)
}
