package storage

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const currentSchemaVersion = 2

var ErrReadOnly = errors.New("storage: database is read-only")
var ErrActiveTurn = errors.New("storage: session already has an active turn")
var ErrNewerSchema = errors.New("storage: database schema is newer than this application")

type DB struct {
	SQL      *sql.DB
	Path     string
	ReadOnly bool
}

// Open 打开/创建 SQLite：收紧文件权限、配置 WAL/外键，并执行嵌入式迁移。
// 若库 schema 比应用新，则以只读模式返回（不写迁移）。
func Open(ctx context.Context, path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Stat(path); err == nil && info.Mode().Perm()&0o077 != 0 {
		_ = os.Chmod(path, 0o600)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &DB{SQL: db, Path: path}
	if err := store.configure(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(ctx); err != nil {
		if errors.Is(err, ErrNewerSchema) {
			store.ReadOnly = true
			return store, nil
		}
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (d *DB) configure(ctx context.Context) error {
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := d.SQL.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure sqlite: %w", err)
		}
	}
	return nil
}

func (d *DB) Close() error { return d.SQL.Close() }

func (d *DB) WriteTx(ctx context.Context, fn func(*sql.Tx) error) error {
	if d.ReadOnly {
		return ErrReadOnly
	}
	tx, err := d.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (d *DB) migrate(ctx context.Context) error {
	if _, err := d.SQL.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	var version int
	if err := d.SQL.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		return err
	}
	if version > currentSchemaVersion {
		return ErrNewerSchema
	}
	for next := version + 1; next <= currentSchemaVersion; next++ {
		if err := d.applyMigration(ctx, next); err != nil {
			return fmt.Errorf("apply migration %d: %w", next, err)
		}
	}
	return nil
}

// applyMigration 在同一事务内执行 SQL 文件并写入 schema_migrations，保证可回滚。
func (d *DB) applyMigration(ctx context.Context, version int) error {
	matches, err := fs.Glob(migrationFiles, fmt.Sprintf("migrations/%03d_*.sql", version))
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf("expected one migration file for version %d, found %d", version, len(matches))
	}
	sqlBytes, err := fs.ReadFile(migrationFiles, matches[0])
	if err != nil {
		return err
	}
	return d.WriteTx(ctx, func(tx *sql.Tx) error {
		for _, statement := range strings.Split(string(sqlBytes), ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)", version, time.Now().UnixMilli())
		return err
	})
}

func (d *DB) Backup() (string, error) {
	if d.Path == "" {
		return "", errors.New("storage: database path is empty")
	}
	backup := d.Path + ".backup-" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	if err := copyFile(d.Path, backup); err != nil {
		return "", err
	}
	return backup, nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Sync()
}
