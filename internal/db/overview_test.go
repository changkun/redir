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

// seedDays records visits across three days: two people and one crawler
// on the first, one person on the third, nothing on the second.
func seedDays(ctx context.Context, t *testing.T, s db.Store) (time.Time, time.Time) {
	t.Helper()
	day1 := time.Now().UTC().Truncate(24 * time.Hour).Add(-72 * time.Hour)

	for _, v := range []struct {
		at time.Time
		ua string
		ip string
	}{
		{day1.Add(time.Hour), chromeUA, "1.1.1.1"},
		{day1.Add(2 * time.Hour), chromeUA, "1.1.1.1"},
		{day1.Add(3 * time.Hour), firefoxUA, "2.2.2.2"},
		{day1.Add(4 * time.Hour), botUA, "9.9.9.9"},
		{day1.Add(48*time.Hour + time.Hour), chromeUA, "3.3.3.3"},
	} {
		if _, err := s.RecordVisit(ctx, &models.Visit{
			Host: khost, Alias: kalias, IP: v.ip, UA: v.ua, Time: v.at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return day1, day1.Add(96 * time.Hour)
}

// TestStatOverviewTotals covers the strip at the top of the console. Its
// figures have to reconcile with the store, and split traffic the way the
// charts below it do, or the page disagrees with itself.
func TestStatOverviewTotals(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		start, end := seedDays(ctx, t, s)

		o, err := s.StatOverview(ctx, khost, start, end)
		if err != nil {
			t.Fatal(err)
		}
		if o.Links != 1 {
			t.Errorf("Links = %d, want 1", o.Links)
		}
		// Visits counts everything, so it reconciles with the table.
		if o.Visits != 5 {
			t.Errorf("Visits = %d, want 5", o.Visits)
		}
		// People and bots split it the way the statistics page does.
		if o.People != 4 || o.Bots != 1 {
			t.Errorf("People/Bots = %d/%d, want 4/1", o.People, o.Bots)
		}
		if o.People+o.Bots != o.Visits {
			t.Errorf("the parts do not add up to the whole: %d + %d != %d",
				o.People, o.Bots, o.Visits)
		}
	})
}

// TestStatOverviewSeries checks the daily shape: bots are out, and a day
// with no traffic is absent rather than reported as zero, since only the
// caller knows how wide its chart is.
func TestStatOverviewSeries(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		start, end := seedDays(ctx, t, s)

		o, err := s.StatOverview(ctx, khost, start, end)
		if err != nil {
			t.Fatal(err)
		}
		if len(o.Series) != 2 {
			t.Fatalf("series has %d days, want 2 (%+v)", len(o.Series), o.Series)
		}
		// Three people on the first day from two addresses, and the
		// crawler is not among them.
		if o.Series[0].PV != 3 || o.Series[0].UV != 2 {
			t.Errorf("day one PV/UV = %d/%d, want 3/2",
				o.Series[0].PV, o.Series[0].UV)
		}
		if o.Series[1].PV != 1 {
			t.Errorf("day three PV = %d, want 1", o.Series[1].PV)
		}
		if o.Series[0].Day >= o.Series[1].Day {
			t.Error("the series is not in date order")
		}
	})
}

// TestStatDailyIsOneQueryForManyAliases covers the sparkline on every
// row. Asking per row would make a page of twenty links twenty round
// trips for a handful of numbers each.
func TestStatDailyIsOneQueryForManyAliases(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		start, end := seedDays(ctx, t, s)

		// A second link with traffic of its own, and a third with none.
		for _, a := range []string{"second", "silent"} {
			if err := s.StoreAlias(ctx, &models.Redir{
				Host: khost, Alias: a, URL: "https://example.com/" + a,
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.RecordVisit(ctx, &models.Visit{
			Host: khost, Alias: "second", IP: "4.4.4.4",
			UA: chromeUA, Time: start.Add(time.Hour),
		}); err != nil {
			t.Fatal(err)
		}

		got, err := s.StatDaily(ctx, khost,
			[]string{kalias, "second", "silent"}, start, end)
		if err != nil {
			t.Fatal(err)
		}

		if len(got[kalias]) != 2 {
			t.Errorf("%v has %d days, want 2", kalias, len(got[kalias]))
		}
		if len(got["second"]) != 1 {
			t.Errorf("second has %d days, want 1", len(got["second"]))
		}
		// A link nobody visited is absent rather than an empty entry, so
		// the caller draws a flat line from nothing.
		if _, ok := got["silent"]; ok {
			t.Error("an unvisited alias produced a series")
		}
	})
}

// TestStatDailyExcludesBots keeps the sparkline counting the same
// population as everything else on the page.
func TestStatDailyExcludesBots(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		start, end := seedDays(ctx, t, s)

		got, err := s.StatDaily(ctx, khost, []string{kalias}, start, end)
		if err != nil {
			t.Fatal(err)
		}
		var pv int64
		for _, d := range got[kalias] {
			pv += d.PV
		}
		if pv != 4 {
			t.Fatalf("series totals %d, want the 4 non-bot visits", pv)
		}
	})
}

// TestStatDailyEmptyInput checks the no-aliases case, which is what an
// empty page of results asks for.
func TestStatDailyEmptyInput(t *testing.T) {
	run(t, func(t *testing.T, s db.Store) {
		ctx := context.Background()
		got, err := s.StatDaily(ctx, khost, nil,
			time.Now().Add(-time.Hour), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("got %d series for no aliases", len(got))
		}
	})
}
