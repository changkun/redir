// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db_test

import (
	"context"
	"testing"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"github.com/jackc/pgx/v5"
)

// seedForRederive records a mix of traffic and then puts the derived
// columns into the state an older classification rule would have left.
func seedForRederive(ctx context.Context, t *testing.T, s db.Store) *pgx.Conn {
	t.Helper()
	now := time.Now().UTC()

	for _, v := range []struct{ ua, ip, referer string }{
		{chromeUA, "1.1.1.1", "https://news.ycombinator.com/item?id=1"},
		{botUA, "9.9.9.1", ""},
		{firefoxUA, "1.1.1.2", ""},
	} {
		if _, err := s.RecordVisit(ctx, &models.Visit{
			Host: khost, Alias: kalias, IP: v.ip,
			UA: v.ua, Referer: v.referer, Time: now,
		}); err != nil {
			t.Fatal(err)
		}
	}

	conn, err := pgx.Connect(ctx, storeURI())
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	t.Cleanup(func() { conn.Close(ctx) })

	if _, err := conn.Exec(ctx, `
		UPDATE visits
		SET is_bot = false, browser = '', os = '', device = '',
		    referer_host = ''
		WHERE host = $1`, khost); err != nil {
		t.Fatal(err)
	}
	return conn
}

// TestRederiveUpdatesStaleRows covers the reason Rederive exists. The
// derived columns cache a pure function of ua and referer, so improving
// that function leaves stored rows classified by the old rule while new
// rows use the new one, and a single chart then mixes the two.
func TestRederiveUpdatesStaleRows(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		conn := seedForRederive(ctx, t, s)

		if err := db.Rederive(ctx, storeURI(), khost); err != nil {
			t.Fatalf("rederive failed: %v", err)
		}

		var bots int64
		if err := conn.QueryRow(ctx,
			`SELECT COUNT(*) FROM visits WHERE host = $1 AND is_bot`,
			khost).Scan(&bots); err != nil {
			t.Fatal(err)
		}
		if bots != 1 {
			t.Errorf("%d bot rows after re-deriving, want 1", bots)
		}

		var browser, refHost string
		if err := conn.QueryRow(ctx, `
			SELECT browser, referer_host FROM visits
			WHERE host = $1 AND ip = '1.1.1.1'`, khost,
		).Scan(&browser, &refHost); err != nil {
			t.Fatal(err)
		}
		if browser != "Chrome" {
			t.Errorf("browser = %q after re-deriving, want Chrome", browser)
		}
		if refHost != "news.ycombinator.com" {
			t.Errorf("referer_host = %q after re-deriving", refHost)
		}
	})
}

// TestRederiveIsIdempotent checks a second run writes nothing. It reads
// live data, so re-running after any rule change must be safe.
func TestRederiveIsIdempotent(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		conn := seedForRederive(ctx, t, s)

		if err := db.Rederive(ctx, storeURI(), khost); err != nil {
			t.Fatal(err)
		}
		before := rederiveSnapshot(ctx, t, conn)
		if err := db.Rederive(ctx, storeURI(), khost); err != nil {
			t.Fatal(err)
		}
		if after := rederiveSnapshot(ctx, t, conn); after != before {
			t.Fatalf("a second run changed the table:\n before %v\n after  %v",
				before, after)
		}
	})
}

// TestRederiveLeavesSourceColumnsAlone checks it rewrites only what it
// computes. ua and referer are the input; losing them would make the
// derivation unrepeatable.
func TestRederiveLeavesSourceColumnsAlone(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		conn := seedForRederive(ctx, t, s)

		var beforeUA, beforeRef string
		if err := conn.QueryRow(ctx, `
			SELECT ua, referer FROM visits WHERE host = $1 AND ip = '1.1.1.1'`,
			khost).Scan(&beforeUA, &beforeRef); err != nil {
			t.Fatal(err)
		}

		if err := db.Rederive(ctx, storeURI(), khost); err != nil {
			t.Fatal(err)
		}

		var afterUA, afterRef string
		if err := conn.QueryRow(ctx, `
			SELECT ua, referer FROM visits WHERE host = $1 AND ip = '1.1.1.1'`,
			khost).Scan(&afterUA, &afterRef); err != nil {
			t.Fatal(err)
		}
		if afterUA != beforeUA || afterRef != beforeRef {
			t.Fatal("re-deriving rewrote a source column")
		}
	})
}

// TestRederiveOtherHostUntouched checks it is scoped to one site, which
// matters now that the table holds more than one.
func TestRederiveOtherHostUntouched(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		conn := seedForRederive(ctx, t, s)

		if _, err := conn.Exec(ctx, `
			INSERT INTO visits (host, alias, ip, ua, time, is_bot)
			VALUES ('other.example', 'a', '8.8.8.8', $1, NOW(), false)`,
			botUA); err != nil {
			t.Fatal(err)
		}

		if err := db.Rederive(ctx, storeURI(), khost); err != nil {
			t.Fatal(err)
		}

		var isBot bool
		if err := conn.QueryRow(ctx,
			`SELECT is_bot FROM visits WHERE host = 'other.example' AND ip = '8.8.8.8'`,
		).Scan(&isBot); err != nil {
			t.Fatal(err)
		}
		if isBot {
			t.Fatal("re-deriving one host changed another host's rows")
		}
	})
}

// rederiveSnapshot returns a stable description of the derived columns.
func rederiveSnapshot(ctx context.Context, t *testing.T, conn *pgx.Conn) string {
	t.Helper()
	var s string
	if err := conn.QueryRow(ctx, `
		SELECT COALESCE(string_agg(
		         id || ':' || browser || ':' || os || ':' || device ||
		         ':' || is_bot || ':' || referer_host, '|' ORDER BY id), '')
		FROM visits WHERE host = $1`, khost).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}
