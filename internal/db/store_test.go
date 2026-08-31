// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"github.com/jackc/pgx/v5"
)

const (
	kalias = "alias"
	khost  = "test.example"
)

// storeURI names the database the store tests run against. They run
// against a real PostgreSQL rather than a mock: the queries are the thing
// under test, and a mock would only assert that they were called.
func storeURI() string {
	if uri := os.Getenv("REDIR_TEST_POSTGRES"); uri != "" {
		return uri
	}
	return "postgres://redir:redir@127.0.0.1:5432/redir_test?sslmode=disable"
}

// open connects, empties the store and seeds one public alias, so every
// test starts from a known state rather than from whatever the local
// database happens to hold.
func open(ctx context.Context, t testing.TB) db.Store {
	t.Helper()

	s, err := db.NewStore(ctx, storeURI())
	if err != nil {
		t.Skipf("cannot connect to store: %v", err)
	}
	reset(ctx, t)

	err = s.StoreAlias(ctx, &models.Redir{
		Host:    khost,
		Alias:   kalias,
		URL:     "link",
		Private: false,
		Trust:   false,
	})
	if err != nil {
		t.Fatalf("cannot store alias to data store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// reset empties the store. The tests count rows, so leftovers from an
// earlier run would make them pass or fail for the wrong reason.
func reset(ctx context.Context, t testing.TB) {
	t.Helper()

	conn, err := pgx.Connect(ctx, storeURI())
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx,
		`TRUNCATE links, visits RESTART IDENTITY`); err != nil {
		t.Fatalf("cannot reset postgres: %v", err)
	}
}

// run executes fn against a freshly seeded store.
func run(t *testing.T, fn func(t *testing.T, s db.Store)) {
	t.Helper()
	ctx := context.Background()
	fn(t, open(ctx, t))
}

func TestStoreAliasRejectsDuplicate(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		err := s.StoreAlias(ctx, &models.Redir{
			Host: khost, Alias: kalias, URL: "other",
		})
		if err == nil {
			t.Fatal("StoreAlias overwrote an existing alias, want an error")
		}
		// Both backends report the same sentinel, so a caller can tell
		// "taken" from "broken" without matching on message text.
		if err != db.ErrAliasExists {
			t.Fatalf("StoreAlias err = %v, want ErrAliasExists", err)
		}

		r, err := s.FetchAlias(ctx, khost, kalias)
		if err != nil {
			t.Fatal(err)
		}
		if r.URL != "link" {
			t.Fatalf("URL = %q, want the original link", r.URL)
		}
	})
}

func TestUpdateAlias(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		const want = "link2"

		r, err := s.FetchAlias(ctx, khost, kalias)
		if err != nil {
			t.Fatalf("FetchAlias failed with err: %v", err)
		}

		err = s.UpdateAlias(ctx, &models.Redir{
			ID: r.ID, Host: khost, Alias: kalias, URL: want,
		})
		if err != nil {
			t.Fatalf("UpdateAlias failed with err: %v", err)
		}

		r, err = s.FetchAlias(ctx, khost, kalias)
		if err != nil {
			t.Fatalf("FetchAlias failed with err: %v", err)
		}
		if r.URL != want {
			t.Fatalf("URL = %v, want %v", r.URL, want)
		}
	})
}

func TestDeleteAlias(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		if err := s.DeleteAlias(ctx, khost, kalias); err != nil {
			t.Fatal(err)
		}
		if _, err := s.FetchAlias(ctx, khost, kalias); err == nil {
			t.Fatal("FetchAlias found a deleted alias")
		}
	})
}

func TestFetchAliasAll(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		rs, total, err := s.FetchAliasAll(ctx, khost, true, 20, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) == 0 || total == 0 {
			t.Fatalf("fetch returned nothing: %v, %v", rs, total)
		}
		// The public listing must not disclose where a link points.
		for _, r := range rs {
			if r.URL != "" {
				t.Fatalf("public listing disclosed URL %q", r.URL)
			}
		}
	})
}

func TestRecordVisitAndCount(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		now := time.Now().UTC()

		// Two visits from one address and one from another: PV is 3 and
		// UV is 2, since UV counts addresses.
		for _, ip := range []string{"1.2.3.4", "1.2.3.4", "5.6.7.8"} {
			if _, err := s.RecordVisit(ctx, &models.Visit{
				Host: khost, Alias: kalias, IP: ip,
				UA:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
				Time: now,
			}); err != nil {
				t.Fatal(err)
			}
		}

		rs, err := s.StatVisit(ctx, khost, []string{kalias})
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 1 {
			t.Fatalf("StatVisit returned %d rows, want 1", len(rs))
		}
		if rs[0].PV != 3 || rs[0].UV != 2 {
			t.Fatalf("PV/UV = %d/%d, want 3/2", rs[0].PV, rs[0].UV)
		}
	})
}

