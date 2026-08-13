package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/klaude/klaude/internal/storage"
)

func TestOpenCanonicalizesAndDeduplicatesProject(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "klaude.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := t.TempDir()
	manager := NewManager(db)
	first, err := manager.Open(context.Background(), filepath.Join(root, "."))
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.RootPath != second.RootPath {
		t.Fatalf("projects were not deduplicated: %+v %+v", first, second)
	}
}

func TestOpenRejectsFileAndSymlinkEscape(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "klaude.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := NewManager(db)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Open(context.Background(), file); err == nil {
		t.Fatal("expected file path error")
	}
}
