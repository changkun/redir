// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package models

import (
	"net/url"
	"strings"
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

	ua := parseUA(v.UA)
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

// botMarkers are substrings that identify automated traffic.
//
// "crawl" rather than "crawler", because an agent announcing itself as
// "crawled for <url>" is one too. BingPreview is the case that motivated this: it announces itself
// as a desktop Windows browser and contains neither "bot" nor "crawler",
// so both the parser and the dashboard's old strings.Contains(ua, "bot")
// test counted it as a person.
var botMarkers = []string{
	"bot", "crawl", "spider", "scraper", "scan", "slurp", "preview",
	"monitor", "uptime", "validator", "archive", "headless", "feedfetcher",
	"httrack", "wget", "fetcher", "sitecheck",
}

// isBot supplements the parser rather than replacing it. The parser is
// right about what it does flag; it is only incomplete.
func isBot(ua agent, raw string) bool {
	if httpClients[ua.Name] {
		return true
	}
	lower := strings.ToLower(raw)
	for _, m := range botMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return platformless(raw, ua)
}

// platformless catches agents that claim no platform, such as
// "Mozilla/5.0 (compatible; Dataprovider.com)" or a bare "Mozilla/5.0".
//
// A browser runs on something, and every platform a browser reports is
// recognised. An agent that names no operating system and no device is
// announcing a tool, not a person, and it is the shape every crawler too
// obscure to be in a list arrives in. Matching on the shape rather than
// on names means the next one is caught without an edit.
//
// An empty user agent is excluded. It says nothing either way, and
// treating silence as evidence would reclassify 16,007 recorded visits on
// no information at all.
func platformless(raw string, ua agent) bool {
	return raw != "" && ua.OS == "" &&
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
func deviceKind(ua agent, bot bool) string {
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
