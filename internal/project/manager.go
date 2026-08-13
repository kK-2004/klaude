package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/klaude/klaude/internal/storage"
)

type Manager struct {
	DB *storage.DB
}

func NewManager(db *storage.DB) *Manager { return &Manager{DB: db} }

// Open 将路径规范为绝对、解 symlink 后的目录；已登记则返回，否则创建 Project 并探测 GitRoot。
func (m *Manager) Open(ctx context.Context, input string) (storage.Project, error) {
	root, err := canonicalRoot(input)
	if err != nil {
		return storage.Project{}, err
	}
	project, err := m.DB.GetProjectByRoot(ctx, root)
	if err == nil {
		return project, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storage.Project{}, err
	}
	project = storage.Project{ID: storage.NewID(), Name: filepath.Base(root), RootPath: root, GitRoot: discoverGitRoot(ctx, root)}
	if err := m.DB.CreateProject(ctx, project); err != nil {
		return storage.Project{}, err
	}
	return project, nil
}

// Reveal 在系统文件管理器中打开目录：macOS 用 Finder，Windows 用资源管理器，其余走 xdg-open。
func Reveal(ctx context.Context, path string) error {
	root, err := canonicalRoot(path)
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "open", root).Run()
	case "windows":
		// explorer.exe 打开成功时也可能返回非零退出码，因此忽略退出状态。
		_ = exec.CommandContext(ctx, "explorer", filepath.FromSlash(root)).Run()
		return nil
	default:
		return exec.CommandContext(ctx, "xdg-open", root).Run()
	}
}

func canonicalRoot(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", errors.New("project path is empty")
	}
	absolute, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve project symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat project path: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("project path is not a directory")
	}
	if info.Mode().Perm()&0o400 == 0 {
		return "", errors.New("project path is not readable")
	}
	return filepath.Clean(resolved), nil
}

func discoverGitRoot(ctx context.Context, root string) string {
	command := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	gitRoot, err := filepath.Abs(strings.TrimSpace(string(output)))
	if err != nil {
		return ""
	}
	return filepath.Clean(gitRoot)
}

type Capability struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Detail    string `json:"detail"`
}

// ProbeCapabilities 探测 git/rg 是否在 PATH，以及模型凭证环境变量是否可用。
func ProbeCapabilities(ctx context.Context, cfgCredentialEnv string) []Capability {
	capabilities := make([]Capability, 0, 3)
	for _, name := range []string{"git", "rg"} {
		path, err := exec.LookPath(name)
		if err != nil {
			capabilities = append(capabilities, Capability{Name: name, Detail: "Install " + name + " to enable this capability"})
			continue
		}
		capabilities = append(capabilities, Capability{Name: name, Available: true, Detail: path})
	}
	if cfgCredentialEnv == "" {
		capabilities = append(capabilities, Capability{Name: "model", Detail: "Configure a credential environment variable"})
	} else if value, ok := os.LookupEnv(cfgCredentialEnv); !ok || strings.TrimSpace(value) == "" {
		capabilities = append(capabilities, Capability{Name: "model", Detail: "Credential environment variable is missing"})
	} else {
		capabilities = append(capabilities, Capability{Name: "model", Available: true, Detail: "Credential is available in memory"})
	}
	_ = ctx
	return capabilities
}
