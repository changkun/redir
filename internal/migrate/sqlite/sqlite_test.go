// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/migrate"
	"changkun.de/x/redir/internal/migrate/sqlite"
	"github.com/jackc/pgx/v5"
	_ "modernc.org/sqlite"
)

// forkDB builds a database with the schema of the redir fork that served
// golang.design, holding the shapes its data actually has: a kind=1 row,
// null ip/ua/referer columns, and links without timestamps.
func forkDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "redir.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE ` + "`collink`" + ` (
		  ` + "`id`" + ` integer NOT NULL PRIMARY KEY AUTOINCREMENT,
		  ` + "`alias`" + ` varchar(50) NOT NULL DEFAULT '',
		  ` + "`kind`" + ` integer NOT NULL DEFAULT '0',
		  ` + "`url`" + ` varchar(1024) NOT NULL DEFAULT '',
		  ` + "`private`" + ` integer NOT NULL DEFAULT '0',
		  ` + "`created_at`" + ` datetime DEFAULT NULL,
		  ` + "`updated_at`" + ` datetime DEFAULT NULL,
		  UNIQUE (` + "`alias`" + `)
		);
		CREATE TABLE ` + "`visit`" + ` (
		  ` + "`id`" + ` integer NOT NULL PRIMARY KEY AUTOINCREMENT,
		  ` + "`alias`" + ` varchar(50) NOT NULL DEFAULT '',
		  ` + "`kind`" + ` integer NOT NULL DEFAULT '0',
		  ` + "`ip`" + ` varchar(50) DEFAULT NULL,
		  ` + "`ua`" + ` varchar(1000) DEFAULT NULL,
		  ` + "`referer`" + ` varchar(500) DEFAULT NULL,
		  ` + "`created_at`" + ` datetime NOT NULL
		);`); err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`
		INSERT INTO collink (alias, kind, url, private, created_at, updated_at)
		VALUES
		  ('lockfree', 0, 'https://github.com/golang-design/lockfree', 0, ?, ?),
		  ('secret',   0, 'https://example.com/secret',                1, ?, ?),
		  ('x-thing',  1, 'https://github.com/golang-design/x-thing',  0, NULL, NULL)`,
		base, base, base, base); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO visit (alias, kind, ip, ua, referer, created_at)
		VALUES
		  ('lockfree', 0, '1.1.1.1', ?, 'https://news.ycombinator.com/item?id=1', ?),
		  ('lockfree', 0, '1.1.1.1', ?, NULL, ?),
		  ('lockfree', 0, NULL, NULL, NULL, ?),
		  ('x-thing',  1, '2.2.2.2', ?, NULL, ?),
		  ('lockfree', 0, '3.3.3.3', ?, NULL, ?)`,
		chromeUA, base,
		chromeUA, base.Add(time.Hour),
		base.Add(2*time.Hour),
		chromeUA, base.Add(3*time.Hour),
		uptimeRobotUA, base.Add(10*time.Hour)); err != nil {
		t.Fatal(err)
	}
	return path
}

const (
	chromeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	uptimeRobotUA = "Mozilla/5.0+(compatible; UptimeRobot/2.0; http://www.uptimerobot.com/)"
	siteHost      = "golang.design.example"
)

// TestSQLiteSinceExcludesTheBoundary is a regression test for an
// incremental pass that duplicated a visit.
//
// created_at is TEXT holding a Go timestamp with nanoseconds. Filtering
// with `created_at > ?` compared the stored string against whatever
// layout the driver renders a time.Time in, so the row exactly at the
// watermark came back as later than itself. On a real file that meant the
// second pass re-imported the last row of the first.
//
// The rows here are inserted as raw text in the fork's own format rather
// than through the driver, because writing and reading with the same
// driver is what hid the fault.
// The end-to-end tests write into a real PostgreSQL, because the point of
// the source is what arrives on the other side.

func targetURI() string {
	if uri := os.Getenv("REDIR_TEST_POSTGRES"); uri != "" {
		return uri
	}
	return "postgres://redir:redir@127.0.0.1:5432/redir_test?sslmode=disable"
}

func connect(ctx context.Context, t *testing.T) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, targetURI())
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	t.Cleanup(func() { conn.Close(ctx) })
	return conn
}

// schema applies redir's migrations, since a migration target needs the
// tables to exist.
func schema(ctx context.Context, t *testing.T) {
	t.Helper()
	s, err := db.NewStore(ctx, targetURI())
	if err != nil {
		t.Skipf("cannot open store: %v", err)
	}
	s.Close()
}

func TestSQLiteSinceExcludesTheBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raw.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE collink (id integer PRIMARY KEY AUTOINCREMENT,
		  alias text, kind int, url text, private int,
		  created_at datetime, updated_at datetime);
		CREATE TABLE visit (id integer PRIMARY KEY AUTOINCREMENT,
		  alias text, kind int, ip text, ua text, referer text,
		  created_at datetime NOT NULL);
		INSERT INTO visit (alias, kind, ip, ua, referer, created_at) VALUES
		  ('a', 0, '1.1.1.1', '', '', '2026-08-31 19:15:35.022133382+00:00'),
		  ('a', 0, '2.2.2.2', '', '', '2026-08-31 19:46:03.294108302+00:00');`,
	); err != nil {
		t.Fatal(err)
	}

	watermark, err := time.Parse(time.RFC3339Nano, "2026-08-31T19:15:35.022133382Z")
	if err != nil {
		t.Fatal(err)
	}

	src, err := sqlite.Open(path, watermark)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	ctx := context.Background()
	_, n, err := src.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("Counts after the watermark = %d, want 1: the row at the "+
			"watermark is not after itself", n)
	}

	var seen []string
	if err := src.Visits(ctx, func(v migrate.Visit) error {
		seen = append(seen, v.IP)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != "2.2.2.2" {
		t.Fatalf("Visits after the watermark = %v, want only the later row", seen)
	}

	// And with no watermark both rows come back, so the filter is not
	// simply dropping everything.
	all, err := sqlite.Open(path, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	defer all.Close()
	if _, n, err := all.Counts(ctx); err != nil || n != 2 {
		t.Fatalf("unfiltered count = %d (err %v), want 2", n, err)
	}
}

func TestSQLiteCounts(t *testing.T) {
	src, err := sqlite.Open(forkDB(t), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	links, visits, err := src.Counts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// kind is dropped: the import path row is an ordinary link, since
	// that is what it always was.
	if links != 3 || visits != 5 {
		t.Fatalf("counts = %d links, %d visits, want 3 and 5", links, visits)
	}
}

// TestSQLiteMigration copies the fork's database and checks what arrived.
func TestSQLiteMigration(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)
	conn := connect(ctx, t)

	src, err := sqlite.Open(forkDB(t), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	if err := migrate.Run(ctx, src, targetURI(), migrate.Options{
		Host: siteHost, Only: "all", Truncate: true,
	}); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var links, visits int64
	if err := conn.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM links  WHERE host = $1),
		       (SELECT COUNT(*) FROM visits WHERE host = $1)`, siteHost,
	).Scan(&links, &visits); err != nil {
		t.Fatal(err)
	}
	if links != 3 || visits != 5 {
		t.Fatalf("migrated %d links and %d visits, want 3 and 5", links, visits)
	}

	// private survives, since the public index depends on it.
	var private bool
	if err := conn.QueryRow(ctx,
		`SELECT private FROM links WHERE host = $1 AND alias = 'secret'`,
		siteHost).Scan(&private); err != nil {
		t.Fatal(err)
	}
	if !private {
		t.Error("a private link came across as public")
	}

	// Null columns become empty strings, not fabricated values, and the
	// row still counts.
	var nullRow int64
	if err := conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM visits
		WHERE host = $1 AND ip = '' AND ua = '' AND referer = ''`,
		siteHost).Scan(&nullRow); err != nil {
		t.Fatal(err)
	}
	if nullRow != 1 {
		t.Errorf("%d rows from null columns, want 1", nullRow)
	}

	// The fork had no visitor cookie, so every row arrives without one
	// rather than with an invented identifier.
	var withVisitor int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1 AND visitor_id IS NOT NULL`,
		siteHost).Scan(&withVisitor); err != nil {
		t.Fatal(err)
	}
	if withVisitor != 0 {
		t.Errorf("%d migrated rows carry an invented visitor id", withVisitor)
	}

	// The derived columns are filled by the same code the server uses,
	// so the fork's history is classified like everything else.
	var bots int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1 AND is_bot`,
		siteHost).Scan(&bots); err != nil {
		t.Fatal(err)
	}
	if bots != 1 {
		t.Errorf("%d bot rows, want 1 (UptimeRobot)", bots)
	}
	var refHost string
	if err := conn.QueryRow(ctx, `
		SELECT referer_host FROM visits
		WHERE host = $1 AND referer <> '' LIMIT 1`, siteHost).Scan(&refHost); err != nil {
		t.Fatal(err)
	}
	if refHost != "news.ycombinator.com" {
		t.Errorf("referer_host = %q", refHost)
	}
}

// TestSQLiteSincePass covers the incremental pass that closes the window
// between an import and moving traffic to the new service.
func TestSQLiteSincePass(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)
	conn := connect(ctx, t)
	path := forkDB(t)

	// A first pass covering everything up to a cutoff.
	cutoff := time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC)
	first, err := sqlite.Open(path, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	// Simulate the first pass having stopped at the cutoff by importing
	// only what precedes it, then appending the rest.
	early, err := sqlite.Open(path, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	defer early.Close()
	_ = early

	late, err := sqlite.Open(path, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	defer late.Close()

	_, lateVisits, err := late.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if lateVisits != 1 {
		t.Fatalf("since filter selected %d visits, want the 1 after the cutoff",
			lateVisits)
	}

	// Import everything, then append the late rows: the appended ones
	// are added rather than replacing what is there.
	if err := migrate.Run(ctx, first, targetURI(), migrate.Options{
		Host: siteHost, Only: "all", Truncate: true,
	}); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1`, siteHost,
	).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if err := migrate.Run(ctx, late, targetURI(), migrate.Options{
		Host: siteHost, Only: "visits", Append: true,
	}); err != nil {
		t.Fatalf("append pass failed: %v", err)
	}

	var after int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1`, siteHost,
	).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before+lateVisits {
		t.Fatalf("visits went from %d to %d, want %d",
			before, after, before+lateVisits)
	}
}

// TestSQLiteIsOpenedReadOnly checks the source cannot be written. It is
// the fallback if the migration is wrong.
func TestSQLiteIsOpenedReadOnly(t *testing.T) {
	path := forkDB(t)
	src, err := sqlite.Open(path, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM visit`); err == nil {
		t.Fatal("the read-only connection accepted a delete")
	}
}
