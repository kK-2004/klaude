package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/klaude/klaude/internal/filesystem"
	"github.com/klaude/klaude/internal/storage"
)

func TestServiceLifecycle(t *testing.T) {
	service := NewService(slog.Default())
	if got := service.Health(); got.Ready {
		t.Fatal("service should start as not ready")
	}
	service.Startup(context.Background())
	if got := service.Health(); !got.Ready || got.Product != "Klaude" {
		t.Fatalf("unexpected ready health: %+v", got)
	}
	service.Shutdown(context.Background())
	if got := service.Health(); got.Ready {
		t.Fatal("service should be stopped")
	}
}

func TestServiceProjectSessionAndSendMessage(t *testing.T) {
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.Startup(context.Background())
	defer service.Shutdown(context.Background())
	projectRoot := t.TempDir()
	project, err := service.OpenProject(context.Background(), projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), project.ID, "Test", "fake", "fake-1")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.SendMessage(context.Background(), session.ID, "Inspect", "fake", "fake-1")
	if err != nil || turn.SessionID != session.ID {
		t.Fatalf("turn=%+v err=%v", turn, err)
	}
}

func TestServiceSendMessageCommitsBeforeRunnerAndSnapshot(t *testing.T) {
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.Startup(context.Background())
	defer service.Shutdown(context.Background())
	root := t.TempDir()
	project, err := service.OpenProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), project.ID, "Snapshot", "fake", "fake-1")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan error, 1)
	service.SetTurnRunner(func(ctx context.Context, sessionID, turnID string) error {
		_, err := service.db.GetSession(ctx, sessionID)
		if err != nil {
			started <- err
			return err
		}
		_, err = service.LoadConversation(ctx, sessionID)
		started <- err
		return err
	})
	turn, err := service.SendMessage(context.Background(), session.ID, "hello", "fake", "fake-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := <-started; err != nil {
		t.Fatalf("runner observed uncommitted state: %v", err)
	}
	snapshot, err := service.LoadConversation(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].TurnID != turn.ID {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestServiceUndoTurnRestoresPersistedChange(t *testing.T) {
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.Startup(context.Background())
	defer service.Shutdown(context.Background())
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.OpenProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), project.ID, "Test", "fake", "fake-1")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.SendMessage(context.Background(), session.ID, "change", "fake", "fake-1")
	if err != nil {
		t.Fatal(err)
	}
	call := storage.ToolCall{ID: storage.NewID(), TurnID: turn.ID, Name: "write_file", Arguments: `{"path":"main.go"}`, Status: storage.ToolRunning}
	if err := service.db.AddToolCall(context.Background(), call); err != nil {
		t.Fatal(err)
	}
	workspace, _ := filesystem.New(root)
	snapshots, _ := filesystem.NewSnapshotStore(dirs.Snapshots)
	change, err := workspace.WriteFile(snapshots, "main.go", []byte("new\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.db.AddFileChange(context.Background(), storage.FileChange{TurnID: turn.ID, ToolCallID: call.ID, Path: change.Path, Status: change.Status, BeforeHash: change.BeforeHash, AfterHash: change.AfterHash, Diff: change.Diff, AddedLines: change.AddedLines, DeletedLines: change.DeletedLines}); err != nil {
		t.Fatal(err)
	}
	if err := service.UndoTurn(context.Background(), turn.ID); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "old\n" {
		t.Fatalf("restored content = %q", data)
	}
}
