package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/klaude/klaude/internal/config"
	"github.com/klaude/klaude/internal/storage"
)

func TestNewProjectToolsRegistersReadAndWrite(t *testing.T) {
	composition, err := Build(context.Background(), storage.NewDataDirs(filepath.Join(t.TempDir(), "profile")), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer composition.Close()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tools, err := composition.NewProjectTools(root, "turn-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"read_file", "write_file", "apply_patch", "shell", "grep"} {
		if _, ok := tools.Registry.Get(name); !ok {
			t.Fatalf("missing tool %s", name)
		}
	}
	meta := tools.Lookup.Lookup("read_file")
	if !meta.Known || !meta.Concurrent || !meta.ReadOnly {
		t.Fatalf("read meta = %+v", meta)
	}
	writeMeta := tools.Lookup.Lookup("write_file")
	if !writeMeta.Known || writeMeta.ReadOnly || writeMeta.Concurrent {
		t.Fatalf("write meta = %+v", writeMeta)
	}
	cfg := composition.SchedulerConfigFromApp()
	if cfg.ParallelTools || cfg.LLMSchedule {
		t.Fatalf("default scheduler cfg = %+v", cfg)
	}
}

func TestUpdateSettingsPersistsParallelFlags(t *testing.T) {
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	composition, err := Build(context.Background(), dirs, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer composition.Close()
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.composition = composition
	service.config = composition.Config
	service.data = dirs
	service.db = composition.DB

	cfg, err := service.UpdateSettings(context.Background(), SettingsUpdate{ParallelTools: true, LLMSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Agent.ParallelTools || !cfg.Agent.LLMSchedule {
		t.Fatalf("updated = %+v", cfg.Agent)
	}
	reloaded := config.Load(config.UserConfigPath(dirs.Base), "")
	if !reloaded.Config.Agent.ParallelTools || !reloaded.Config.Agent.LLMSchedule {
		t.Fatalf("reloaded = %+v", reloaded.Config.Agent)
	}
	cfg, err = service.UpdateSettings(context.Background(), SettingsUpdate{ParallelTools: false, LLMSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.ParallelTools || cfg.Agent.LLMSchedule {
		t.Fatalf("llm should require parallel: %+v", cfg.Agent)
	}
}
