// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
)

// listingStore serves a fixed page of links and records what was asked
// for, so the handler can be tested without a database.
type listingStore struct {
	db.Store
	askedDaily []string
	series     map[string][]models.DayCount
}

func (s *listingStore) FetchAliasAll(
	_ context.Context, _ string, public bool, _, _ int64,
) ([]models.RedirIndex, int64, error) {
	r := models.RedirIndex{Alias: "blog", URL: "https://blog.example", PV: 9, UV: 4}
	if public {
		r.URL = ""
		r.PV, r.UV = 0, 0
	}
	return []models.RedirIndex{r}, 1, nil
}

func (s *listingStore) StatDaily(
	_ context.Context, _ string, aliases []string, _, _ time.Time,
) (map[string][]models.DayCount, error) {
	s.askedDaily = aliases
	return s.series, nil
}

// TestPublicIndexCarriesNoSeries checks the public listing is not charged
// for what it does not draw, and still discloses no target URL or counts.
func TestPublicIndexCarriesNoSeries(t *testing.T) {
	store := &listingStore{}
	s := &server{db: store}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/s/?mode=index&pn=1&ps=20", nil)
	if err := s.indexData(context.Background(), w, r, true); err != nil {
		t.Fatal(err)
	}

	var out indexOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Series != nil {
		t.Errorf("the public listing carried a series: %v", out.Series)
	}
	if store.askedDaily != nil {
		t.Errorf("the public listing queried daily counts for %v", store.askedDaily)
	}
	if out.Data[0].URL != "" {
		t.Errorf("the public listing disclosed a target URL: %q", out.Data[0].URL)
	}
}

// TestAdminIndexCarriesOneSeriesPerRow covers the sparkline on every row:
// the page's series arrive with the page, in one query, rather than one
// request per row.
func TestAdminIndexCarriesOneSeriesPerRow(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	store := &listingStore{series: map[string][]models.DayCount{
		"blog": {{Day: today.Format(time.DateOnly), PV: 7, UV: 3}},
	}}
	s := &server{db: store, latere: nil}

	saved := config.Conf
	t.Cleanup(func() { config.Conf = saved })
	config.Conf.Auth.Enable = config.None

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/s/?mode=index-pro&pn=1&ps=20", nil)
	if err := s.indexData(context.Background(), w, r, false); err != nil {
		t.Fatal(err)
	}

	var out indexOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(store.askedDaily) != 1 || store.askedDaily[0] != "blog" {
		t.Fatalf("asked for daily counts of %v, want the page's aliases",
			store.askedDaily)
	}

	row, ok := out.Series["blog"]
	if !ok {
		t.Fatal("no series for the row on the page")
	}
	// Every day in the window gets a value, so the chart draws a fixed
	// number of bars and a quiet day is a gap rather than a missing bar
	// that shifts everything after it.
	if len(row) != sparkDays {
		t.Fatalf("series has %d points, want %d", len(row), sparkDays)
	}
	var sum int64
	for _, v := range row {
		sum += v
	}
	if sum != 7 {
		t.Fatalf("series totals %d, want the 7 recorded", sum)
	}
}

// TestFillSeriesPadsQuietDays pins the padding on its own.
func TestFillSeriesPadsQuietDays(t *testing.T) {
	start, end := sparkRange()
	day := start.AddDate(0, 0, 2).Format(time.DateOnly)

	got := fillSeries(map[string][]models.DayCount{
		"a": {{Day: day, PV: 5}},
	}, start, end)

	row := got["a"]
	if len(row) != sparkDays {
		t.Fatalf("got %d points, want %d", len(row), sparkDays)
	}
	if row[2] != 5 {
		t.Errorf("the recorded day landed at index %d, not 2", indexOfNonZero(row))
	}
	for i, v := range row {
		if i != 2 && v != 0 {
			t.Errorf("index %d = %d, want a quiet day to read zero", i, v)
		}
	}
}

func indexOfNonZero(xs []int64) int {
	for i, v := range xs {
		if v != 0 {
			return i
		}
	}
	return -1
}
