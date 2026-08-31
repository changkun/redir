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
)

// seedMixed records a known mix of human and crawler traffic and returns the
// range covering it.
func seedMixed(ctx context.Context, t *testing.T, s db.Store) (time.Time, time.Time) {
	t.Helper()
	now := time.Now().UTC()

	visits := []struct {
		ua, ip, referer string
	}{
		{chromeUA, "1.1.1.1", "https://news.ycombinator.com/item?id=1"},
		{chromeUA, "1.1.1.2", "https://news.ycombinator.com/item?id=2"},
		{firefoxUA, "1.1.1.3", ""},
		{iphoneUA, "1.1.1.4", "https://x.com/someone/status/1"},
		// Three crawlers, one of which announces itself as a desktop
		// browser and would pass a substring test for "bot".
		{botUA, "9.9.9.1", ""},
		{amazonBotUA, "9.9.9.2", ""},
		{"curl/8.4.0", "9.9.9.3", ""},
	}
	for _, v := range visits {
		if _, err := s.RecordVisit(ctx, &models.Visit{
			Host: khost, Alias: kalias, IP: v.ip,
			UA: v.ua, Referer: v.referer, Time: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return now.Add(-time.Hour), now.Add(time.Hour)
}

func total(rs []models.NameCount) int64 {
	var n int64
	for _, r := range rs {
		n += r.Count
	}
	return n
}

func byName(rs []models.NameCount) map[string]int64 {
	m := map[string]int64{}
	for _, r := range rs {
		m[r.Name] = r.Count
	}
	return m
}

// TestStatGroupExcludesBots is the change this spec exists to make. Every
// grouped figure counts people only, so the charts on one page agree with
// each other. Previously bots were dropped from the browser and device
// charts and kept in the referrer and time series.
func TestStatGroupExcludesBots(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		from, to := seedMixed(ctx, t, s)

		for _, by := range []string{"referer", "browser", "os", "device"} {
			rs, err := s.StatGroup(ctx, khost, kalias, by, from, to)
			if err != nil {
				t.Fatalf("%v: %v", by, err)
			}
			if got := total(rs); got != 4 {
				t.Errorf("%v counts %d visits, want the 4 non-bot ones (%+v)",
					by, got, rs)
			}
		}
	})
}

// TestStatBotsReportsWhatIsExcluded checks the exclusion is visible. An
// exclusion nobody can see is indistinguishable from missing data.
func TestStatBotsReportsWhatIsExcluded(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		from, to := seedMixed(ctx, t, s)

		b, err := s.StatBots(ctx, khost, kalias, from, to)
		if err != nil {
			t.Fatal(err)
		}
		if b.PV != 3 {
			t.Errorf("bot PV = %d, want 3", b.PV)
		}
		if b.UV != 3 {
			t.Errorf("bot UV = %d, want 3", b.UV)
		}

		// The parts add up to the whole: what is shown plus what is
		// excluded is every visit recorded.
		shown, err := s.StatGroup(ctx, khost, kalias, "browser", from, to)
		if err != nil {
			t.Fatal(err)
		}
		if got := total(shown) + b.PV; got != 7 {
			t.Errorf("shown %d + excluded %d = %d, want 7 recorded visits",
				total(shown), b.PV, got)
		}
	})
}

// TestStatGroupBrowserAndOS checks the grouping the dashboard used to do
// in the browser now happens in the database.
func TestStatGroupBrowserAndOS(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		from, to := seedMixed(ctx, t, s)

		browsers, err := s.StatGroup(ctx, khost, kalias, "browser", from, to)
		if err != nil {
			t.Fatal(err)
		}
		b := byName(browsers)
		if b["Chrome"] != 2 {
			t.Errorf("Chrome = %d, want 2 (%+v)", b["Chrome"], browsers)
		}
		if b["Firefox"] != 1 {
			t.Errorf("Firefox = %d, want 1 (%+v)", b["Firefox"], browsers)
		}
		if b["Safari"] != 1 {
			t.Errorf("Safari = %d, want 1 (%+v)", b["Safari"], browsers)
		}

		oses, err := s.StatGroup(ctx, khost, kalias, "os", from, to)
		if err != nil {
			t.Fatal(err)
		}
		o := byName(oses)
		if o["macOS"] != 2 || o["Linux"] != 1 || o["iOS"] != 1 {
			t.Errorf("os counts = %+v", oses)
		}

		devices, err := s.StatGroup(ctx, khost, kalias, "device", from, to)
		if err != nil {
			t.Fatal(err)
		}
		d := byName(devices)
		if d["desktop"] != 3 || d["mobile"] != 1 {
			t.Errorf("device counts = %+v", devices)
		}
	})
}

// TestStatGroupOrdersByCount checks the charts get their rows already
// sorted, so the dashboard does not re-sort what the database can order.
func TestStatGroupOrdersByCount(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		from, to := seedMixed(ctx, t, s)

		rs, err := s.StatGroup(ctx, khost, kalias, "browser", from, to)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(rs); i++ {
			if rs[i-1].Count < rs[i].Count {
				t.Fatalf("rows are not ordered by count: %+v", rs)
			}
		}
	})
}

// TestStatGroupRejectsUnknownColumn checks the grouping column cannot come
// from the request. It is concatenated into the query, so an unknown name
// must be refused rather than passed through.
func TestStatGroupRejectsUnknownColumn(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		now := time.Now().UTC()

		for _, by := range []string{
			"ip", "ua", "visitor_id", "", "referer_host",
			"browser; DROP TABLE visits--",
			"browser'", "1=1",
		} {
			if _, err := s.StatGroup(ctx, khost, kalias, by,
				now.Add(-time.Hour), now.Add(time.Hour)); err == nil {
				t.Errorf("StatGroup accepted %q as a grouping column", by)
			}
		}

		// The visit table is still there.
		if _, err := s.StatGroup(ctx, khost, kalias, "browser",
			now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			t.Fatalf("a valid grouping stopped working: %v", err)
		}
	})
}

// TestStatVisitHistExcludesBots checks the time series counts the same
// population as the other charts. It is the figure that drops most: on
// production data roughly three visits in four are automated.
func TestStatVisitHistExcludesBots(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		from, to := seedMixed(ctx, t, s)

		hist, err := s.StatVisitHist(ctx, khost, kalias, from, to)
		if err != nil {
			t.Fatal(err)
		}
		var pv int
		for _, h := range hist {
			pv += h.PV
		}
		if pv != 4 {
			t.Fatalf("history PV = %d, want the 4 non-bot visits (%+v)", pv, hist)
		}
	})
}

// TestIndexKeepsAllTraffic pins the deliberate asymmetry. The stats page
// shows people; the index listing shows everything that arrived. Changing
// the listing too would alter numbers that v0.7.0 and v0.8.0 report
// identically, which would make a rollback look like data loss.
func TestIndexKeepsAllTraffic(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		seedMixed(ctx, t, s)

		rs, err := s.StatVisit(ctx, khost, []string{kalias})
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 1 || rs[0].PV != 7 {
			t.Fatalf("StatVisit = %+v, want all 7 visits", rs)
		}

		idx, _, err := s.FetchAliasAll(ctx, khost, false, 20, 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range idx {
			if r.Alias == kalias && r.PV != 7 {
				t.Fatalf("index PV = %d, want all 7 visits", r.PV)
			}
		}
	})
}
