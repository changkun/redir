// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package config

import (
	"cmp"
	_ "embed"
	"log"
	"net"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type authType string

var (
	None   authType = "none"
	Basic  authType = "basic"
	Latere authType = "latere"
)

type config struct {
	Title string `yaml:"title"`
	Host  string `yaml:"host"`
	// Hosts are the further sites this instance serves, keyed by
	// hostname. The store keys links by hostname, so each entry is a
	// separate namespace of aliases. Empty means Host is the only site.
	//
	// An entry carries only what differs from the settings above. In
	// practice that is the VCS organisation and whether the legal pages
	// apply; everything else is shared.
	Hosts       map[string]SiteOverride `yaml:"hosts"`
	Addr        string                  `yaml:"addr"`
	Development bool                    `yaml:"development"`
	Store       string                  `yaml:"store"`
	CORS        bool                    `yaml:"cors"`
	S           struct {
		Prefix string `yaml:"prefix"`
	} `yaml:"s"`
	X struct {
		Enable     bool   `yaml:"enable"`
		Prefix     string `yaml:"prefix"`
		VCS        string `yaml:"vcs"`
		ImportPath string `yaml:"import_path"`
		RepoPath   string `yaml:"repo_path"`
		GoDocHost  string `yaml:"godoc_host"`
	} `yaml:"x"`
	Auth struct {
		Enable authType `yaml:"enable"`
		Basic  []struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"basic"`
	} `yaml:"auth"`
	Stats struct {
		Enable bool `yaml:"enable"`
	} `yaml:"stats"`
	GDPR struct {
		HideIP bool `yaml:"hide_ip"`
		Owner  struct {
			Name   string `yaml:"name"`
			Domain string `yaml:"domain"`
		} `yaml:"owner"`
		Contact struct {
			Enable bool   `yaml:"enable"`
			Email  string `yaml:"email"`
		} `yaml:"contact"`
		Impressum struct {
			Enable bool `yaml:"enable"`
			// Content is HTML, rendered into the page as it is. It was
			// markdown converted at start up, which meant carrying a
			// markdown library to render two static documents that
			// change once a year. The renderer had unsafe HTML enabled,
			// so the field was already trusted input and this changes
			// nothing about who may write it.
			Content string `yaml:"content"`
		} `yaml:"impressum"`
		Privacy struct {
			Enable bool `yaml:"enable"`
			// Content is HTML; see Impressum.Content.
			Content string `yaml:"content"`
		} `yaml:"privacy"`
	} `yaml:"gdpr"`
}

// SiteOverride is what one additional site changes.
type SiteOverride struct {
	// RepoPath is the VCS root that /x/ import paths resolve to, and the
	// organisation checkvcs probes when an alias is not found.
	RepoPath string `yaml:"repo_path"`
	// Legal reports whether the impressum, privacy and contact pages
	// apply to this site. They name an operator, so they do not follow a
	// process onto another domain by default.
	Legal bool `yaml:"legal"`
}

// Site is the configuration that applies to one request, resolved from
// its Host header.
//
// Almost nothing is per-site: the import path is built from the request,
// the prefixes are shared, and the title is not rendered. Resolving it in
// one place anyway keeps the handlers from reaching into the base
// configuration for values that do differ.
type Site struct {
	// Host is the hostname the store keys this request's links by.
	Host string
	// RepoPath is the VCS root for this site.
	RepoPath string
	// ShowImpressum, ShowPrivacy and ShowContact gate the legal links.
	ShowImpressum bool
	ShowPrivacy   bool
	ShowContact   bool
}

// SiteFor resolves a request's Host header to the site it belongs to.
//
// The header is chosen by the client and reaches the store as part of a
// link's identity, so it is matched against the configured sites rather
// than trusted. An unrecognised header falls back to the primary site,
// which is what a health check, a direct address, or a local development
// request sends.
func (c *config) SiteFor(requestHost string) Site {
	primary := Site{
		Host:          c.Hostname(),
		RepoPath:      c.X.RepoPath,
		ShowImpressum: c.GDPR.Impressum.Enable,
		ShowPrivacy:   c.GDPR.Privacy.Enable,
		ShowContact:   c.GDPR.Contact.Enable,
	}

	n := NormalizeHost(requestHost)
	if n == "" || n == primary.Host {
		return primary
	}
	for name, o := range c.Hosts {
		if n != NormalizeHost(name) {
			continue
		}
		s := Site{Host: n, RepoPath: cmp.Or(o.RepoPath, c.X.RepoPath)}
		if o.Legal {
			s.ShowImpressum = c.GDPR.Impressum.Enable
			s.ShowPrivacy = c.GDPR.Privacy.Enable
			s.ShowContact = c.GDPR.Contact.Enable
		}
		return s
	}
	return primary
}

//go:embed config.yml
var defaultConf []byte

// Hostname is the configured host reduced to a bare hostname, lowercased
// and without scheme or port.
//
// Host in the configuration is a URL, because it is also used to build
// absolute links. The store keys links and visits by hostname, so that one
// process can serve several sites, and it needs the bare form.
func (c *config) Hostname() string {
	return NormalizeHost(c.Host)
}

// ResolveHost maps a request's Host header to the hostname the store keys
// its links by. It is SiteFor reduced to that one field.
func (c *config) ResolveHost(h string) string {
	return c.SiteFor(h).Host
}

// NormalizeHost reduces a Host header or a configured URL to the form the
// store keys on: lowercase, no scheme, no port, no trailing dot.
func NormalizeHost(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	if u, err := url.Parse(h); err == nil && u.Host != "" {
		h = u.Host
	}
	// Host headers carry a port; IPv6 literals carry brackets.
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	h = strings.Trim(h, "[]")
	return strings.ToLower(strings.TrimSuffix(h, "."))
}

func (c *config) parse() {
	// An unset REDIR_CONF means "run on the built-in defaults". A set one
	// names a file the operator expects to be used, so failing to read it
	// is an error: falling back would quietly serve the sample config,
	// which points at another database and re-enables the sample
	// credentials.
	d := defaultConf
	if f := os.Getenv("REDIR_CONF"); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("cannot read REDIR_CONF %v: %v\n", f, err)
		}
		d = b
	}
	if d == nil {
		log.Fatalln("no configuration to read")
	}
	err := yaml.Unmarshal(d, c)
	if err != nil {
		log.Fatalf("cannot parse configuration: %v\n", err)
	}

	// An unrecognised auth mode used to fall through to basic auth, which
	// turns a typo into a silently different login. Report it here;
	// handleAuth then refuses administration. Serving public redirects
	// does not depend on this setting, so it is not worth refusing to
	// start over.
	switch c.Auth.Enable {
	case None, Basic, Latere:
	default:
		log.Printf("unknown auth.enable %q, administration is disabled; want one of: %v, %v, %v",
			c.Auth.Enable, None, Basic, Latere)
	}

}

var Conf config

func init() {
	Conf.parse()
}
