package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kk-2004/klaude/internal/event"
	"github.com/kk-2004/klaude/internal/filesystem"
	"github.com/kk-2004/klaude/internal/storage"
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

func TestServiceSendMessageEmitsFailureWhenRunnerCannotStart(t *testing.T) {
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.Startup(context.Background())
	defer service.Shutdown(context.Background())
	project, err := service.OpenProject(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), project.ID, "Runtime error", "fake", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	runtimeErr := errors.New("credential is missing")
	service.SetTurnRunner(func(context.Context, string, string) error { return runtimeErr })
	events := make(chan event.Type, 1)
	service.composition.Events.Subscribe(func(_ context.Context, envelope event.Envelope) error {
		if envelope.Type == event.AgentError {
			events <- envelope.Type
		}
		return nil
	})
	turn, err := service.SendMessage(context.Background(), session.ID, "hello", "fake", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("runner failure did not emit an agent error event")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		turns, listErr := service.db.ListTurns(context.Background(), session.ID)
		if listErr == nil {
			for _, stored := range turns {
				if stored.ID == turn.ID && stored.Status == storage.TurnFailed {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	turns, err := service.db.ListTurns(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range turns {
		if stored.ID == turn.ID {
			t.Fatalf("turn status = %q, want failed", stored.Status)
		}
	}
	t.Fatalf("turn %q was not persisted", turn.ID)
}

func TestServiceDeleteProjectRemovesProjectAndSessions(t *testing.T) {
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.Startup(context.Background())
	defer service.Shutdown(context.Background())

	project, err := service.OpenProject(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), project.ID, "To delete", "fake", "fake-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteProject(context.Background(), project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.db.GetProject(context.Background(), project.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted project lookup error = %v, want %v", err, sql.ErrNoRows)
	}
	if _, err := service.db.GetSession(context.Background(), session.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted session lookup error = %v, want %v", err, sql.ErrNoRows)
	}
}

func TestServiceDeleteProjectWaitsForActiveRuntime(t *testing.T) {
	dirs := storage.NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	service := NewServiceWithDataDirs(slog.Default(), dirs)
	service.Startup(context.Background())
	defer service.Shutdown(context.Background())
	project, err := service.OpenProject(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateSession(context.Background(), project.ID, "Active", "fake", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	exited := make(chan struct{})
	service.SetTurnRunner(func(ctx context.Context, _, _ string) error {
		close(started)
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	})
	if _, err := service.SendMessage(context.Background(), session.ID, "hello", "fake", "fixture"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runtime did not start")
	}
	if err := service.DeleteProject(context.Background(), project.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("DeleteProject returned before runtime exited")
	}
	if _, err := service.db.GetProject(context.Background(), project.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted project lookup error = %v, want %v", err, sql.ErrNoRows)
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
