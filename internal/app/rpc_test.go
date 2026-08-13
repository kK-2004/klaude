package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDialogDirectoryUsesExistingDirectory(t *testing.T) {
	directory := t.TempDir()
	want, err := filepath.Abs(directory)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolveDialogDirectory(directory); got != want {
		t.Fatalf("resolveDialogDirectory() = %q, want %q", got, want)
	}
}

func TestResolveDialogDirectoryFallsBackFromMissingDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if got := resolveDialogDirectory(missing); got != home {
		t.Fatalf("resolveDialogDirectory() = %q, want home %q", got, home)
	}
}

func TestResolveDialogDirectoryExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got := resolveDialogDirectory("~"); got != home {
		t.Fatalf("resolveDialogDirectory() = %q, want %q", got, home)
	}
}
