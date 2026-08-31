// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package models

import (
	"net/url"
	"strings"

	"github.com/mileusna/useragent"
)

// Derive fills the columns computed from UA and Referer.
//
// It runs once, where the visit is recorded, rather than in the browser on
// every dashboard load. The dashboard used to receive every distinct user
// agent string and parse them client side, which shipped the raw strings
// over the wire only to discard most of them, and decided what a bot was
// with a substring test for "bot".
//
// Both the server and the data migration call this, so a row written by a
// live redirect and a row written by the migration are identical.
func (v *Visit) Derive() {
	v.RefererHost = refererHost(v.Referer)

	ua := useragent.Parse(v.UA)
	v.IsBot = ua.Bot || isBot(ua, v.UA)
	v.Browser = ua.Name
	v.OS = ua.OS
	v.Device = deviceKind(ua, v.IsBot)
}

// httpClients are non-browser agents the parser names correctly but does
// not flag, because they are clients rather than crawlers. For counting
// visitors they are the same thing: nobody is reading the page.
var httpClients = map[string]bool{
	"curl": true, "Wget": true, "python-requests": true, "Go-http-client": true,
	"HTTPie": true, "okhttp": true, "axios": true, "node-fetch": true,
	"PostmanRuntime": true, "Apache-HttpClient": true, "libwww-perl": true,
	"Java": true, "aiohttp": true, "Guzzle": true, "lychee": true,
}

// botMarkers are substrings that identify automated traffic the parser
// misses. BingPreview is the case that motivated this: it announces itself
// as a desktop Windows browser and contains neither "bot" nor "crawler",
// so both the parser and the dashboard's old strings.Contains(ua, "bot")
// test counted it as a person.
var botMarkers = []string{
	"bot", "crawler", "spider", "scraper", "scan", "slurp", "preview",
	"monitor", "uptime", "validator", "archive", "headless", "feedfetcher",
}

// isBot supplements the parser rather than replacing it. The parser is
// right about what it does flag; it is only incomplete.
func isBot(ua useragent.UserAgent, raw string) bool {
	if httpClients[ua.Name] {
		return true
	}
	lower := strings.ToLower(raw)
	for _, m := range botMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return namedButPlatformless(ua)
}

// namedButPlatformless catches agents that identify themselves but claim
// no platform, such as "Mozilla/5.0 (compatible; Dataprovider.com)".
//
// A browser runs on something, and the parser recognises every platform a
// browser reports. An agent that gives a name while reporting no
// operating system and no device is announcing a tool, not a person, and
// it is the shape every crawler too obscure to be in a list arrives in.
// Matching on the shape rather than on names means the next one is caught
// without an edit.
//
// The name must be present: an empty user agent says nothing either way
// and is left alone.
func namedButPlatformless(ua useragent.UserAgent) bool {
	return ua.Name != "" && ua.OS == "" &&
		!ua.Mobile && !ua.Tablet && !ua.Desktop
}

// refererHost reduces a referer to its hostname, so that one referring
// page does not become many rows because of its query parameters. An
// unparseable or empty referer yields the empty string, which the queries
// report as a direct visit.
func refererHost(ref string) string {
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// deviceKind collapses the parser's flags into one value, because the
// charts show a single device breakdown rather than three booleans.
//
// It takes the decided bot flag rather than reading ua.Bot, so a crawler
// that announces itself as a desktop browser, which several do, does not
// end up counted as a desktop.
func deviceKind(ua useragent.UserAgent, bot bool) string {
	switch {
	case bot:
		return "bot"
	case ua.Tablet:
		return "tablet"
	case ua.Mobile:
		return "mobile"
	case ua.Desktop:
		return "desktop"
	default:
		return ""
	}
}