// TestRecordVisitRejectsNonUUIDCookie is a regression test for the values
// found in production, where the visitor cookie was stored verbatim and
// echoed back. Values such as
// "-1%20OR%202%2B471-471-1=0%2B0%2B0%2B1%20--%20" were recorded as visitor
// identifiers.
func TestRecordVisitRejectsNonUUIDCookie(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()

		const probe = `-1%20OR%202%2B471-471-1=0%2B0%2B0%2B1%20--%20`
		vid, err := s.RecordVisit(ctx, &models.Visit{
			Host: khost, Alias: kalias, IP: "1.2.3.4", Time: time.Now().UTC(),
			VisitorID: probe,
		})
		if err != nil {
			t.Fatal(err)
		}
		if vid == probe {
			t.Fatal("RecordVisit returned the unvalidated cookie value")
		}
		if len(vid) != 36 {
			t.Fatalf("visitor id %q is not a UUID", vid)
		}

		// A valid one is kept, so a returning visitor stays the same.
		const good = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
		vid, err = s.RecordVisit(ctx, &models.Visit{
			Host: khost, Alias: kalias, IP: "1.2.3.4", Time: time.Now().UTC(),
			VisitorID: good,
		})
		if err != nil {
			t.Fatal(err)
		}
		if vid != good {
			t.Fatalf("visitor id = %q, want the cookie value %q", vid, good)
		}
	})
}

// TestOrphanVisitsAreNotCounted covers the reason the stat queries join
// links. 21% of production visit rows have an alias that is not a link:
// the index page records an empty alias, and a visit is recorded before
// the alias is resolved, so 404s are counted. Querying visits alone would
// report them.
func TestOrphanVisitsAreNotCounted(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		now := time.Now().UTC()

		for _, alias := range []string{"", "never-existed", kalias} {
			if _, err := s.RecordVisit(ctx, &models.Visit{
				Host: khost, Alias: alias, IP: "1.2.3.4", Time: now,
			}); err != nil {
				t.Fatal(err)
			}
		}

		rs, err := s.StatVisit(ctx, khost, []string{kalias})
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 1 || rs[0].PV != 1 {
			t.Fatalf("StatVisit = %+v, want one row with PV 1", rs)
		}

		hist, err := s.StatVisitHist(ctx, khost, kalias,
			now.Add(-time.Hour), now.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		var pv int
		for _, h := range hist {
			pv += h.PV
		}
		if pv != 1 {
			t.Fatalf("history PV = %d, want 1", pv)
		}
	})
}

// TestZeroVisitAliasCountsZero checks that an alias nobody has visited
// reports no traffic rather than one phantom visit. The counts come from
// a LEFT JOIN, so the link is listed with zeroes instead of being either
// dropped or credited with a row that does not exist.
func TestZeroVisitAliasCountsZero(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		rs, err := s.StatVisit(ctx, khost, []string{kalias})
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 1 {
			t.Fatalf("StatVisit returned %d rows, want 1", len(rs))
		}
		if rs[0].PV != 0 || rs[0].UV != 0 {
			t.Fatalf("PV/UV = %d/%d for an unvisited alias, want 0/0",
				rs[0].PV, rs[0].UV)
		}
	})
}

// TestHostsAreSeparate is the property 004 depends on: the same alias on
// two sites is two links with two histories.
func TestHostsAreSeparate(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		const other = "other.example"

		if err := s.StoreAlias(ctx, &models.Redir{
			Host: other, Alias: kalias, URL: "elsewhere",
		}); err != nil {
			t.Fatalf("the same alias on another host was rejected: %v", err)
		}

		r, err := s.FetchAlias(ctx, other, kalias)
		if err != nil {
			t.Fatal(err)
		}
		if r.URL != "elsewhere" {
			t.Fatalf("URL = %q, want the other host's link", r.URL)
		}

		if _, err := s.RecordVisit(ctx, &models.Visit{
			Host: other, Alias: kalias, IP: "1.2.3.4", Time: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
		rs, err := s.StatVisit(ctx, khost, []string{kalias})
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 1 || rs[0].PV != 0 {
			t.Fatalf("StatVisit on %v = %+v, want PV 0: the visit belongs to %v",
				khost, rs, other)
		}
	})
}

