package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUndoIsAllOrNothingAndDetectsDivergence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	_ = os.WriteFile(path, []byte("old"), 0o600)
	service, _ := New(root)
	store, _ := NewSnapshotStore(filepath.Join(t.TempDir(), "snapshots"))
	change, err := service.WriteFile(store, "file.txt", []byte("new"), "")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(path, []byte("user edit"), 0o600)
	if err := service.Undo(store, []Change{change}); err != ErrUndoConflict {
		t.Fatalf("undo error = %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "user edit" {
		t.Fatalf("conflict overwrote user edit: %q", data)
	}
	_ = os.WriteFile(path, []byte("new"), 0o600)
	if err := service.Undo(store, []Change{change}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "old" {
		t.Fatalf("undo content = %q", data)
	}
}
