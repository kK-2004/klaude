package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kk-2004/klaude/internal/filesystem"
)

type memRecorder struct{ changes []filesystem.Change }

func (m *memRecorder) Record(_ context.Context, change filesystem.Change) error {
	m.changes = append(m.changes, change)
	return nil
}

func TestWriteFileAndApplyPatchTools(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace, err := filesystem.New(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := filesystem.NewSnapshotStore(filepath.Join(t.TempDir(), "snap"))
	if err != nil {
		t.Fatal(err)
	}
	recorder := &memRecorder{}
	ctx := WriteContext{Workspace: workspace, Snapshots: snapshots, Recorder: recorder, MaxOutput: 4000}
	registry := NewRegistry()
	if err := RegisterMutating(registry, ctx, nil); err != nil {
		t.Fatal(err)
	}

	writeTool, _ := registry.Get("write_file")
	result, err := writeTool.Execute(context.Background(), json.RawMessage(`{"path":"main.go","content":"package main\n\nfunc main() {}\n"}`))
	if err != nil || !result.Success {
		t.Fatalf("write: %+v err=%v", result, err)
	}
	if len(recorder.changes) != 1 {
		t.Fatalf("recorded = %d", len(recorder.changes))
	}

	patchTool, _ := registry.Get("apply_patch")
	result, err = patchTool.Execute(context.Background(), json.RawMessage(`{"path":"main.go","old_text":"func main() {}","new_text":"func main() { println(1) }"}`))
	if err != nil || !result.Success {
		t.Fatalf("patch: %+v err=%v", result, err)
	}

	result, err = patchTool.Execute(context.Background(), json.RawMessage(`{"path":"main.go","old_text":"missing","new_text":"x"}`))
	if result.ErrorCode != "stale_baseline" {
		t.Fatalf("stale = %+v err=%v", result, err)
	}

	result, err = writeTool.Execute(context.Background(), json.RawMessage(`{"path":"../escape.go","content":"x"}`))
	if result.ErrorCode != "workspace_boundary" {
		t.Fatalf("boundary = %+v err=%v", result, err)
	}
}