func TestStatRefererAndUA(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		now := time.Now().UTC()

		for _, ref := range []string{"https://news.ycombinator.com/x", "", ""} {
			if _, err := s.RecordVisit(ctx, &models.Visit{
				Host: khost, Alias: kalias, IP: "1.2.3.4",
				Referer: ref, UA: "", Time: now,
			}); err != nil {
				t.Fatal(err)
			}
		}

		refs, err := s.StatReferer(ctx, khost, kalias,
			now.Add(-time.Hour), now.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int64{}
		for _, r := range refs {
			got[r.Referer] = r.Count
		}
		// The empty referer is reported as "unknown": the dashboard
		// matches that exact string.
		if got["unknown"] != 2 {
			t.Fatalf("unknown referer count = %d, want 2 (%+v)", got["unknown"], refs)
		}
		if got["https://news.ycombinator.com/x"] != 1 {
			t.Fatalf("referer counts = %+v", refs)
		}

		uas, err := s.StatUA(ctx, khost, kalias,
			now.Add(-time.Hour), now.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if len(uas) != 1 || uas[0].UA != "unknown" || uas[0].Count != 3 {
			t.Fatalf("ua stats = %+v, want one unknown row with count 3", uas)
		}
	})
}

// TestStatRangeExcludesEnd checks the range is half-open: time >= start
// and time < end. Adjacent days must not both count a visit on their
// shared boundary.
func TestStatRangeExcludesEnd(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.Add(2 * time.Hour)

		for _, at := range []time.Time{
			start.Add(-time.Second), start, end.Add(-time.Second), end,
		} {
			if _, err := s.RecordVisit(ctx, &models.Visit{
				Host: khost, Alias: kalias, IP: "1.2.3.4", Time: at,
			}); err != nil {
				t.Fatal(err)
			}
		}

		hist, err := s.StatVisitHist(ctx, khost, kalias, start, end)
		if err != nil {
			t.Fatal(err)
		}
		var pv int
		for _, h := range hist {
			pv += h.PV
		}
		if pv != 2 {
			t.Fatalf("PV in range = %d, want 2: start is included, end is not", pv)
		}
	})
}

func BenchmarkFetchAliasAll(b *testing.B) {
	ctx := context.Background()
	s := open(ctx, b)

	b.ReportAllocs()
	for b.Loop() {
		rs, total, err := s.FetchAliasAll(ctx, khost, false, 100, 1)
		if err != nil || len(rs) == 0 || total == 0 {
			b.Fatalf("fetch failed: %v, %v, %v", err, rs, total)
		}
	}
}

// TestHistBucketsAreUTC pins the hour boundary.
//
// time is timestamptz and date_trunc truncates in the session time zone,
// so an instance configured for another zone would shift every bucket.
// The store sets the session zone rather than inheriting it, and this
// asks for another zone in the URI to prove the setting wins.
func TestHistBucketsAreUTC(t *testing.T) {
	ctx := context.Background()

	uri := storeURI()
	if strings.Contains(uri, "?") {
		uri += "&timezone=Asia/Kolkata"
	} else {
		uri += "?timezone=Asia/Kolkata"
	}
	s, err := db.NewStore(ctx, uri)
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	defer s.Close()
	reset(ctx, t)

	if err := s.StoreAlias(ctx, &models.Redir{
		Host: khost, Alias: kalias, URL: "link",
	}); err != nil {
		t.Fatal(err)
	}

	// 23:30 UTC is 05:00 the next day in Kolkata, which is already on an
	// hour boundary there: truncating in that zone returns the visit
	// time unchanged. The bucket must be the UTC hour instead.
	at := time.Date(2026, 1, 1, 23, 30, 0, 0, time.UTC)
	if _, err := s.RecordVisit(ctx, &models.Visit{
		Host: khost, Alias: kalias, IP: "1.2.3.4", Time: at,
	}); err != nil {
		t.Fatal(err)
	}

	hist, err := s.StatVisitHist(ctx, khost, kalias,
		at.Add(-time.Hour), at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 {
		t.Fatalf("got %d buckets, want 1", len(hist))
	}
	want := time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC)
	if !hist[0].Time.UTC().Equal(want) {
		t.Fatalf("bucket = %v, want %v", hist[0].Time.UTC(), want)
	}
}
