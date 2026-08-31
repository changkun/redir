// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package models

import "testing"

// TestParseUABrowserOrder covers the reason the browser list is ordered.
// Nearly every browser claims to be the ones before it: Edge announces
// Chrome and Safari, Chrome announces Safari, and all announce Mozilla.
func TestParseUABrowserOrder(t *testing.T) {
	for _, tt := range []struct{ name, ua, want string }{
		{"edge", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0", "Edge"},
		{"opera", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 OPR/105.0.0.0", "Opera"},
		{"vivaldi", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Vivaldi/6.5", "Vivaldi"},
		{"yandex", "Mozilla/5.0 (Linux; Android 10; SM-G965F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 YaBrowser/22.9 Mobile Safari/537.36", "YaBrowser"},
		{"samsung", "Mozilla/5.0 (Linux; Android 12; SM-S908B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/19.0 Chrome/117.0.0.0 Mobile Safari/537.36", "Samsung Browser"},
		{"wechat", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.98 Mobile Safari/537.36 MicroMessenger/6.6", "MicroMessenger"},
		{"chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Chrome"},
		{"chrome on ios", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.0.0 Mobile/15E148 Safari/604.1", "Chrome"},
		{"firefox", "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0", "Firefox"},
		{"safari", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15", "Safari"},
		{"ie11", "Mozilla/5.0 (Windows NT 6.1; Trident/7.0; rv:11.0) like Gecko", "Internet Explorer"},
		// An Apple platform has one engine, so an embedded web view
		// there is Safari's even when it names no browser.
		{"ios webview", "Mozilla/5.0 (iPhone; CPU iPhone OS 14_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148", "Safari"},
		{"curl", "curl/8.4.0", "curl"},
		{"empty", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseUA(tt.ua).Name; got != tt.want {
				t.Errorf("Name = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseUAOSFromPlatformComment covers a fault the comparison against
// production data found: searching the whole string for a platform token
// finds it inside product names. "LinuxGetSsl/1.0" is a tool, and reading
// it as a browser on Linux gave it a platform, which hid it from the rule
// that classifies platformless agents as automated.
func TestParseUAOSFromPlatformComment(t *testing.T) {
	for _, tt := range []struct{ ua, want string }{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", "Windows"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15", "macOS"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15", "iOS"},
		{"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36", "Android"},
		{"Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36", "ChromeOS"},
		{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36", "Linux"},
		// The platform is only ever in the first comment.
		{"LinuxGetSsl/1.0", ""},
		{"WindowsPowerShell/5.1", ""},
		{"curl/8.4.0", ""},
		{"", ""},
	} {
		if got := parseUA(tt.ua).OS; got != tt.want {
			t.Errorf("OS(%.48s) = %q, want %q", tt.ua, got, tt.want)
		}
	}
}

// TestParseUADevice pins the Android convention: a phone says Mobile and
// a tablet does not. The library this replaced read both as mobile, which
// put Galaxy Tabs and 10-inch tablets in the phone bucket.
func TestParseUADevice(t *testing.T) {
	for _, tt := range []struct{ name, ua, want string }{
		{"android phone", "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", "mobile"},
		{"android tablet", "Mozilla/5.0 (Linux; Android 4.2.2; GT-P5110 Build/JDQ39) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/43.0.2357.93 Safari/537.36", "tablet"},
		{"ipad", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/604.1", "tablet"},
		{"iphone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "mobile"},
		{"desktop", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "desktop"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := parseUA(tt.ua)
			if got := deviceKind(a, false); got != tt.want {
				t.Errorf("device = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestLeadingTokenNamesCompatibleAgents covers how a crawler that claims
// to be Mozilla identifies itself. Discarding the name because the string
// starts with Mozilla lost the only signal distinguishing these from a
// browser, and stopped them being classified as automated.
func TestLeadingTokenNamesCompatibleAgents(t *testing.T) {
	for _, tt := range []struct{ ua, want string }{
		{"Mozilla/5.0 (compatible; Dataprovider.com)", "Dataprovider.com"},
		{"Mozilla/5.0 (compatible; Y!J-WSC/1.0; +https://yahoo.jp/3BSZgF)", "Y!J-WSC"},
		{"curl/8.4.0", "curl"},
		{"Go-http-client/1.1", "Go-http-client"},
		{"Mozilla/5.0", ""},
	} {
		if got := leadingToken(tt.ua); got != tt.want {
			t.Errorf("leadingToken(%.48s) = %q, want %q", tt.ua, got, tt.want)
		}
	}
}
