// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"changkun.de/x/redir/internal/config"
	"latere.ai/x/pkg/oidc"
)

func TestPrincipalSet(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"  ", nil},
		{"hi@changkun.de", []string{"hi@changkun.de"}},
		{"A@B.de, C@D.de", []string{"a@b.de", "c@d.de"}},
		{"a@b.de,,  ,c@b.de ", []string{"a@b.de", "c@b.de"}},
	} {
		got := principalSet(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("principalSet(%q) has %v entries, want %v: %v",
				tt.in, len(got), len(tt.want), got)
			continue
		}
		for _, w := range tt.want {
			if !got[w] {
				t.Errorf("principalSet(%q) is missing %q, got %v", tt.in, w, got)
			}
		}
	}
}

// TestPermit pins the split between authentication and authorization: a
// token from auth.latere.ai proves who is calling, and the allowlist alone
// decides whether that principal may administer this deployment.
func TestPermit(t *testing.T) {
	a := &latereAuth{allowed: principalSet("hi@changkun.de, some-principal-id")}

	for _, tt := range []struct {
		name string
		user *oidc.User
		want string
	}{
		{"allowed by email", &oidc.User{Sub: "s1", Email: "hi@changkun.de"}, "hi@changkun.de"},
		{"email match is case insensitive", &oidc.User{Sub: "s1", Email: "HI@Changkun.DE"}, "HI@Changkun.DE"},
		{"allowed by principal id", &oidc.User{Sub: "some-principal-id"}, "some-principal-id"},
		{"a valid but unlisted principal is refused", &oidc.User{Sub: "s2", Email: "eve@example.com"}, ""},
		{"no session", nil, ""},
	} {
		if got := a.permit(tt.user); got != tt.want {
			t.Errorf("%s: permit() = %q, want %q", tt.name, got, tt.want)
		}
	}

	// An unconfigured relying party must refuse everyone rather than fall
	// through to an anonymous session.
	var off *latereAuth
	if got := off.permit(&oidc.User{Email: "hi@changkun.de"}); got != "" {
		t.Errorf("unconfigured latereAuth admitted %q, want a refusal", got)
	}
	if got := off.user(httptest.NewRecorder(), httptest.NewRequest("GET", "/s/", nil)); got != "" {
		t.Errorf("unconfigured latereAuth named %q, want a refusal", got)
	}
}

// testAuth builds a relying party pointed at a fake auth service. No
// request reaches it: HandleLogin only needs the URL to redirect to.
func testAuth(t *testing.T) *latereAuth {
	t.Helper()
	c := oidc.New(oidc.Config{
		AuthURL:     "https://auth.example.test",
		ClientID:    "redir-test",
		RedirectURL: "https://changkun.de/s/" + callbackPath,
		CookieKey:   "0123456789abcdef0123456789abcdef",
	})
	if c == nil {
		t.Fatal("cannot build a test oidc client")
	}
	return &latereAuth{
		clients: map[string]*oidc.Client{config.Conf.Hostname(): c},
		allowed: principalSet("hi@changkun.de"),
	}
}

// TestLoginRoute checks that the login route reaches the relying party and
// starts an authorization code flow with PKCE, rather than being swallowed
// by the alias lookup.
func TestLoginRoute(t *testing.T) {
	s := &server{latere: testAuth(t)}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/s/"+loginPath+"?return_to=%2Fs%2F%3Fmode%3Dadmin", nil)

	if err := s.serveStatic(t.Context(), w, r, "/s/"); err != nil {
		t.Fatalf("serveStatic: %v", err)
	}
	if w.Code != http.StatusFound {
		t.Fatalf("login route returned %v, want %v", w.Code, http.StatusFound)
	}

	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("cannot parse the redirect: %v", err)
	}
	if got, want := loc.Host, "auth.example.test"; got != want {
		t.Errorf("login redirected to %q, want the auth service at %q", got, want)
	}
	q := loc.Query()
	for k, want := range map[string]string{
		"response_type":         "code",
		"client_id":             "redir-test",
		"code_challenge_method": "S256",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("authorize %v = %q, want %q", k, got, want)
		}
	}
	if q.Get("code_challenge") == "" {
		t.Error("authorize request carries no PKCE code_challenge")
	}
}

// TestHandleAuthSendsAnonymousToLogin covers the path a visitor takes when
// they open the admin dashboard without a session.
func TestHandleAuthSendsAnonymousToLogin(t *testing.T) {
	prev := config.Conf.Auth.Enable
	config.Conf.Auth.Enable = config.Latere
	t.Cleanup(func() { config.Conf.Auth.Enable = prev })

	s := &server{latere: testAuth(t)}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/s/?mode=admin", nil)

	user, err := s.handleAuth(w, r)
	if err == nil {
		t.Fatal("handleAuth accepted a request with no session")
	}
	if user != "" {
		t.Errorf("handleAuth named %q for an anonymous request", user)
	}
	if w.Code != http.StatusFound {
		t.Fatalf("handleAuth returned %v, want a %v to the login route", w.Code, http.StatusFound)
	}

	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/s/"+loginPath) {
		t.Errorf("handleAuth redirected to %q, want the login route", loc)
	}
	// The visitor has to land back where they started.
	if got := mustQuery(t, loc).Get("return_to"); got != "/s/?mode=admin" {
		t.Errorf("return_to = %q, want the originally requested page", got)
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("cannot parse %q: %v", raw, err)
	}
	return u.Query()
}

