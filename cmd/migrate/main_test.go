// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const testHost = "migrate.example"

func uris() (mongoURI, pgURI string) {
	mongoURI = os.Getenv("REDIR_TEST_MONGO")
	if mongoURI == "" {
		mongoURI = "mongodb://0.0.0.0:27018"
	}
	pgURI = os.Getenv("REDIR_TEST_POSTGRES")
	if pgURI == "" {
		pgURI = "postgres://redir:redir@127.0.0.1:5432/redir_test?sslmode=disable"
	}
	return
}

// seed fills MongoDB with the shapes the production data actually holds:
// an alias nobody visited, a visit whose alias is not a link, a visit for
// the index page, a document with no visitor_id, one whose visitor_id is a
// scanner probe, and a string carrying a NUL byte that a PostgreSQL text
// column will not accept.
func seed(ctx context.Context, t *testing.T, uri string) {
	t.Helper()

	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).
		SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		t.Skipf("cannot connect to mongo: %v", err)
	}
	t.Cleanup(func() { cli.Disconnect(ctx) })
	if err := cli.Ping(ctx, nil); err != nil {
		t.Skipf("cannot connect to mongo: %v", err)
	}
	if err := cli.Database("redir").Drop(ctx); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	links := []any{
		bson.M{"alias": "visited", "url": "https://example.com/a",
			"private": false, "trust": false, "valid_from": now,
			"created_at": now, "updated_at": now},
		bson.M{"alias": "unvisited", "url": "https://example.com/b",
			"private": true, "trust": false, "valid_from": now,
			"created_at": now, "updated_at": now},
	}
	if _, err := cli.Database("redir").Collection("links").
		InsertMany(ctx, links); err != nil {
		t.Fatal(err)
	}

	visits := []any{
		// A normal visit.
		bson.M{"visitor_id": "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
			"alias": "visited", "ip": "1.2.3.4",
			"ua":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"referer": "https://news.ycombinator.com/item?id=1", "time": now},
		// A bot, which the dashboard counts as a person today.
		bson.M{"visitor_id": "3f2504e0-4f89-41d3-9a0c-0305e82c3302",
			"alias": "visited", "ip": "1.2.3.5",
			"ua":      "Mozilla/5.0+(compatible; UptimeRobot/2.0; http://www.uptimerobot.com/)",
			"referer": "", "time": now},
		// No visitor_id at all: 10,708 production rows.
		bson.M{"alias": "visited", "ip": "1.2.3.6", "ua": "", "referer": "", "time": now},
		// A scanner probe stored as a visitor id: 505 production rows.
		bson.M{"visitor_id": `-1%20OR%202%2B471-471-1=0%2B0%2B0%2B1%20--%20`,
			"alias": "visited", "ip": "1.2.3.7", "ua": "", "referer": "", "time": now},
		// An alias that is not a link: 3,155 production rows.
		bson.M{"alias": "never-existed", "ip": "1.2.3.8", "ua": "", "referer": "", "time": now},
		// The index page: 68,963 production rows.
		bson.M{"alias": "", "ip": "1.2.3.9", "ua": "", "referer": "", "time": now},
		// A NUL byte, which PostgreSQL text rejects outright.
		bson.M{"alias": "visited", "ip": "1.2.3.10",
			"ua": "bad\x00ua", "referer": "", "time": now},
	}
	if _, err := cli.Database("redir").Collection("visit").
		InsertMany(ctx, visits); err != nil {
		t.Fatal(err)
	}
}

