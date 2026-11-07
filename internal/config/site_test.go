// Copyright 2026 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package config

import (
	"maps"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// deployed is the site part of the configuration changkun.de runs, with
// poly.red added as a site that is a module path and nothing else.
const deployed = `
host: https://changkun.de
hosts:
  golang.design:
    repo_path: https://github.com/golang-design
    legal: false
  poly.red:
    links: false
    repos: false
    imports:
      poly.red:
        repo: https://github.com/changkun/polyred
        web: https://github.com/changkun/polyred
s:
  prefix: /s/
x:
  enable: true
  prefix: /x/
  vcs: git
  repo_path: https://github.com/changkun
  godoc_host: https://pkg.go.dev/
gdpr:
  impressum:
    enable: true
`

func parseSites(t *testing.T, s string) *config {
	t.Helper()
	var c config
	if err := yaml.Unmarshal([]byte(s), &c); err != nil {
		t.Fatal(err)
	}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	return &c
}

// TestSitesKeepWhatTheyServed checks that the sites configured before a
// site could choose its services still offer all of them, and that
// poly.red offers only its module.
func TestSitesKeepWhatTheyServed(t *testing.T) {
	c := parseSites(t, deployed)
	polyred := map[string]Import{"poly.red": {
		Repo: "https://github.com/changkun/polyred",
		Web:  "https://github.com/changkun/polyred",
	}}
	for _, tt := range []struct {
		host                 string
		want                 Site
		wantImports          map[string]Import
		wantImpressumVisible bool
	}{
		{"changkun.de", Site{Host: "changkun.de", RepoPath: "https://github.com/changkun", Links: true, Repos: true}, nil, true},
		{"golang.design", Site{Host: "golang.design", RepoPath: "https://github.com/golang-design", Links: true, Repos: true}, nil, false},
		{"poly.red", Site{Host: "poly.red", RepoPath: "https://github.com/changkun"}, polyred, false},
		// An unknown Host header is the primary site, without poly.red's
		// imports.
		{"evil.example", Site{Host: "changkun.de", RepoPath: "https://github.com/changkun", Links: true, Repos: true}, nil, true},
	} {
		got := c.SiteFor(tt.host)
		if got.Host != tt.want.Host || got.RepoPath != tt.want.RepoPath ||
			got.Links != tt.want.Links || got.Repos != tt.want.Repos {
			t.Errorf("SiteFor(%q) = %+v, want %+v", tt.host, got, tt.want)
		}
		if !maps.Equal(got.Imports, tt.wantImports) {
			t.Errorf("SiteFor(%q) imports = %v, want %v", tt.host, got.Imports, tt.wantImports)
		}
		if got.ShowImpressum != tt.wantImpressumVisible {
			t.Errorf("SiteFor(%q) shows the impressum: %v", tt.host, got.ShowImpressum)
		}
	}

	// Without x.enable no site has repositories, but imports still work.
	c.X.Enable = false
	if c.SiteFor("golang.design").Repos {
		t.Error("golang.design serves /x/ with x.enable false")
	}
	if _, _, _, ok := c.SiteFor("poly.red").ImportFor("/"); !ok {
		t.Error("poly.red lost its import with x.enable false")
	}
}

func TestImportFor(t *testing.T) {
	site := Site{Host: "poly.red", Imports: map[string]Import{
		"poly.red":         {Repo: "https://github.com/changkun/polyred"},
		"poly.red/sub/mod": {Repo: "https://github.com/changkun/submod"},
	}}
	for _, tt := range []struct {
		path, importPath, repo, suffix string
	}{
		{"/", "poly.red", "https://github.com/changkun/polyred", ""},
		{"", "poly.red", "https://github.com/changkun/polyred", ""},
		{"/geometry", "poly.red", "https://github.com/changkun/polyred", "/geometry"},
		{"/geometry/primitive/", "poly.red", "https://github.com/changkun/polyred", "/geometry/primitive"},
		{"/s/foo", "poly.red", "https://github.com/changkun/polyred", "/s/foo"},
		// The longest import path wins.
		{"/sub/mod", "poly.red/sub/mod", "https://github.com/changkun/submod", ""},
		{"/sub/mod/pkg", "poly.red/sub/mod", "https://github.com/changkun/submod", "/pkg"},
		{"/sub/module", "poly.red", "https://github.com/changkun/polyred", "/sub/module"},
	} {
		p, imp, suffix, ok := site.ImportFor(tt.path)
		if !ok || p != tt.importPath || imp.Repo != tt.repo || suffix != tt.suffix {
			t.Errorf("ImportFor(%q) = %q, %q, %q, %v, want %q, %q, %q",
				tt.path, p, imp.Repo, suffix, ok, tt.importPath, tt.repo, tt.suffix)
		}
	}

	// A path is in an import path only below a slash.
	other := Site{Host: "poly.red", Imports: map[string]Import{"poly.red/m": {Repo: "https://x.example/m"}}}
	for _, path := range []string{"/", "/mod", "/x"} {
		if p, _, _, ok := other.ImportFor(path); ok {
			t.Errorf("ImportFor(%q) = %q, want none", path, p)
		}
	}
	if _, _, _, ok := (Site{Host: "changkun.de"}).ImportFor("/x/redir"); ok {
		t.Error("a site without imports found one")
	}
}

func TestValidateRejectsImports(t *testing.T) {
	for _, tt := range []struct{ name, imports, want string }{
		{"another site's path", "golang.design/x: {repo: https://github.com/changkun/x}", "not an import path of poly.red"},
		{"a longer hostname", "poly.redx: {repo: https://github.com/changkun/x}", "not an import path of poly.red"},
		{"a trailing slash", "poly.red/: {repo: https://github.com/changkun/x}", "ends with a slash"},
		{"no repository", "poly.red: {web: https://github.com/changkun/x}", "is not a URL"},
		{"a repository without a scheme", "poly.red: {repo: github.com/changkun/polyred}", "is not a URL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var c config
			conf := "hosts:\n  poly.red:\n    imports:\n      " + tt.imports + "\n"
			if err := yaml.Unmarshal([]byte(conf), &c); err != nil {
				t.Fatal(err)
			}
			err := c.validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("validate() = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}
