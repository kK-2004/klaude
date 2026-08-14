package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func TestListRecentSessionsReturnsGlobalLatestTen(t *testing.T) {
	db, ctx := openTestDB(t)
	now := time.Now().UTC()
	projects := []Project{
		{ID: NewID(), Name: "one", RootPath: t.TempDir(), CreatedAt: now, UpdatedAt: now},
		{ID: NewID(), Name: "two", RootPath: t.TempDir(), CreatedAt: now, UpdatedAt: now},
	}
	for _, project := range projects {
		if err := db.CreateProject(ctx, project); err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < 12; index++ {
		project := projects[index%len(projects)]
		created := now.Add(time.Duration(index) * time.Second)
		if err := db.CreateSession(ctx, Session{ID: NewID(), ProjectID: project.ID, Title: fmt.Sprintf("session-%02d", index), Provider: "fake", Model: "fixture", CreatedAt: created, UpdatedAt: created}); err != nil {
			t.Fatal(err)
		}
	}

	recent, err := db.ListRecentSessions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 10 {
		t.Fatalf("recent session count = %d, want 10", len(recent))
	}
	if recent[0].Title != "session-11" || recent[9].Title != "session-02" {
		t.Fatalf("recent sessions are not globally sorted and limited: first=%q last=%q", recent[0].Title, recent[9].Title)
	}
	if recent[0].ProjectID == recent[1].ProjectID {
		t.Fatalf("recent sessions should include sessions from multiple projects: %+v", recent[:2])
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

func TestProjectPinningOrdersListAndRenameDeletePersist(t *testing.T) {
	db, ctx := openTestDB(t)
	now := time.Now().UTC()
	older := Project{ID: NewID(), Name: "older", RootPath: t.TempDir(), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	newer := Project{ID: NewID(), Name: "newer", RootPath: t.TempDir(), CreatedAt: now, UpdatedAt: now}
	for _, item := range []Project{older, newer} {
		if err := db.CreateProject(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SetProjectPinned(ctx, older.ID, true); err != nil {
		t.Fatal(err)
	}
	projects, err := db.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].ID != older.ID || !projects[0].Pinned {
		t.Fatalf("pinned project must sort first, got %+v", projects)
	}
	if err := db.RenameProject(ctx, older.ID, "renamed"); err != nil {
		t.Fatal(err)
	}
	renamed, err := db.GetProject(ctx, older.ID)
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "renamed" || !renamed.Pinned {
		t.Fatalf("rename must keep pin state, got %+v", renamed)
	}
	if err := db.DeleteProject(ctx, older.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetProject(ctx, older.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted project lookup error = %v, want %v", err, sql.ErrNoRows)
	}
	if err := db.DeleteProject(ctx, older.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("repeated delete error = %v, want %v", err, sql.ErrNoRows)
	}
}

func TestDeleteProjectCascadesSessionsAndMoveSessionRebindsProject(t *testing.T) {
	db, ctx := openTestDB(t)
	source := Project{ID: NewID(), Name: "source", RootPath: t.TempDir()}
	target := Project{ID: NewID(), Name: "target", RootPath: t.TempDir()}
	for _, item := range []Project{source, target} {
		if err := db.CreateProject(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	draft := Session{ID: NewID(), ProjectID: source.ID, Title: "draft", Provider: "fake", Model: "fake-1"}
	if err := db.CreateSession(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if err := db.MoveSession(ctx, draft.ID, target.ID); err != nil {
		t.Fatal(err)
	}
	moved, err := db.GetSession(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ProjectID != target.ID {
		t.Fatalf("session project = %q, want %q", moved.ProjectID, target.ID)
	}
	if err := db.DeleteProject(ctx, target.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetSession(ctx, draft.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("session must be removed with its project, error = %v", err)
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
