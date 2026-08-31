// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package models_test

import (
	"testing"

	"changkun.de/x/redir/internal/models"
)

// TestDeriveBot covers the reason the derived columns exist: the dashboard
// used to decide what a bot was with strings.Contains(ua, "bot"), which
// misses every crawler that does not spell it that way.
func TestDeriveBot(t *testing.T) {
	for _, tt := range []struct {
		name string
		ua   string
		want bool
	}{
		{"googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", true},
		// Neither of the next two contains "bot", so the old substring
		// test counted them as people.
		{"bingpreview", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/534+ (KHTML, like Gecko) BingPreview/1.0b", true},
		{"yandex", "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)", true},
		{"curl", "curl/8.4.0", true},
		{"chrome", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", false},
		// Names itself but claims no platform. A browser runs on
		// something, so this shape is a tool, and it is how crawlers too
		// obscure for any list arrive.
		{"named without a platform", "Mozilla/5.0 (compatible; Dataprovider.com)", true},
		{"link checker", "lychee/0.8.0", true},
		{"safari ios", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &models.Visit{UA: tt.ua}
			v.Derive()
			if v.IsBot != tt.want {
				t.Fatalf("IsBot = %v, want %v for %q", v.IsBot, tt.want, tt.ua)
			}
		})
	}
}

func TestDeriveBrowserOSDevice(t *testing.T) {
	v := &models.Visit{
		UA: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	v.Derive()
	if v.Browser != "Chrome" {
		t.Errorf("Browser = %q, want Chrome", v.Browser)
	}
	if v.OS != "macOS" {
		t.Errorf("OS = %q, want macOS", v.OS)
	}
	if v.Device != "desktop" {
		t.Errorf("Device = %q, want desktop", v.Device)
	}

	m := &models.Visit{
		UA: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	}
	m.Derive()
	if m.Device != "mobile" {
		t.Errorf("Device = %q, want mobile", m.Device)
	}
}

// TestDeriveRefererHost is why referer_host exists: one referring page
// with varying query parameters used to become one row per variant.
func TestDeriveRefererHost(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"https://News.YCombinator.com/item?id=1", "news.ycombinator.com"},
		{"https://news.ycombinator.com/item?id=2", "news.ycombinator.com"},
		{"https://golang.design:8080/s/x", "golang.design"},
		{"", ""},
		{"not a url", ""},
	} {
		v := &models.Visit{Referer: tt.in}
		v.Derive()
		if v.RefererHost != tt.want {
			t.Errorf("RefererHost(%q) = %q, want %q", tt.in, v.RefererHost, tt.want)
		}
	}
}

// TestDeriveEmptyUA checks that an absent user agent, which 8,205
// production rows have, does not become a bot or a fabricated browser.
//
// It says nothing either way, so the platformless rule deliberately does
// not apply to it: that rule needs a name to act on.
func TestDeriveEmptyUA(t *testing.T) {
	v := &models.Visit{}
	v.Derive()
	if v.IsBot || v.Browser != "" || v.OS != "" || v.Device != "" {
		t.Fatalf("empty UA derived %+v, want all zero", v)
	}
}

// TestDerivePlatformlessRuleSparesRealBrowsers checks the rule does not
// reach browsers that do report a platform, which is the way it could do
// damage: quietly reclassifying people as bots.
func TestDerivePlatformlessRuleSparesRealBrowsers(t *testing.T) {
	for _, ua := range []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	} {
		v := &models.Visit{UA: ua}
		v.Derive()
		if v.IsBot {
			t.Errorf("a browser reporting a platform was called a bot: %.60s", ua)
		}
	}
}
