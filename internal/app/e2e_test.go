package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kk-2004/klaude/internal/filesystem"
	"github.com/kk-2004/klaude/internal/model"
	"github.com/kk-2004/klaude/internal/model/fake"
	"github.com/kk-2004/klaude/internal/storage"
	"log/slog"
)

// TestDesktopBoundaryFlow exercises the Wails-bound service contract without
// requiring a GUI/WebView in CI. The same RPC methods are what the generated
// Wails bindings call, while the deterministic provider keeps the flow
// network-free and reproducible.
func TestDesktopBoundaryFlow(t *testing.T) {
	ctx := context.Background()
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.Startup(ctx)
	defer service.Shutdown(ctx)

	gitRoot := t.TempDir()
	if err := exec.Command("git", "-C", gitRoot, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	gitProject, err := service.OpenProject(ctx, gitRoot)
	if err != nil || gitProject.GitRoot == "" {
		t.Fatalf("git project=%+v err=%v", gitProject, err)
	}
	nonGitProject, err := service.OpenProject(ctx, t.TempDir())
	if err != nil || nonGitProject.GitRoot != "" {
		t.Fatalf("non-git project=%+v err=%v", nonGitProject, err)
	}

	session, err := service.CreateSession(ctx, gitProject.ID, "Acceptance", "fake", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	fixture := fake.NewCodingTurn()
	stream, err := fixture.Stream(ctx, structRequest())
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
		// The fake emits text, a read call, usage, and an approved patch call.
	}

	path := filepath.Join(gitRoot, "README.md")
	if err := os.WriteFile(path, []byte("# Klaude\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	turn, err := service.SendMessage(ctx, session.ID, "apply the reviewed patch", "fake", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	call := storage.ToolCall{ID: storage.NewID(), TurnID: turn.ID, Name: "apply_patch", Arguments: "{}", Status: storage.ToolCompleted}
	if err := service.db.AddToolCall(ctx, call); err != nil {
		t.Fatal(err)
	}
	workspace, _ := filesystem.New(gitRoot)
	snapshots, _ := filesystem.NewSnapshotStore(dirs.Snapshots)
	change, err := workspace.WriteFile(snapshots, "README.md", []byte("# Klaude\n\nA local-first coding agent.\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.db.AddFileChange(ctx, storage.FileChange{TurnID: turn.ID, ToolCallID: call.ID, Path: change.Path, Status: change.Status, BeforeHash: change.BeforeHash, AfterHash: change.AfterHash, Diff: change.Diff, AddedLines: change.AddedLines, DeletedLines: change.DeletedLines}); err != nil {
		t.Fatal(err)
	}
	if changes, err := service.GetTurnChanges(ctx, turn.ID); err != nil || len(changes) != 1 {
		t.Fatalf("changes=%+v err=%v", changes, err)
	}
	if err := service.UndoTurn(ctx, turn.ID); err != nil {
		t.Fatal(err)
	}
}

func structRequest() model.Request { return model.Request{Model: "fixture"} }
