// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package config

import (
	"cmp"
	_ "embed"
	"fmt"
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
	// An entry carries only what differs from the settings above: the VCS
	// organisation, whether the legal pages apply, which of the services
	// the site offers, and the import paths it serves.
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
	// Links and Repos, set to false, turn off a service the site does not
	// offer: its short links under s.prefix, or its repositories under
	// x.prefix. Unset, a site offers both, as the primary one does. That
	// golang.design does too is its choice, not a rule: a site may be a
	// module path and nothing else.
	Links *bool `yaml:"links"`
	Repos *bool `yaml:"repos"`
	// Imports are Go import paths of the site, each served from a
	// repository of its own rather than from the tree under x.prefix,
	// such as poly.red from github.com/changkun/polyred. They are keyed
	// by import path, which starts with the site's hostname.
	Imports map[string]Import `yaml:"imports"`
}

// Import is a Go import path served from one repository: the path itself
// and every package below it.
type Import struct {
	// Repo is the repository that holds the code.
	Repo string `yaml:"repo"`
	// Web is where a browser asking for the import path is sent. Unset,
	// it is the package's documentation on x.godoc_host, as for x.prefix.
	Web string `yaml:"web"`
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
	// Links and Repos report whether the site offers short links under
	// s.prefix and repositories under x.prefix.
	Links bool
	Repos bool
	// Imports are the import paths the site serves one by one, keyed by
	// import path.
	Imports map[string]Import
}

// ImportFor returns the import path of the site that a request for path,
// a URL path, is for: the longest one that path is or is below. suffix is
// the rest of path, such as /geometry for a package of the module.
func (s Site) ImportFor(path string) (importPath string, imp Import, suffix string, ok bool) {
	full := s.Host + strings.TrimSuffix(path, "/")
	for p, i := range s.Imports {
		if (full == p || strings.HasPrefix(full, p+"/")) && len(p) > len(importPath) {
			importPath, imp, ok = p, i, true
		}
	}
	if !ok {
		return "", Import{}, "", false
	}
	return importPath, imp, full[len(importPath):], true
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
		Links:         true,
		Repos:         c.X.Enable,
	}

	n := NormalizeHost(requestHost)
	if n == "" || n == primary.Host {
		return primary
	}
	for name, o := range c.Hosts {
		if n != NormalizeHost(name) {
			continue
		}
		s := Site{
			Host:     n,
			RepoPath: cmp.Or(o.RepoPath, c.X.RepoPath),
			Links:    o.Links == nil || *o.Links,
			Repos:    c.X.Enable && (o.Repos == nil || *o.Repos),
			Imports:  o.Imports,
		}
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
	if err := c.validate(); err != nil {
		log.Fatalf("invalid configuration: %v\n", err)
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

// validate reports a configuration that would serve something other than
// what it says. An import path on another site's hostname would never be
// asked for, and one without a repository would answer go get with nothing.
func (c *config) validate() error {
	for name, o := range c.Hosts {
		host := NormalizeHost(name)
		for p, i := range o.Imports {
			if p != host && !strings.HasPrefix(p, host+"/") {
				return fmt.Errorf("hosts.%s.imports: %q is not an import path of %s", name, p, host)
			}
			if strings.HasSuffix(p, "/") {
				return fmt.Errorf("hosts.%s.imports: %q ends with a slash", name, p)
			}
			if u, err := url.Parse(i.Repo); err != nil || u.Scheme == "" || u.Host == "" {
				return fmt.Errorf("hosts.%s.imports.%s: repo %q is not a URL", name, p, i.Repo)
			}
		}
	}
	return nil
}

var Conf config

func init() {
	Conf.parse()
}
