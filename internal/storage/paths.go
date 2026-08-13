package storage

import (
	"os"
	"path/filepath"
)

// DataDirs 描述本地数据布局：库、轨迹、日志、缓存与文件快照根目录。
type DataDirs struct {
	Base      string
	Database  string
	Traces    string
	Logs      string
	Cache     string
	Snapshots string
}

func DefaultDataDirs() (DataDirs, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return DataDirs{}, err
	}
	return NewDataDirs(filepath.Join(base, "Klaude")), nil
}

func NewDataDirs(base string) DataDirs {
	return DataDirs{
		Base:      base,
		Database:  filepath.Join(base, "klaude.db"),
		Traces:    filepath.Join(base, "traces"),
		Logs:      filepath.Join(base, "logs"),
		Cache:     filepath.Join(base, "cache"),
		Snapshots: filepath.Join(base, "snapshots"),
	}
}

func (d DataDirs) Ensure() error {
	// 目录权限固定 0700，避免用户态配置/轨迹被同机其他用户读取。
	for _, path := range []string{d.Base, d.Traces, d.Logs, d.Cache, d.Snapshots} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return err
		}
	}
	return nil
}
