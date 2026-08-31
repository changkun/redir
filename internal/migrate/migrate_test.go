// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package migrate_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/migrate"
	"github.com/jackc/pgx/v5"
)

const testHost = "migrate.example"

func targetURI() string {
	if uri := os.Getenv("REDIR_TEST_POSTGRES"); uri != "" {
		return uri
	}
	return "postgres://redir:redir@127.0.0.1:5432/redir_test?sslmode=disable"
}

// fakeSource stands in for a store being migrated away from.
//
// Its contents are the shapes redir's own data actually held, which is
// what the migration has to survive: an alias nobody visited, a visit
// whose alias is not a link, a visit for the index page, a document with
// no visitor id, one whose visitor id is a scanner probe, a string with a
// NUL byte, and a link with no timestamps at all.
type fakeSource struct {
	links    []migrate.Link
	visits   []migrate.Visit
	failOn   int // return an error after this many visits, 0 for never
	countErr error
}

func (f *fakeSource) Links(context.Context) ([]migrate.Link, error) {
	return f.links, nil
}

func (f *fakeSource) Visits(_ context.Context, fn func(migrate.Visit) error) error {
	for i, v := range f.visits {
		if f.failOn > 0 && i == f.failOn {
			return errors.New("source failed part way through")
		}
		if err := fn(v); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeSource) Counts(context.Context) (int64, int64, error) {
	if f.countErr != nil {
		return 0, 0, f.countErr
	}
	return int64(len(f.links)), int64(len(f.visits)), nil
}

func seed() *fakeSource {
	now := time.Now().UTC().Truncate(time.Second)
	return &fakeSource{
		links: []migrate.Link{
			{Alias: "visited", URL: "https://example.com/a",
				ValidFrom: now, CreatedAt: now, UpdatedAt: now},
			{Alias: "unvisited", URL: "https://example.com/b", Private: true,
				ValidFrom: now, CreatedAt: now, UpdatedAt: now},
			// No timestamps at all, as 131 of redir's links had.
			{Alias: "undated", URL: "https://example.com/undated"},
		},
		visits: []migrate.Visit{
			{VisitorID: "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
				Alias: "visited", IP: "1.2.3.4",
				UA:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
				Referer: "https://news.ycombinator.com/item?id=1", Time: now},
			// A bot, which the dashboard counts as a person today.
			{VisitorID: "3f2504e0-4f89-41d3-9a0c-0305e82c3302",
				Alias: "visited", IP: "1.2.3.5",
				UA:   "Mozilla/5.0+(compatible; UptimeRobot/2.0; http://www.uptimerobot.com/)",
				Time: now},
			// No visitor id.
			{Alias: "visited", IP: "1.2.3.6", Time: now},
			// A scanner probe recorded as a visitor id.
			{VisitorID: `-1%20OR%202%2B471-471-1=0%2B0%2B0%2B1%20--%20`,
				Alias: "visited", IP: "1.2.3.7", Time: now},
			// An alias that is not a link.
			{Alias: "never-existed", IP: "1.2.3.8", Time: now},
			// The index page.
			{Alias: "", IP: "1.2.3.9", Time: now},
			// A NUL byte, which a postgres text column rejects. The
			// agent is otherwise a real browser, so this row tests
			// the stripping and nothing else.
			{Alias: "visited", IP: "1.2.3.10",
				UA:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64)\x00 AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
				Time: now},
		},
	}
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

func opts(only string, truncate bool) migrate.Options {
	return migrate.Options{Host: testHost, Only: only, Truncate: truncate}
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)
	conn := connect(ctx, t)

	if err := migrate.Run(ctx, seed(), targetURI(), opts("all", true)); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// Every row is carried, orphans included. They are counted by no stat
	// query, but dropping them would lose history.
	var links, visits int64
	if err := conn.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM links  WHERE host = $1),
		       (SELECT COUNT(*) FROM visits WHERE host = $1)`, testHost,
	).Scan(&links, &visits); err != nil {
		t.Fatal(err)
	}
	if links != 3 || visits != 7 {
		t.Fatalf("migrated %d links and %d visits, want 3 and 7", links, visits)
	}

	// Unparseable visitor ids become NULL, and the valid ones survive.
	var nulls, kept int64
	if err := conn.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE visitor_id IS NULL),
		       COUNT(*) FILTER (WHERE visitor_id IN
		         ('3f2504e0-4f89-41d3-9a0c-0305e82c3301',
		          '3f2504e0-4f89-41d3-9a0c-0305e82c3302'))
		FROM visits WHERE host = $1`, testHost).Scan(&nulls, &kept); err != nil {
		t.Fatal(err)
	}
	if nulls != 5 {
		t.Errorf("%d null visitor ids, want 5", nulls)
	}
	if kept != 2 {
		t.Errorf("%d preserved visitor ids, want 2", kept)
	}

	// The NUL byte is gone and the rest of the string survived, so the
	// agent still parses as the browser it names.
	var ua, browser string
	if err := conn.QueryRow(ctx,
		`SELECT ua, browser FROM visits WHERE host = $1 AND ip = '1.2.3.10'`,
		testHost).Scan(&ua, &browser); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(ua, 0) {
		t.Errorf("ua = %q, still holds a NUL byte", ua)
	}
	if !strings.HasPrefix(ua, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit") {
		t.Errorf("ua = %q, the string did not survive stripping", ua)
	}
	if browser != "Chrome" {
		t.Errorf("browser = %q, want Chrome", browser)
	}

	// The derived columns are filled by the code the server uses.
	var bots int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1 AND is_bot`,
		testHost).Scan(&bots); err != nil {
		t.Fatal(err)
	}
	if bots != 1 {
		t.Errorf("%d bot rows, want 1 (UptimeRobot)", bots)
	}
	var refHost string
	if err := conn.QueryRow(ctx,
		`SELECT referer_host FROM visits WHERE host = $1 AND ip = '1.2.3.4'`,
		testHost).Scan(&refHost); err != nil {
		t.Fatal(err)
	}
	if refHost != "news.ycombinator.com" {
		t.Errorf("referer_host = %q", refHost)
	}
}

// TestRunKeepsZeroTimestamps is the regression test for a migration that
// substituted the current time for a missing one. valid_from gates the
// redirect and updated_at orders the admin index, so stamping every
// undated link with the moment of the migration changes both.
func TestRunKeepsZeroTimestamps(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)
	conn := connect(ctx, t)

	if err := migrate.Run(ctx, seed(), targetURI(), opts("all", true)); err != nil {
		t.Fatal(err)
	}

	var validFrom, createdAt, updatedAt time.Time
	if err := conn.QueryRow(ctx, `
		SELECT valid_from, created_at, updated_at
		FROM links WHERE host = $1 AND alias = 'undated'`, testHost,
	).Scan(&validFrom, &createdAt, &updatedAt); err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]time.Time{
		"valid_from": validFrom, "created_at": createdAt, "updated_at": updatedAt,
	} {
		if !got.IsZero() {
			t.Errorf("%v = %v, want the zero time: the source had none", name, got)
		}
	}
}

// TestRunRefusesNonEmptyTarget guards against running a migration twice.
// A doubled visit count looks plausible enough to go unnoticed.
func TestRunRefusesNonEmptyTarget(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)

	if err := migrate.Run(ctx, seed(), targetURI(), opts("all", true)); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Run(ctx, seed(), targetURI(), opts("all", false)); err == nil {
		t.Fatal("migrate ran against a non-empty target, want a refusal")
	}
}

// TestRunLinksOnly is the repair path. Visits carry no foreign key into
// links, so the link rows can be replaced without disturbing a visit
// history that has been growing since a cutover.
func TestRunLinksOnly(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)
	conn := connect(ctx, t)

	if err := migrate.Run(ctx, seed(), targetURI(), opts("all", true)); err != nil {
		t.Fatal(err)
	}

	// A visit recorded after the cutover, which the repair must keep.
	if _, err := conn.Exec(ctx, `
		INSERT INTO visits (host, alias, ip, time)
		VALUES ($1, 'visited', '9.9.9.9', NOW())`, testHost); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1`, testHost,
	).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if err := migrate.Run(ctx, seed(), targetURI(), opts("links", true)); err != nil {
		t.Fatalf("links-only repair failed: %v", err)
	}

	var after, links int64
	if err := conn.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM visits WHERE host = $1),
		       (SELECT COUNT(*) FROM links  WHERE host = $1)`, testHost,
	).Scan(&after, &links); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("visits went from %d to %d during a links-only run", before, after)
	}
	if links != 3 {
		t.Fatalf("%d links after the repair, want 3", links)
	}
}

// TestRunLinksTwiceIsIdempotent checks the per-host replacement: running
// the link copy again neither duplicates rows nor fails on the unique
// constraint.
func TestRunLinksTwiceIsIdempotent(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)
	conn := connect(ctx, t)

	for range 2 {
		if err := migrate.Run(ctx, seed(), targetURI(),
			opts("links", true)); err != nil {
			t.Fatal(err)
		}
	}
	var links int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM links WHERE host = $1`, testHost,
	).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 3 {
		t.Fatalf("%d links after two runs, want 3", links)
	}
}

