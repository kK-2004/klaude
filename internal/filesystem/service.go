package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrOutsideWorkspace = errors.New("filesystem: target is outside the workspace")

// Service 把所有路径解析限制在工作区 Root 内，防止符号链接/绝对路径越界。
type Service struct{ Root string }

func New(root string) (Service, error) {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Service{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return Service{}, errors.New("filesystem: workspace root is not a directory")
	}
	return Service{Root: filepath.Clean(resolved)}, nil
}

// Resolve 规范化目标路径并校验仍在工作区内。
// forWrite=true 时，对尚不存在的文件用其父目录做 symlink 解析。
func (s Service) Resolve(target string, forWrite bool) (string, error) {
	if s.Root == "" {
		return "", errors.New("filesystem: workspace root is empty")
	}
	if target == "" {
		target = "."
	}
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.Root, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	check := path
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) && forWrite {
		check = filepath.Dir(path)
	}
	resolved, err := filepath.EvalSymlinks(check)
	if err != nil {
		return "", fmt.Errorf("resolve target: %w", err)
	}
	if !inside(s.Root, resolved) {
		return "", ErrOutsideWorkspace
	}
	if check != path {
		return path, nil
	}
	return resolved, nil
}

func (s Service) ReadFile(ctx context.Context, target string) ([]byte, error) {
	path, err := s.Resolve(target, false)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return os.ReadFile(path)
}

type Entry struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	Dir             bool   `json:"dir"`
	Size            int64  `json:"size"`
	ExternalSymlink bool   `json:"externalSymlink"`
}

func (s Service) ListDirectory(ctx context.Context, target string) ([]Entry, error) {
	path, err := s.Resolve(target, false)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		item := Entry{Name: entry.Name(), Path: filepath.Join(target, entry.Name())}
		info, err := entry.Info()
		if err == nil {
			item.Dir, item.Size = info.IsDir(), info.Size()
		}
		// 指向工作区外的 symlink 标 ExternalSymlink，UI 可提示风险。
		if entry.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(filepath.Join(path, entry.Name()))
			item.ExternalSymlink = err != nil || !inside(s.Root, resolved)
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}

func inside(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

var _ fs.FileInfo
