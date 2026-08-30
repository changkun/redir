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
	return &latereAuth{client: c, allowed: principalSet("hi@changkun.de")}
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

// TestHandleAuthUnknownModeRefuses covers a stale or mistyped auth.enable,
// such as the "sso" this migration replaced. Administration must fail
// closed rather than quietly fall through to a different login. Public
// redirects do not go through handleAuth and keep working.
func TestHandleAuthUnknownModeRefuses(t *testing.T) {
	prev := config.Conf.Auth.Enable
	config.Conf.Auth.Enable = "sso"
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
