// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"cmp"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"changkun.de/x/redir/internal/config"
	"latere.ai/x/pkg/oidc"
)

// Routes for the login flow. They sit under the shortener prefix beside
// .static and .impressum; an alias may not begin with a dot, so these can
// never collide with a short link.
const (
	loginPath    = ".auth/login"
	callbackPath = ".auth/callback"
	logoutPath   = ".auth/logout"
)

// latereAuth is the relying party for auth.latere.ai. It runs the
// authorization code + PKCE flow in the browser and keeps the result in an
// encrypted session cookie, so handleAuth can name the visitor.
type latereAuth struct {
	client  *oidc.Client
	allowed map[string]bool // lowercased email or principal id (sub)
}

// newLatereAuth builds the relying party from the environment. It returns
// nil when the deployment is not configured for it, and every request then
// fails closed rather than falling back to an anonymous session.
func newLatereAuth(prefix string) *latereAuth {
	allowed := principalSet(os.Getenv("AUTH_ALLOWED_PRINCIPALS"))
	if len(allowed) == 0 {
		log.Println("AUTH_ALLOWED_PRINCIPALS is unset, latere logins will be rejected")
		return nil
	}

	cfg := oidc.LoadConfig()
	// A relying party needs a callback it can be reached on. Derive it from
	// the configured host so a deployment only has to set AUTH_REDIRECT_URL
	// when it sits behind a different public name.
	cfg.RedirectURL = cmp.Or(cfg.RedirectURL,
		strings.TrimRight(config.Conf.Host, "/")+prefix+callbackPath)

	c := oidc.New(cfg)
	if c == nil {
		// oidc.New logs the specific reason: no AUTH_CLIENT_ID, or a public
		// client without AUTH_COOKIE_KEY to encrypt the session with.
		log.Println("latere auth is not configured, logins will be rejected")
		return nil
	}

	log.Printf("latere auth enabled: %v principals, callback %v",
		len(allowed), cfg.RedirectURL)
	return &latereAuth{client: c, allowed: allowed}
}

// principalSet parses a comma separated list of emails or principal ids.
func principalSet(s string) map[string]bool {
	out := map[string]bool{}
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			out[p] = true
		}
	}
	return out
}

// user returns the signed in principal, or the empty string. A token from
// auth.latere.ai proves who is calling, not that they may administer this
// deployment, so the allowlist decides separately.
func (a *latereAuth) user(w http.ResponseWriter, r *http.Request) string {
	if a == nil {
		return ""
	}
	return a.permit(a.client.UserFromRequest(w, r))
}

// permit names the principal when the allowlist admits it, and returns the
// empty string otherwise. Split from user so the decision can be tested
// without an auth service.
func (a *latereAuth) permit(u *oidc.User) string {
	if a == nil || u == nil {
		return ""
	}
	if a.allowed[strings.ToLower(u.Email)] || a.allowed[strings.ToLower(u.Sub)] {
		return cmp.Or(u.Email, u.Sub)
	}
	log.Printf("latere principal not allowed: sub=%v email=%v", u.Sub, u.Email)
	return ""
}

// logoutURL is the route that ends a latere session, or the empty string
// when the deployment has no session to end.
func logoutURL() string {
	if config.Conf.Auth.Enable != config.Latere {
		return ""
	}
	return config.Conf.S.Prefix + logoutPath
}

// loginURL sends the visitor to the login route, remembering where they
// were so the callback can put them back.
func (a *latereAuth) loginURL(prefix string, r *http.Request) string {
	q := url.Values{}
	q.Set("return_to", r.URL.RequestURI())
	return prefix + loginPath + "?" + q.Encode()
}
