package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePatchAndUndoPreconditions(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("package main\nold\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewSnapshotStore(filepath.Join(t.TempDir(), "snapshots"))
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Capture(path)
	if err != nil {
		t.Fatal(err)
	}
	change, err := service.ApplyPatch(store, "main.go", "old", "new", before.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if change.Status != "modified" || change.AfterHash == change.BeforeHash || !strings.Contains(change.Diff, "-old\n+new") {
		t.Fatalf("change = %+v", change)
	}
	if _, err := service.ApplyPatch(store, "main.go", "new", "other", before.Hash); err != ErrStaleBaseline {
		t.Fatalf("stale patch error = %v", err)
	}
	if err := store.Restore(change.Before, path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "package main\nold\n" {
		t.Fatalf("restored = %q", data)
	}
}

func TestNewFileSnapshotAndNoChange(t *testing.T) {
	root := t.TempDir()
	service, _ := New(root)
	store, _ := NewSnapshotStore(filepath.Join(t.TempDir(), "snapshots"))
	change, err := service.WriteFile(store, "new.txt", []byte("hello"), "")
	if err != nil {
		t.Fatal(err)
	}
	if change.Status != "added" || change.Before.Exists {
		t.Fatalf("change = %+v", change)
	}
	noChange, err := service.WriteFile(store, "new.txt", []byte("hello"), change.AfterHash)
	if err != nil {
		t.Fatal(err)
	}
	if noChange.Status != "no_change" {
		t.Fatalf("no change = %+v", noChange)
	}
}

func TestAmbiguousPatchRejected(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	_ = os.WriteFile(path, []byte("x\nx\n"), 0o600)
	service, _ := New(root)
	store, _ := NewSnapshotStore(filepath.Join(t.TempDir(), "snapshots"))
	if _, err := service.ApplyPatch(store, "file.txt", "x", "y", ""); err != ErrAmbiguousPatch {
		t.Fatalf("error = %v", err)
	}
}
