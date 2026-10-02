// Package sqlstore implements storage.Store over database/sql for two dialects:
// SQLite (modernc.org/sqlite, pure Go, used for local dev and tests) and PostgreSQL (pgx).
//
// Queries are written once with "?" placeholders and rebound to "$N" for PostgreSQL.
// Schema differences are isolated in migrations/<dialect>/*.sql.
package sqlstore

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver
	_ "modernc.org/sqlite"             // registers "sqlite" driver

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

//go:embed migrations
var migrationsFS embed.FS

// Dialect of the SQL database.
type Dialect string

const (
	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
)

// Store is the database/sql backed implementation of storage.Store.
type Store struct {
	db      *sql.DB
	dialect Dialect
	kind    string
}

// OpenSQLite opens (or creates) a SQLite database. path ":memory:" gives a fresh in-memory DB.
//
// SQLite allows a single writer; we keep exactly one open connection, which serializes all
// access through database/sql. That is plenty for a household app and avoids SQLITE_BUSY.
func OpenSQLite(ctx context.Context, path string) (*Store, error) {
	kind := "sqlite"
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	if path == ":memory:" {
		kind = "memory"
		dsn = "file::memory:?_pragma=foreign_keys(1)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	s := &Store{db: db, dialect: SQLite, kind: kind}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// OpenPostgres connects to PostgreSQL via pgx and applies migrations.
func OpenPostgres(ctx context.Context, url string) (*Store, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	s := &Store{db: db, dialect: Postgres, kind: "postgres"}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Kind returns "sqlite", "memory" or "postgres".
func (s *Store) Kind() string { return s.kind }

// Ping checks the connection.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Close closes the pool.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle (used by tests and reset tooling).
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	dir := "migrations/" + string(s.dialect)
	entries, err := fs.ReadDir(migrationsFS, dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.SplitN(name, "_", 2)[0]
		var exists int
		err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`), version).Scan(&exists)
		if err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		body, err := migrationsFS.ReadFile(dir + "/" + name)
		if err != nil {
			return err
		}
		err = s.inTx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return fmt.Errorf("apply %s: %w", name, err)
			}
			_, err := tx.ExecContext(ctx, s.q(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`), version, time.Now().UTC().Format(time.RFC3339))
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// q rebinds "?" placeholders to "$N" for PostgreSQL. Queries in this package never contain
// literal question marks inside string constants, so a simple scan is enough.
func (s *Store) q(query string) string {
	if s.dialect != Postgres {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// NewID returns a random UUID string.
func NewID() string { return uuid.NewString() }

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	m := err.Error()
	return strings.Contains(m, "UNIQUE constraint") || strings.Contains(m, "duplicate key") || strings.Contains(m, "SQLSTATE 23505")
}

func isFKViolation(err error) bool {
	if err == nil {
		return false
	}
	m := err.Error()
	return strings.Contains(m, "FOREIGN KEY constraint") || strings.Contains(m, "SQLSTATE 23503")
}
