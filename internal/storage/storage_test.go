package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) (*DB, context.Context) {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "nested", "klaude.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, context.Background()
}

func TestDataDirsEnsureUsesRestrictivePermissions(t *testing.T) {
	dirs := NewDataDirs(filepath.Join(t.TempDir(), "profile"))
	if err := dirs.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dirs.Base, dirs.Traces, dirs.Logs, dirs.Cache, dirs.Snapshots} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Fatalf("%s permissions = %o, want 700", path, got)
		}
	}
}

func TestMigrationsCreateRelationalSchema(t *testing.T) {
	db, ctx := openTestDB(t)
	var version int
	if err := db.SQL.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, currentSchemaVersion)
	}
	var foreignKeys int
	if err := db.SQL.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatal("foreign keys are disabled")
	}
}

func TestCreateTurnWithUserMessageIsAtomicAndEnforcesOneActiveTurn(t *testing.T) {
	db, ctx := openTestDB(t)
	now := time.Now().UTC()
	project := Project{ID: NewID(), Name: "demo", RootPath: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := db.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	session := Session{ID: NewID(), ProjectID: project.ID, Title: "Debug", Provider: "fake", Model: "fake-1"}
	if err := db.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	message, turn, err := db.CreateTurnWithUserMessage(ctx, session.ID, "Inspect this", session.Provider, session.Model)
	if err != nil {
		t.Fatal(err)
	}
	if message.TurnID != turn.ID {
		t.Fatalf("message turn = %q, turn = %q", message.TurnID, turn.ID)
	}
	if _, _, err := db.CreateTurnWithUserMessage(ctx, session.ID, "Again", session.Provider, session.Model); err != ErrActiveTurn {
		t.Fatalf("second active turn error = %v, want %v", err, ErrActiveTurn)
	}
	var messages int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if messages != 1 {
		t.Fatalf("message count = %d, want 1", messages)
	}
}

func TestRecoveryMarksActiveWorkWithoutReplay(t *testing.T) {
	db, ctx := openTestDB(t)
	now := time.Now().UTC()
	project := Project{ID: NewID(), Name: "demo", RootPath: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	if err := db.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	session := Session{ID: NewID(), ProjectID: project.ID, Title: "Debug", Provider: "fake", Model: "fake-1"}
	if err := db.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	_, turn, err := db.CreateTurnWithUserMessage(ctx, session.ID, "Inspect this", session.Provider, session.Model)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RecoverInFlight(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.SQL.QueryRowContext(ctx, `SELECT status FROM agent_turns WHERE id=?`, turn.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(TurnInterrupted) {
		t.Fatalf("turn status = %q, want interrupted", status)
	}
}

func TestReadOnlyDBRejectsWrites(t *testing.T) {
	db, ctx := openTestDB(t)
	db.ReadOnly = true
	if err := db.SetSetting(ctx, Setting{Key: "x", ValueJSON: `1`}); err != ErrReadOnly {
		t.Fatalf("write error = %v, want %v", err, ErrReadOnly)
	}
}

func TestForeignKeyConstraint(t *testing.T) {
	db, ctx := openTestDB(t)
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO sessions(id,project_id,title,provider,model,status,created_at,updated_at) VALUES('orphan','missing','x','x','x','idle',1,1)`)
		return err
	})
	if err == nil {
		t.Fatal("expected foreign key error")
	}
}
