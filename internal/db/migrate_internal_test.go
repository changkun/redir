// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPostgresURI() string {
	if uri := os.Getenv("REDIR_TEST_POSTGRES"); uri != "" {
		return uri
	}
	return "postgres://redir:redir@127.0.0.1:5432/redir_test?sslmode=disable"
}

// TestLoadMigrations checks the files are found and ordered. A migration
// applied out of order would build the schema wrongly, and one skipped
// because its name does not parse would leave the schema incomplete
// without saying so.
func TestLoadMigrations(t *testing.T) {
	ms, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) == 0 {
		t.Fatal("no migrations were embedded")
	}
	for i, m := range ms {
		if m.stmts == "" {
			t.Errorf("migration %v is empty", m.name)
		}
		if i > 0 && ms[i-1].version >= m.version {
			t.Errorf("migration %v is not after %v", m.name, ms[i-1].name)
		}
	}
	if ms[0].version != 1 {
		t.Errorf("first migration is version %d, want 1", ms[0].version)
	}
}

// TestMigrateFromEmpty runs the whole schema against a database with
// nothing in it, which is the case the deploy will actually hit: redir's
// database on the shared instance is new.
func TestMigrateFromEmpty(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testPostgresURI())
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}

	// Start from nothing, including the bookkeeping table.
	if _, err := pool.Exec(ctx,
		`DROP TABLE IF EXISTS visits, links, schema_migrations`); err != nil {
		t.Fatal(err)
	}

	if err := migrate(ctx, pool); err != nil {
		t.Fatalf("migrate from empty failed: %v", err)
	}

	ms, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var applied int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != len(ms) {
		t.Fatalf("%d migrations recorded, want %d", applied, len(ms))
	}

	// The tables the store needs are there, and the visit table keeps
	// the columns the stats depend on.
	for _, col := range []string{
		"host", "alias", "visitor_id", "ip", "ua", "referer",
		"referer_host", "browser", "os", "device", "is_bot", "time",
	} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'visits' AND column_name = $1)`,
			col).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Errorf("visits is missing column %v", col)
		}
	}

	// visitor_id must be nullable: 11,213 production rows have no
	// parseable UUID and are migrated as NULL rather than invented.
	var nullable string
	if err := pool.QueryRow(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_name = 'visits' AND column_name = 'visitor_id'`,
	).Scan(&nullable); err != nil {
		t.Fatal(err)
	}
	if nullable != "YES" {
		t.Error("visits.visitor_id is NOT NULL, but historical rows have no UUID")
	}

	// ip must be text: with gdpr.hide_ip the address is a SHA-1 digest,
	// which inet would reject.
	var ipType string
	if err := pool.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'visits' AND column_name = 'ip'`,
	).Scan(&ipType); err != nil {
		t.Fatal(err)
	}
	if ipType != "text" {
		t.Errorf("visits.ip is %v, want text: hide_ip stores a digest", ipType)
	}

	// Running again applies nothing.
	if err := migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
	var again int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM schema_migrations`).Scan(&again); err != nil {
		t.Fatal(err)
	}
	if again != applied {
		t.Fatalf("second run recorded %d migrations, want %d", again, applied)
	}
}

// TestApplyMigrationRollsBack checks that a failing file leaves nothing
// behind, neither its tables nor its version row. A half-applied schema
// is worse than none, because the next start would skip it.
func TestApplyMigrationRollsBack(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testPostgresURI())
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	if err := migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DROP TABLE IF EXISTS half_applied`) //nolint:errcheck
	})

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	// The first statement succeeds and the second does not.
	bad := migration{
		version: 999999,
		name:    "999999_bad.sql",
		stmts: `CREATE TABLE half_applied (id INT);
		        SELECT * FROM a_table_that_does_not_exist;`,
	}
	if err := applyMigration(ctx, conn.Conn(), bad); err == nil {
		t.Fatal("a failing migration reported success")
	}

	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.tables
		               WHERE table_name = 'half_applied')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("the failing migration left its table behind")
	}

	var recorded bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`,
		bad.version).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded {
		t.Error("the failing migration was recorded as applied")
	}
}

// TestLoadMigrationsRejectsBadNames covers the naming rule. A file whose
// version does not parse would otherwise be skipped in silence, leaving
// the schema incomplete with nothing to say so.
func TestLoadMigrationsRejectsBadNames(t *testing.T) {
	for _, tt := range []struct{ name, file string }{
		{"no separator", "initial.sql"},
		{"unparseable version", "abc_initial.sql"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, ok := parseMigrationName(tt.file)
			if ok {
				t.Fatalf("parseMigrationName(%q) accepted a bad name", tt.file)
			}
		})
	}
	if v, _, ok := parseMigrationName("001_initial.sql"); !ok || v != 1 {
		t.Fatalf("parseMigrationName rejected a good name: %v %v", v, ok)
	}
}

// TestMigrateFailsWhenLockedOut checks that a failure to reach the
// database is returned, not logged and stepped over: a store without its
// schema must not serve traffic.
func TestMigrateFailsOnClosedPool(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testPostgresURI())
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("cannot connect to postgres: %v", err)
	}
	pool.Close()

	if err := migrate(ctx, pool); err == nil {
		t.Fatal("migrate on a closed pool reported success")
	}
}