// TestHandleAuthUnknownModeRefuses covers a stale or mistyped auth.enable.
// Administration must fail closed rather than quietly fall through to a
// different login. Public redirects do not go through handleAuth and keep
// working.
func TestHandleAuthUnknownModeRefuses(t *testing.T) {
	prev := config.Conf.Auth.Enable
	config.Conf.Auth.Enable = "no-such-mode"
	t.Cleanup(func() { config.Conf.Auth.Enable = prev })

	s := &server{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/s/?mode=admin", nil)

	user, err := s.handleAuth(w, r)
	if err == nil {
		t.Fatal("handleAuth accepted a request under an unknown auth mode")
	}
	if user != "" {
		t.Errorf("handleAuth named %q under an unknown auth mode", user)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("handleAuth returned %v, want %v", w.Code, http.StatusForbidden)
	}
	// It must not have offered basic auth as a fallback.
	if got := w.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("handleAuth fell through to basic auth: WWW-Authenticate=%q", got)
	}
}

// TestCallbackURLsPerSite covers the fault that stopped logins working on
// the second site.
//
// A relying party's callback is fixed when its client is built, and it was
// derived once from the configured host. A login started on another site
// therefore sent the visitor to the primary site's callback: they came
// back on the wrong domain, and the session cookie, which carries the
// "__Host-" prefix and so is bound to one origin, was set there instead of
// on the site they were signing in to.
func TestCallbackURLsPerSite(t *testing.T) {
	saved := config.Conf
	t.Cleanup(func() { config.Conf = saved })
	t.Setenv("AUTH_REDIRECT_URL", "")

	config.Conf.Host = "https://changkun.de"
	config.Conf.Hosts = map[string]config.SiteOverride{
		"golang.design": {RepoPath: "https://github.com/golang-design"},
	}

	got := callbackURLs("/s/")
	want := map[string]string{
		"changkun.de":   "https://changkun.de/s/.auth/callback",
		"golang.design": "https://golang.design/s/.auth/callback",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d callbacks, want %d: %v", len(got), len(want), got)
	}
	for host, url := range want {
		if got[host] != url {
			t.Errorf("callback for %v = %q, want %q", host, got[host], url)
		}
	}
}

// TestCallbackURLsHonourTheOverride keeps AUTH_REDIRECT_URL working for a
// deployment behind a different public name. Only one value can be given
// that way, so it applies to the primary site and the others are derived.
func TestCallbackURLsHonourTheOverride(t *testing.T) {
	saved := config.Conf
	t.Cleanup(func() { config.Conf = saved })
	t.Setenv("AUTH_REDIRECT_URL", "https://proxy.example/s/.auth/callback")

	config.Conf.Host = "https://changkun.de"
	config.Conf.Hosts = map[string]config.SiteOverride{
		"golang.design": {},
	}

	got := callbackURLs("/s/")
	if got["changkun.de"] != "https://proxy.example/s/.auth/callback" {
		t.Errorf("the override was ignored: %q", got["changkun.de"])
	}
	if got["golang.design"] != "https://golang.design/s/.auth/callback" {
		t.Errorf("the second site = %q", got["golang.design"])
	}
}

// TestCallbackURLsFollowTheScheme checks a development deployment on http
// does not derive https callbacks for its other sites.
func TestCallbackURLsFollowTheScheme(t *testing.T) {
	saved := config.Conf
	t.Cleanup(func() { config.Conf = saved })
	t.Setenv("AUTH_REDIRECT_URL", "")

	config.Conf.Host = "http://localhost:9123"
	config.Conf.Hosts = map[string]config.SiteOverride{"other.local": {}}

	if got := callbackURLs("/s/")["other.local"]; got != "http://other.local/s/.auth/callback" {
		t.Errorf("callback = %q, want the configured scheme", got)
	}
}

// TestClientForSelectsTheSite checks a request is answered by its own
// site's relying party, and that an unrecognised Host header falls back to
// the primary one rather than to nothing.
func TestClientForSelectsTheSite(t *testing.T) {
	saved := config.Conf
	t.Cleanup(func() { config.Conf = saved })
	config.Conf.Host = "https://changkun.de"
	config.Conf.Hosts = map[string]config.SiteOverride{"golang.design": {}}

	primary := &oidc.Client{}
	second := &oidc.Client{}
	a := &latereAuth{clients: map[string]*oidc.Client{
		"changkun.de":   primary,
		"golang.design": second,
	}}

	for _, tt := range []struct {
		host string
		want *oidc.Client
	}{
		{"changkun.de", primary},
		{"golang.design", second},
		{"golang.design:443", second},
		{"evil.example", primary},
		{"", primary},
	} {
		r := httptest.NewRequest(http.MethodGet, "/s/.auth/login", nil)
		r.Host = tt.host
		if got := a.clientFor(r); got != tt.want {
			t.Errorf("clientFor(%q) picked the wrong site", tt.host)
		}
	}

	// A nil relying party must not panic: the routes are only reached
	// when latere is configured, but the method is called from handleAuth
	// on every admin request.
	var nilAuth *latereAuth
	if nilAuth.clientFor(httptest.NewRequest(http.MethodGet, "/", nil)) != nil {
		t.Error("a nil latereAuth returned a client")
	}
}
