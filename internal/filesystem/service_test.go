package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Resolve("link/secret.txt", false); err != ErrOutsideWorkspace {
		t.Fatalf("resolve error = %v, want %v", err, ErrOutsideWorkspace)
	}
}

func TestListDirectoryIsDeterministic(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := service.ListDirectory(context.Background(), ".")
	if err != nil || len(entries) != 2 || entries[0].Name != "a" {
		t.Fatalf("entries = %+v, err=%v", entries, err)
	}
}
