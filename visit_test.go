// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
)

// stubStore records what recognizeVisitor passes to the store. Only the
// visit path is exercised, so the rest of the interface is unimplemented.
type stubStore struct {
	db.Store
	got *models.Visit
	vid string
}

func (s *stubStore) RecordVisit(_ context.Context, v *models.Visit) (string, error) {
	s.got = v
	return s.vid, nil
}

// TestVisitorCookieScope is a regression test for a cookie written as a
// raw header with no attributes. Without an explicit Path it defaults to
// the path of the request, /s/<alias>, so a visitor who follows a second
// alias does not send it back and is issued a new identity. On production
// data 277,318 of 348,356 visits invented a visitor.
func TestVisitorCookieScope(t *testing.T) {
	const vid = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	s := &server{db: &stubStore{vid: vid}}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/s/some-alias", nil)
	s.recognizeVisitor(context.Background(), w, r, "some-alias")

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != redirVidCookie || c.Value != vid {
		t.Fatalf("cookie = %v=%v, want %v=%v",
			c.Name, c.Value, redirVidCookie, vid)
	}
	if c.Path != "/" {
		t.Fatalf("cookie Path = %q, want \"/\": a cookie scoped to the "+
			"alias path identifies a visit, not a visitor", c.Path)
	}
	if !c.HttpOnly {
		t.Error("cookie is readable from script, want HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want Lax", c.SameSite)
	}
}

// TestVisitorCookieRoundTrips checks the consequence: a visitor arriving
// at a second alias is recognised rather than counted as a new one.
func TestVisitorCookieRoundTrips(t *testing.T) {
	const vid = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	stub := &stubStore{vid: vid}
	s := &server{db: stub}

	w := httptest.NewRecorder()
	first := httptest.NewRequest(http.MethodGet, "/s/first", nil)
	s.recognizeVisitor(context.Background(), w, first, "first")

	second := httptest.NewRequest(http.MethodGet, "/s/second", nil)
	for _, c := range w.Result().Cookies() {
		// A browser sends a cookie back only if its path matches. Ask
		// the cookie itself rather than assuming.
		if c.Path != "/" {
			t.Fatalf("cookie would not be sent to /s/second, Path is %q", c.Path)
		}
		second.AddCookie(c)
	}
	s.recognizeVisitor(context.Background(), httptest.NewRecorder(), second, "second")

	if stub.got.VisitorID != vid {
		t.Fatalf("second visit carried visitor id %q, want %q",
			stub.got.VisitorID, vid)
	}
}

// TestVisitCarriesRequestFacts checks that the fields the stats are built
// from actually arrive at the store.
func TestVisitCarriesRequestFacts(t *testing.T) {
	stub := &stubStore{vid: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"}
	s := &server{db: stub}

	r := httptest.NewRequest(http.MethodGet, "/s/alias", nil)
	r.Header.Set("User-Agent", "curl/8.4.0")
	r.Header.Set("Referer", "https://news.ycombinator.com/item?id=1")
	before := time.Now().UTC().Add(-time.Second)

	s.recognizeVisitor(context.Background(), httptest.NewRecorder(), r, "alias")

	if stub.got == nil {
		t.Fatal("no visit was recorded")
	}
	if stub.got.Alias != "alias" {
		t.Errorf("Alias = %q, want alias", stub.got.Alias)
	}
	if stub.got.UA != "curl/8.4.0" {
		t.Errorf("UA = %q", stub.got.UA)
	}
	if stub.got.Referer != "https://news.ycombinator.com/item?id=1" {
		t.Errorf("Referer = %q", stub.got.Referer)
	}
	if stub.got.Host == "" {
		t.Error("Host is empty: the visit would not belong to a site")
	}
	if stub.got.Time.Before(before) {
		t.Errorf("Time = %v, want a recent timestamp", stub.got.Time)
	}
}
