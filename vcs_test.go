// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
)

// recordingStore captures the links checkvcs creates.
type recordingStore struct {
	db.Store
	mu    sync.Mutex
	saved []*models.Redir
}

func (s *recordingStore) StoreAlias(_ context.Context, r *models.Redir) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := *r
	s.saved = append(s.saved, &c)
	return nil
}

// TestCheckVCSProbesTheSiteOrganisation is the sharpest edge in serving
// two sites from one process.
//
// checkvcs runs when /s/<alias> finds no link. It requests
// repo_path + "/" + alias and, on 200, creates a link row. If the repo
// path came from the process rather than from the site the request
// arrived on, a miss on golang.design would probe changkun's organisation
// and could mint a link pointing at the wrong owner's repository.
func TestCheckVCSProbesTheSiteOrganisation(t *testing.T) {
	var (
		mu     sync.Mutex
		probed []string
	)
	vcs := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			probed = append(probed, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
	defer vcs.Close()

	saved := config.Conf
	t.Cleanup(func() { config.Conf = saved })
	config.Conf.Host = "https://changkun.de"
	config.Conf.X.RepoPath = vcs.URL + "/changkun"
	config.Conf.Hosts = map[string]config.SiteOverride{
		"golang.design": {RepoPath: vcs.URL + "/golang-design"},
	}

	store := &recordingStore{}
	s := &server{db: store}

	for _, tt := range []struct {
		requestHost string
		wantPath    string
		wantHost    string
	}{
		{"changkun.de", "/changkun/thing", "changkun.de"},
		{"golang.design", "/golang-design/thing", "golang.design"},
		// A Host header naming no configured site falls back to the
		// primary one rather than opening a namespace of its own.
		{"evil.example", "/changkun/thing", "changkun.de"},
	} {
		t.Run(tt.requestHost, func(t *testing.T) {
			mu.Lock()
			probed = nil
			mu.Unlock()
			store.saved = nil

			site := config.Conf.SiteFor(tt.requestHost)
			r, err := s.checkvcs(context.Background(), site, "thing")
			if err != nil {
				t.Fatal(err)
			}

			mu.Lock()
			got := append([]string(nil), probed...)
			mu.Unlock()
			if len(got) != 1 || got[0] != tt.wantPath {
				t.Fatalf("probed %v, want %v", got, tt.wantPath)
			}
			if r.URL != vcs.URL+tt.wantPath {
				t.Errorf("link URL = %q, want %q", r.URL, vcs.URL+tt.wantPath)
			}
			// The row must belong to the site that was asked for.
			if len(store.saved) != 1 || store.saved[0].Host != tt.wantHost {
				t.Fatalf("stored %+v, want a link on %v", store.saved, tt.wantHost)
			}
			if store.saved[0].Alias != "thing" {
				t.Errorf("stored alias = %q", store.saved[0].Alias)
			}
		})
	}
}

// TestXHandlerUsesTheSiteRepository checks the go-import meta tag names
// the repository of the site that was asked for. Getting this wrong sends
// `go get` to another organisation.
func TestXHandlerUsesTheSiteRepository(t *testing.T) {
	saved := config.Conf
	t.Cleanup(func() { config.Conf = saved })
	config.Conf.Host = "https://changkun.de"
	config.Conf.X.Enable = true
	config.Conf.X.Prefix = "/x/"
	config.Conf.X.VCS = "git"
	config.Conf.X.RepoPath = "https://github.com/changkun"
	config.Conf.X.GoDocHost = "https://pkg.go.dev/"
	config.Conf.Hosts = map[string]config.SiteOverride{
		"golang.design": {RepoPath: "https://github.com/golang-design"},
	}

	s := &server{}
	h := s.xHandler()

	for _, tt := range []struct{ host, want string }{
		{"changkun.de", "changkun.de/x/redir git https://github.com/changkun/redir"},
		{"golang.design", "golang.design/x/lockfree git https://github.com/golang-design/lockfree"},
	} {
		t.Run(tt.host, func(t *testing.T) {
			alias := "redir"
			if tt.host == "golang.design" {
				alias = "lockfree"
			}
			r := httptest.NewRequest(http.MethodGet, "/x/"+alias, nil)
			r.Host = tt.host
			w := httptest.NewRecorder()
			h(w, r)

			body := w.Body.String()
			if !contains(body, tt.want) {
				t.Fatalf("go-import does not name the site repository.\n want %v\n body %v",
					tt.want, body)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
