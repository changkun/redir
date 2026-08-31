// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed all:migrations/*.sql
var migrationFS embed.FS

// migrateLockKey is an arbitrary but fixed key for the advisory lock that
// serialises migration across instances. Two redir processes starting at
// the same time must not both apply the same file.
const migrateLockKey int64 = 0x7265646972 // "redir"

// parseMigrationName splits a file name into its version and label. A
// name that does not parse is an error rather than a file to skip: a
// skipped migration leaves the schema incomplete and says nothing.
func parseMigrationName(name string) (version int64, label string, ok bool) {
	prefix, rest, found := strings.Cut(name, "_")
	if !found {
		return 0, "", false
	}
	v, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil {
		return 0, "", false
	}
	return v, rest, true
}

// migration is one numbered file from the migrations directory.
type migration struct {
	version int64
	name    string
	stmts   string
}

// loadMigrations reads the embedded files and returns them in version
// order. A file whose name does not start with a number is a mistake that
// would otherwise be skipped silently, so it is an error.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("cannot read migrations: %w", err)
	}

	ms := make([]migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		v, _, ok := parseMigrationName(e.Name())
		if !ok {
			return nil, fmt.Errorf(
				"migration %v: want <version>_<name>.sql with a numeric version",
				e.Name())
		}
		b, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("migration %v: %w", e.Name(), err)
		}
		ms = append(ms, migration{version: v, name: e.Name(), stmts: string(b)})
	}
	slices.SortFunc(ms, func(a, b migration) int {
		return int(a.version - b.version)
	})

	for i := 1; i < len(ms); i++ {
		if ms[i].version == ms[i-1].version {
			return nil, fmt.Errorf("migrations %v and %v share version %d",
				ms[i-1].name, ms[i].name, ms[i].version)
		}
	}
	return ms, nil
}

// migrate applies every migration the database has not seen yet.
//
// The schema cannot come from the postgres image's docker-entrypoint-initdb.d
// hook: that runs only against an empty PGDATA, and redir shares an instance
// that was initialised long ago by another service. Files placed there would
// be a silent no-op, so the server applies its own schema instead.
//
// Each file runs in one transaction together with the row that records it,
// so a failure leaves nothing half applied. A store missing its schema must
// not serve traffic, so the caller is expected to treat an error as fatal.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	ms, err := loadMigrations()
	if err != nil {
		return err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("cannot acquire connection for migration: %w", err)
	}
	defer conn.Release()

	// Hold the lock on one connection for the whole run, so a second
	// instance waits here rather than racing to apply the same file.
	if _, err := conn.Exec(ctx,
		`SELECT pg_advisory_lock($1)`, migrateLockKey); err != nil {
		return fmt.Errorf("cannot take migration lock: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(ctx,
			`SELECT pg_advisory_unlock($1)`, migrateLockKey); err != nil {
			log.Printf("cannot release migration lock: %v", err)
		}
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    BIGINT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("cannot create schema_migrations: %w", err)
	}

	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("cannot read schema_migrations: %w", err)
	}
	applied := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("cannot scan schema_migrations: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("cannot read schema_migrations: %w", err)
	}

	for _, m := range ms {
		if applied[m.version] {
			continue
		}
		if err := applyMigration(ctx, conn.Conn(), m); err != nil {
			return err
		}
		log.Printf("applied migration %v", m.name)
	}
	return nil
}

// applyMigration runs one file and records it in the same transaction.
func applyMigration(ctx context.Context, conn *pgx.Conn, m migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migration %v: cannot begin: %w", m.name, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, m.stmts); err != nil {
		return fmt.Errorf("migration %v: %w", m.name, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`,
		m.version); err != nil {
		return fmt.Errorf("migration %v: cannot record: %w", m.name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migration %v: cannot commit: %w", m.name, err)
	}
	return nil
}
