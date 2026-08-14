//go:build windows

package sandbox_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/klaude/klaude/internal/sandbox"
)

func TestWinACLProbeAndPrepare(t *testing.T) {
	backend := sandbox.Platform()
	if backend.Name() != "winacl" {
		t.Fatalf("name=%s", backend.Name())
	}
	ok, enforcement, err := backend.Probe(context.Background())
	if err != nil || !ok {
		t.Skipf("winacl probe unavailable: %v", err)
	}
	if enforcement != sandbox.EnforcementPartial {
		t.Fatalf("enforcement=%s", enforcement)
	}
	builder, okCB := backend.(sandbox.CommandBuilder)
	if !okCB {
		t.Fatal("winacl must implement CommandBuilder")
	}
	dir := t.TempDir()
	inside := filepath.Join(dir, "ok.txt")
	cmd, confined, err := builder.PrepareCommand(context.Background(), "cmd.exe", []string{"/C", "echo hi>" + inside}, dir, os.Environ(), sandbox.Policy{
		Mode: sandbox.ModeWorkspaceWrite, WorkspaceRoot: dir, SessionID: "win-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer confined.Release()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("inside write failed: %v %s", err, output)
	}
	if _, err := os.Stat(inside); err != nil {
		t.Fatal(err)
	}

	outsideDir := filepath.Join(os.TempDir(), "..", "klaude-winacl-outside")
	if home, err := os.UserHomeDir(); err == nil {
		outsideDir = filepath.Join(home, "AppData", "Local", "klaude-winacl-test")
	}
	_ = os.MkdirAll(outsideDir, 0o700)
	outside := filepath.Join(outsideDir, "deny.txt")
	_ = os.Remove(outside)
	defer os.RemoveAll(outsideDir)

	cmd, confined, err = builder.PrepareCommand(context.Background(), "cmd.exe", []string{"/C", "echo hi>" + outside}, dir, os.Environ(), sandbox.Policy{
		Mode: sandbox.ModeWorkspaceWrite, WorkspaceRoot: dir, SessionID: "win-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer confined.Release()
	_, _ = cmd.CombinedOutput()
	if _, err := os.Stat(outside); err == nil {
		t.Fatalf("outside write succeeded under winacl: %s", outside)
	}
}
