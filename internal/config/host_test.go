// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package config_test

import (
	"testing"

	"changkun.de/x/redir/internal/config"
)

// TestNormalizeHost pins the key the store uses. Two spellings of the same
// site must not become two tenants: a link created through one would be
// invisible through the other.
func TestNormalizeHost(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"https://changkun.de", "changkun.de"},
		{"https://changkun.de/", "changkun.de"},
		{"changkun.de", "changkun.de"},
		{"changkun.de:443", "changkun.de"},
		{"CHANGKUN.DE", "changkun.de"},
		{"changkun.de.", "changkun.de"},
		{"redir:80", "redir"},
		{"localhost:9123", "localhost"},
		{"[::1]:9123", "::1"},
		{"https://golang.design/s/", "golang.design"},
		{"", ""},
	} {
		if got := config.NormalizeHost(tt.in); got != tt.want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestSiteForResolvesPerSiteValues covers the values that differ between
// the sites one process serves.
func TestSiteForResolvesPerSiteValues(t *testing.T) {
	c := config.Conf
	t.Cleanup(func() { config.Conf = c })

	config.Conf.Host = "https://changkun.de"
	config.Conf.X.RepoPath = "https://github.com/changkun"
	config.Conf.GDPR.Impressum.Enable = true
	config.Conf.GDPR.Privacy.Enable = true
	config.Conf.GDPR.Contact.Enable = true
	config.Conf.Hosts = map[string]config.SiteOverride{
		"golang.design": {RepoPath: "https://github.com/golang-design"},
	}

	primary := config.Conf.SiteFor("changkun.de")
	if primary.Host != "changkun.de" {
		t.Errorf("primary host = %q", primary.Host)
	}
	if primary.RepoPath != "https://github.com/changkun" {
		t.Errorf("primary repo path = %q", primary.RepoPath)
	}
	if !primary.ShowImpressum || !primary.ShowPrivacy || !primary.ShowContact {
		t.Error("the primary site lost its legal pages")
	}

	second := config.Conf.SiteFor("golang.design")
	if second.Host != "golang.design" {
		t.Errorf("second host = %q", second.Host)
	}
	if second.RepoPath != "https://github.com/golang-design" {
		t.Errorf("second repo path = %q, want the site's own organisation",
			second.RepoPath)
	}
	// The legal pages name an operator. Serving changkun.de's impressum
	// under golang.design would misstate who runs the site.
	if second.ShowImpressum || second.ShowPrivacy || second.ShowContact {
		t.Error("changkun.de's legal pages followed onto golang.design")
	}

	// www and a port are the same site.
	if got := config.Conf.SiteFor("golang.design:443").Host; got != "golang.design" {
		t.Errorf("SiteFor with a port = %q", got)
	}

	// An unrecognised Host header is client-controlled and must not open
	// a namespace of its own, because checkvcs writes rows.
	for _, h := range []string{"evil.example", "", "127.0.0.1:9123"} {
		if got := config.Conf.SiteFor(h); got.Host != "changkun.de" {
			t.Errorf("SiteFor(%q) = %q, want the primary site", h, got.Host)
		}
		if got := config.Conf.SiteFor(h).RepoPath; got != "https://github.com/changkun" {
			t.Errorf("SiteFor(%q) repo path = %q", h, got)
		}
	}
}

// TestSiteForLegalOptIn checks a second site can carry the legal pages if
// it is the same operator.
func TestSiteForLegalOptIn(t *testing.T) {
	c := config.Conf
	t.Cleanup(func() { config.Conf = c })

	config.Conf.Host = "https://changkun.de"
	config.Conf.GDPR.Impressum.Enable = true
	config.Conf.Hosts = map[string]config.SiteOverride{
		"changkun.us": {Legal: true},
	}
	if !config.Conf.SiteFor("changkun.us").ShowImpressum {
		t.Error("legal: true did not enable the impressum")
	}
	// It inherits the base repo path when it does not set one.
	if got := config.Conf.SiteFor("changkun.us").RepoPath; got != config.Conf.X.RepoPath {
		t.Errorf("repo path = %q, want the base %q", got, config.Conf.X.RepoPath)
	}
}

// TestResolveHostMatchesSiteFor checks the two cannot drift, since one is
// defined in terms of the other.
func TestResolveHostMatchesSiteFor(t *testing.T) {
	for _, h := range []string{
		"changkun.de", "golang.design", "evil.example", "", "localhost:9123",
	} {
		if config.Conf.ResolveHost(h) != config.Conf.SiteFor(h).Host {
			t.Errorf("ResolveHost(%q) and SiteFor(%q).Host disagree", h, h)
		}
	}
}
