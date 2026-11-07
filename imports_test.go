// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"changkun.de/x/redir/internal/config"
)

// withSites configures changkun.de and golang.design as deployed, poly.red
// as a site that is one module path, and mod.example as a site with an
// import path but neither short links nor /x/.
func withSites(t *testing.T) {
	t.Helper()
	saved := config.Conf
	t.Cleanup(func() { config.Conf = saved })
	off := false
	config.Conf.Host = "https://changkun.de"
	config.Conf.S.Prefix = "/s/"
	config.Conf.X.Enable = true
	config.Conf.X.Prefix = "/x/"
	config.Conf.X.VCS = "git"
	config.Conf.X.RepoPath = "https://github.com/changkun"
	config.Conf.X.GoDocHost = "https://pkg.go.dev/"
	config.Conf.Hosts = map[string]config.SiteOverride{
		"golang.design": {RepoPath: "https://github.com/golang-design"},
		"poly.red": {Links: &off, Repos: &off, Imports: map[string]config.Import{
			"poly.red": {
				Repo: "https://github.com/changkun/polyred",
				Web:  "https://github.com/changkun/polyred",
			},
		}},
		"mod.example": {Links: &off, Repos: &off, Imports: map[string]config.Import{
			"mod.example/m": {Repo: "https://git.example/m"},
		}},
	}
}

func serve(h http.Handler, host, target string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r.Host = host
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// TestImportsAnswerForTheirSite checks poly.red answers as the static
// server it replaces did: the module's go-import for `go get` at any path
// below it, and the repository for a browser.
func TestImportsAnswerForTheirSite(t *testing.T) {
	withSites(t)
	h := (&server{}).handler()

	const tag = `<meta name="go-import" content="poly.red git https://github.com/changkun/polyred">`
	for _, target := range []string{
		"/?go-get=1",
		"/geometry/primitive?go-get=1",
		// Below poly.red even where another site has short links.
		"/s/foo?go-get=1",
		"/x/foo?go-get=1",
	} {
		w := serve(h, "poly.red", target)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), tag) {
			t.Errorf("poly.red%s: got %d %q, want the module's go-import", target, w.Code, w.Body.String())
		}
	}

	for host, want := range map[string]string{
		"poly.red/geometry": "https://github.com/changkun/polyred",
		// Without a page of its own, a browser gets the documentation.
		"mod.example/m/pkg": "https://pkg.go.dev/mod.example/m/pkg",
	} {
		h0, path, _ := strings.Cut(host, "/")
		w := serve(h, h0, "/"+path)
		if w.Code != http.StatusFound || w.Header().Get("Location") != want {
			t.Errorf("a browser at %s: got %d to %q, want %q", host, w.Code, w.Header().Get("Location"), want)
		}
	}

	w := serve(h, "poly.red", "/info/refs?service=git-upload-pack")
	if want := "https://github.com/changkun/polyred/info/refs?service=git-upload-pack"; w.Code != http.StatusFound || w.Header().Get("Location") != want {
		t.Errorf("git clone https://poly.red: got %d to %q, want %q", w.Code, w.Header().Get("Location"), want)
	}
}

// TestSitesOfferOnlyTheirServices checks that a site without short links
// never reaches the shortener. If it did, a miss on mod.example/s/thing
// would open a namespace of aliases for it, and checkvcs would probe an
// organisation and store a link there.
func TestSitesOfferOnlyTheirServices(t *testing.T) {
	withSites(t)
	// A store any call to which panics, except StoreAlias, which records.
	store := &recordingStore{}
	h := (&server{db: store}).handler()

	for _, target := range []string{"/s/thing", "/s/", "/s", "/x/thing", "/elsewhere"} {
		if w := serve(h, "mod.example", target); w.Code != http.StatusNotFound {
			t.Errorf("mod.example%s: got %d, want %d", target, w.Code, http.StatusNotFound)
		}
	}
	if len(store.saved) != 0 {
		t.Errorf("a site without short links stored %v", store.saved)
	}

	// The sites that offer /x/ still do, each from its organisation.
	for host, want := range map[string]string{
		"changkun.de":   `content="changkun.de/x/redir git https://github.com/changkun/redir"`,
		"golang.design": `content="golang.design/x/redir git https://github.com/golang-design/redir"`,
	} {
		if w := serve(h, host, "/x/redir?go-get=1"); !strings.Contains(w.Body.String(), want) {
			t.Errorf("%s/x/redir: got %d %q, want %s", host, w.Code, w.Body.String(), want)
		}
	}
	if w := serve(h, "changkun.de", "/x/redir"); !strings.Contains(w.Body.String(), `url=https://pkg.go.dev/changkun.de/x/redir"`) {
		t.Errorf("changkun.de/x/redir no longer sends a browser to its documentation: %q", w.Body.String())
	}
}

// TestUnsetPrefixGatesNothing checks that a deployment without /x/, which
// may leave x.prefix unset, still serves its short links: an empty prefix
// is a prefix of every path, and gating on it closed the whole site.
func TestUnsetPrefixGatesNothing(t *testing.T) {
	withSites(t)
	config.Conf.X.Enable = false
	config.Conf.X.Prefix = ""
	h := (&server{}).handler()
	// The mux redirects /s to /s/ without reaching the store.
	if w := serve(h, "changkun.de", "/s"); w.Header().Get("Location") != "/s/" {
		t.Errorf("changkun.de/s: got %d to %q, want the redirect to /s/", w.Code, w.Header().Get("Location"))
	}
}

// TestCallbackURLsSkipSitesWithoutLinks checks that a site without short
// links gets no login: it has no console to sign in to, and its callback
// would have to be registered with the authorization server for nothing.
func TestCallbackURLsSkipSitesWithoutLinks(t *testing.T) {
	withSites(t)
	t.Setenv("AUTH_REDIRECT_URL", "")
	got := callbackURLs("/s/")
	if len(got) != 2 || got["changkun.de"] == "" || got["golang.design"] == "" {
		t.Errorf("callbacks for %v, want changkun.de and golang.design only", got)
	}
}