// TestRunOtherHostUntouched checks that migrating one site does not
// disturb another's links, which is what 004 depends on.
func TestRunOtherHostUntouched(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)
	conn := connect(ctx, t)

	if err := migrate.Run(ctx, seed(), targetURI(), opts("all", true)); err != nil {
		t.Fatal(err)
	}
	other := migrate.Options{Host: "other.example", Only: "links"}
	if err := migrate.Run(ctx, seed(), targetURI(), other); err != nil {
		t.Fatal(err)
	}

	var first, second int64
	if err := conn.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM links WHERE host = $1),
		       (SELECT COUNT(*) FROM links WHERE host = $2)`,
		testHost, "other.example").Scan(&first, &second); err != nil {
		t.Fatal(err)
	}
	if first != 3 || second != 3 {
		t.Fatalf("links are %d and %d, want 3 each", first, second)
	}
}

// TestRunReportsSourceMismatch checks the count verification. A copy that
// silently moves fewer rows than the source holds is the failure this
// exists to catch.
func TestRunReportsSourceMismatch(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)

	src := seed()
	src.failOn = 3 // stop after three visits, but Counts still claims seven
	err := migrate.Run(ctx, src, targetURI(), opts("all", true))
	if err == nil {
		t.Fatal("a truncated copy reported success")
	}
}

func TestOptionsValid(t *testing.T) {
	if err := (migrate.Options{Host: "h", Only: "tables"}).Valid(); err == nil {
		t.Error("an unknown table set was accepted")
	}
	if err := (migrate.Options{Only: "all"}).Valid(); err == nil {
		t.Error("a missing host was accepted")
	}
	if err := (migrate.Options{Host: "h"}).Valid(); err != nil {
		t.Errorf("an empty Only was rejected: %v", err)
	}
}

func TestVisitorID(t *testing.T) {
	if id := migrate.VisitorID("3f2504e0-4f89-41d3-9a0c-0305e82c3301"); id == nil {
		t.Error("a valid uuid was discarded")
	}
	for _, s := range []string{"", "not-a-uuid", `-1%20OR%202%2B471=0`} {
		if id := migrate.VisitorID(s); id != nil {
			t.Errorf("VisitorID(%q) = %v, want nil", s, id)
		}
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	ctx := context.Background()
	schema(ctx, t)
	conn := connect(ctx, t)

	if _, err := conn.Exec(ctx,
		`DELETE FROM visits WHERE host = $1`, testHost); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx,
		`DELETE FROM links WHERE host = $1`, testHost); err != nil {
		t.Fatal(err)
	}

	o := opts("all", false)
	o.DryRun = true
	if err := migrate.Run(ctx, seed(), targetURI(), o); err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	var links, visits int64
	if err := conn.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM links  WHERE host = $1),
		       (SELECT COUNT(*) FROM visits WHERE host = $1)`, testHost,
	).Scan(&links, &visits); err != nil {
		t.Fatal(err)
	}
	if links != 0 || visits != 0 {
		t.Fatalf("a dry run wrote %d links and %d visits", links, visits)
	}
}