func TestMigrate(t *testing.T) {
	ctx := context.Background()
	mongoURI, pgURI := uris()
	seed(ctx, t, mongoURI)

	conn, err := pgx.Connect(ctx, pgURI)
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	defer conn.Close(ctx)

	if err := run(ctx, mongoURI, pgURI, testHost, true, false, "all"); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// Every row is carried, orphans included. They are not counted by any
	// stat query, but dropping them would lose history.
	var links, visits int64
	if err := conn.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM links  WHERE host = $1),
		       (SELECT COUNT(*) FROM visits WHERE host = $1)`, testHost,
	).Scan(&links, &visits); err != nil {
		t.Fatal(err)
	}
	if links != 2 || visits != 7 {
		t.Fatalf("migrated %d links and %d visits, want 2 and 7", links, visits)
	}

	// Rows without a parseable visitor id become NULL rather than
	// invented UUIDs. Five of the seven seeded visits are in that state:
	// four carry no visitor_id and one carries a scanner probe.
	var nulls int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1 AND visitor_id IS NULL`,
		testHost).Scan(&nulls); err != nil {
		t.Fatal(err)
	}
	if nulls != 5 {
		t.Fatalf("%d null visitor ids, want 5", nulls)
	}

	// The probe is not stored anywhere: it is discarded, not kept as text.
	var probes int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1 AND ip = '1.2.3.7'
		   AND visitor_id IS NULL`, testHost).Scan(&probes); err != nil {
		t.Fatal(err)
	}
	if probes != 1 {
		t.Fatal("the scanner probe was not discarded")
	}

	// The two valid identifiers are preserved exactly.
	var kept int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits
		  WHERE host = $1 AND visitor_id IN
		    ('3f2504e0-4f89-41d3-9a0c-0305e82c3301',
		     '3f2504e0-4f89-41d3-9a0c-0305e82c3302')`,
		testHost).Scan(&kept); err != nil {
		t.Fatal(err)
	}
	if kept != 2 {
		t.Fatalf("%d preserved visitor ids, want 2", kept)
	}

	// The NUL byte is gone and the rest of the string survived.
	var ua string
	if err := conn.QueryRow(ctx,
		`SELECT ua FROM visits WHERE host = $1 AND ip = '1.2.3.10'`,
		testHost).Scan(&ua); err != nil {
		t.Fatal(err)
	}
	if ua != "badua" {
		t.Fatalf("ua = %q, want badua", ua)
	}

	// The derived columns are filled by the same code the server uses.
	var bots int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits WHERE host = $1 AND is_bot`,
		testHost).Scan(&bots); err != nil {
		t.Fatal(err)
	}
	if bots != 1 {
		t.Fatalf("%d bot rows, want 1 (UptimeRobot)", bots)
	}
	var refHost string
	if err := conn.QueryRow(ctx,
		`SELECT referer_host FROM visits WHERE host = $1 AND ip = '1.2.3.4'`,
		testHost).Scan(&refHost); err != nil {
		t.Fatal(err)
	}
	if refHost != "news.ycombinator.com" {
		t.Fatalf("referer_host = %q", refHost)
	}
}

// TestMigrateRefusesNonEmptyTarget is the guard against running the copy
// twice: a doubled visit count looks plausible enough to go unnoticed.
func TestMigrateRefusesNonEmptyTarget(t *testing.T) {
	ctx := context.Background()
	mongoURI, pgURI := uris()
	seed(ctx, t, mongoURI)

	if err := run(ctx, mongoURI, pgURI, testHost, true, false, "all"); err != nil {
		t.Fatalf("first migrate failed: %v", err)
	}
	err := run(ctx, mongoURI, pgURI, testHost, false, false, "all")
	if err == nil {
		t.Fatal("migrate ran against a non-empty target, want a refusal")
	}
}

// TestMigrateParity is the check the cutover depends on: both stores must
// answer the same question with the same number.
func TestMigrateParity(t *testing.T) {
	ctx := context.Background()
	mongoURI, pgURI := uris()
	seed(ctx, t, mongoURI)

	if err := run(ctx, mongoURI, pgURI, testHost, true, false, "all"); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	src, err := db.NewStore(ctx, mongoURI)
	if err != nil {
		t.Skipf("cannot open mongo store: %v", err)
	}
	defer src.Close()
	dst, err := db.NewStore(ctx, pgURI)
	if err != nil {
		t.Skipf("cannot open postgres store: %v", err)
	}
	defer dst.Close()

	aliases := []string{"visited"}
	want, err := src.StatVisit(ctx, testHost, aliases)
	if err != nil {
		t.Fatal(err)
	}
	got, err := dst.StatVisit(ctx, testHost, aliases)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Fatalf("mongo returned %d rows, postgres %d", len(want), len(got))
	}
	for i := range want {
		if want[i].Alias != got[i].Alias ||
			want[i].PV != got[i].PV || want[i].UV != got[i].UV {
			t.Fatalf("alias %v: mongo pv/uv %d/%d, postgres %d/%d",
				want[i].Alias, want[i].PV, want[i].UV, got[i].PV, got[i].UV)
		}
	}
	t.Logf("parity: %+v", got)

	// The referer and user agent breakdowns must agree too.
	now := time.Now().UTC()
	from, to := now.Add(-24*time.Hour), now.Add(24*time.Hour)
	wantRef, err := src.StatReferer(ctx, testHost, "visited", from, to)
	if err != nil {
		t.Fatal(err)
	}
	gotRef, err := dst.StatReferer(ctx, testHost, "visited", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if !sameRefs(wantRef, gotRef) {
		t.Fatalf("referer stats differ:\n mongo %+v\n pg    %+v", wantRef, gotRef)
	}
}

func sameRefs(a, b []models.RefStat) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int64{}
	for _, r := range a {
		counts[r.Referer] += r.Count
	}
	for _, r := range b {
		counts[r.Referer] -= r.Count
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

// TestMigrateZeroTimestampsPreserved checks that a link with no
// timestamps keeps them. valid_from gates the redirect and updated_at
// orders the admin index, so stamping every undated link with the moment
// of the migration changes both.
func TestMigrateZeroTimestampsPreserved(t *testing.T) {
	ctx := context.Background()
	mongoURI, pgURI := uris()
	seed(ctx, t, mongoURI)

	// A link as 124 production rows look: no valid_from, no created_at,
	// no updated_at.
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI).
		SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		t.Skipf("cannot connect to mongo: %v", err)
	}
	defer cli.Disconnect(ctx)
	if _, err := cli.Database("redir").Collection("links").InsertOne(ctx,
		bson.M{"alias": "undated", "url": "https://example.com/undated",
			"private": false}); err != nil {
		t.Fatal(err)
	}

	if err := run(ctx, mongoURI, pgURI, testHost, true, false, "all"); err != nil {
		t.Fatal(err)
	}

	conn, err := pgx.Connect(ctx, pgURI)
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	defer conn.Close(ctx)

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
			t.Errorf("%v = %v, want the zero time: the source had none",
				name, got)
		}
	}
}

// TestMigrateLinksOnly checks the repair path. Visits carry no foreign
// key into links, so the link rows can be replaced without touching the
// visit history, which is what makes a fix possible after a cutover.
func TestMigrateLinksOnly(t *testing.T) {
	ctx := context.Background()
	mongoURI, pgURI := uris()
	seed(ctx, t, mongoURI)

	if err := run(ctx, mongoURI, pgURI, testHost, true, false, "all"); err != nil {
		t.Fatal(err)
	}

	conn, err := pgx.Connect(ctx, pgURI)
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	defer conn.Close(ctx)

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

	if err := run(ctx, mongoURI, pgURI, testHost, true, false, "links"); err != nil {
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
		t.Fatalf("visits went from %d to %d during a links-only run",
			before, after)
	}
	if links != 2 {
		t.Fatalf("%d links after the repair, want 2", links)
	}
}
