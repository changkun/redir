// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package models

import "strings"

// agent is what the derivation needs from a user agent string: a browser
// name, an operating system, a device kind and whether it is automated.
//
// This replaces a general purpose parser. Only the fields above are ever
// read, and the accuracy that matters is on the traffic that actually
// arrives, which is measured against every distinct user agent in the
// store rather than against a specification.
type agent struct {
	Name    string
	OS      string
	Mobile  bool
	Tablet  bool
	Desktop bool
	Bot     bool
}

// browsers are matched in order, because nearly every browser claims to
// be the ones that came before it: Edge announces Chrome and Safari,
// Chrome announces Safari, and all of them announce Mozilla. The first
// token that matches is the real one, so the list runs from most specific
// to least.
var browsers = []struct {
	token string
	name  string
}{
	{"Edg/", "Edge"}, {"EdgA/", "Edge"}, {"EdgiOS/", "Edge"}, {"Edge/", "Edge"},
	{"OPR/", "Opera"}, {"OPiOS/", "Opera"}, {"Opera", "Opera"},
	{"Vivaldi/", "Vivaldi"},
	{"YaBrowser/", "YaBrowser"},
	{"SamsungBrowser/", "Samsung Browser"},
	{"HuaweiBrowser/", "Huawei Browser"},
	{"MiuiBrowser/", "Miui Browser"},
	{"MicroMessenger/", "MicroMessenger"},
	{"FxiOS/", "Firefox"}, {"Firefox/", "Firefox"},
	{"CriOS/", "Chrome"}, {"Chrome/", "Chrome"}, {"Chromium/", "Chrome"},
	{"Version/", "Safari"}, // only reached when no Chromium token matched
	{"MSIE ", "Internet Explorer"}, {"Trident/", "Internet Explorer"},
}

// operatingSystems are matched in order for the same reason: an Android
// user agent also says Linux, and an iOS one also says Mac OS X.
var operatingSystems = []struct {
	token string
	name  string
}{
	{"CrOS", "ChromeOS"},
	{"Android", "Android"},
	{"iPhone", "iOS"}, {"iPad", "iOS"}, {"iPod", "iOS"}, {"iOS", "iOS"},
	{"Windows", "Windows"},
	{"Mac OS X", "macOS"}, {"Macintosh", "macOS"},
	{"Linux", "Linux"}, {"X11", "Linux"}, {"FreeBSD", "Linux"},
}

// platformComment is the first parenthesised group, which by convention
// holds the platform: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) ...".
// An agent without one is naming no platform at all.
func platformComment(s string) string {
	_, rest, ok := strings.Cut(s, "(")
	if !ok {
		return ""
	}
	inside, _, _ := strings.Cut(rest, ")")
	return inside
}

// parseUA reads the fields the statistics group by.
func parseUA(s string) agent {
	var a agent
	if s == "" {
		return a
	}

	// The platform is only ever named in the first parenthesised
	// comment. Searching the whole string finds it inside product names
	// too: "LinuxGetSsl/1.0" is a tool, not a browser on Linux, and
	// reading it as one gave it a platform and hid it from the rule below.
	platform := platformComment(s)
	for _, o := range operatingSystems {
		if strings.Contains(platform, o.token) {
			a.OS = o.name
			break
		}
	}
	for _, b := range browsers {
		if strings.Contains(s, b.token) {
			a.Name = b.name
			break
		}
	}

	// Apple's platforms have one engine, and an app embedding a web view
	// there is running Safari's. A user agent on iOS or macOS that names
	// AppleWebKit and no browser is one of those, not an unidentifiable
	// client.
	if a.Name == "" && strings.Contains(s, "AppleWebKit") &&
		(a.OS == "iOS" || a.OS == "macOS") {
		a.Name = "Safari"
	}

	// A user agent that names no known browser is either a tool or
	// something too obscure to identify. Its own first token is the best
	// name available, and it is what distinguishes "curl" from "lychee"
	// for the bot rules below.
	if a.Name == "" {
		a.Name = leadingToken(s)
	}

	switch {
	case strings.Contains(s, "iPad") || strings.Contains(s, "Tablet"),
		a.OS == "Android" && !strings.Contains(s, "Mobile"):
		// Android without the Mobile token is the convention for a
		// tablet.
		a.Tablet = true
	case strings.Contains(s, "Mobile") || strings.Contains(s, "iPhone") ||
		strings.Contains(s, "iPod"):
		a.Mobile = true
	case a.OS != "":
		// A platform was named and nothing says otherwise.
		a.Desktop = true
	}

	a.Bot = isKnownBot(s)
	return a
}

// leadingToken is the product name a client puts first, as in "curl/8.4.0"
// or "Go-http-client/1.1".
//
// An agent claiming to be Mozilla has already failed to match a browser,
// so "Mozilla" itself says nothing. Such an agent conventionally names
// itself in a "(compatible; NAME...)" comment instead, which is how
// "Mozilla/5.0 (compatible; Dataprovider.com)" identifies itself. That
// name is what tells a crawler apart from a browser, so it is worth
// digging out rather than leaving empty.
func leadingToken(s string) string {
	if !strings.HasPrefix(s, "Mozilla") {
		name, _, _ := strings.Cut(s, "/")
		name, _, _ = strings.Cut(name, " ")
		return strings.TrimSpace(name)
	}

	_, rest, ok := strings.Cut(s, "compatible;")
	if !ok {
		return ""
	}
	rest = strings.TrimSpace(rest)
	for _, sep := range []string{";", ")", "/", " "} {
		rest, _, _ = strings.Cut(rest, sep)
	}
	return strings.TrimSpace(rest)
}

// knownBots are crawlers whose names carry no marker from botMarkers.
//
// Each token has to be unambiguous on its own, because it is matched
// against the whole string. A first draft listed vendor names such as
// "baidu" and "sogou"; those appear inside real browsers' user agents
// ("baidu.sogo.uc.UCBrowser" is UC Browser), which classified people as
// crawlers. Vendors whose crawlers spell "bot" are already covered by
// botMarkers and need no entry here.
var knownBots = []string{
	"yeti/",                 // Naver
	"googleother",           // Google, fetches without saying "bot"
	"google-inspectiontool", // Search Console
	"chrome-lighthouse",
	"pingdom", "phantomjs",
	"y!j-", // Yahoo Japan
}

// isKnownBot matches a crawler by name. The general rules in isBot catch
// almost everything; on the traffic in the store these names account for
// 0.1% of automated visits, which is the entire gap a general purpose
// parser was covering.
func isKnownBot(s string) bool {
	lower := strings.ToLower(s)
	for _, b := range knownBots {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}
