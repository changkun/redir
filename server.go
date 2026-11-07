// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"cmp"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"changkun.de/x/redir/internal/cache"
	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/utils"
)

type server struct {
	db     db.Store
	cache  *cache.LRU
	latere *latereAuth // nil unless auth.enable is latere and it is configured
}

var (
	//go:embed templates/base.html
	basetmpl string
	//go:embed templates/x.html
	xtmpl string
	//go:embed templates/wait.html
	waittmpl string
	//go:embed templates/warn.html
	warntmpl string
	//go:embed templates/impressum.html
	impressumtmpl string
	//go:embed templates/privacy.html
	privacytmpl string
	//go:embed templates/contact.html
	contacttmpl string
	//go:embed dashboard/build/index.html
	dtmpl string
	//go:embed dashboard/build/static/*
	sasse embed.FS
)

var (
	xTmpl         *template.Template
	waitTmpl      *template.Template
	warnTmpl      *template.Template
	impressumTmpl *template.Template
	privacyTmpl   *template.Template
	contactTmpl   *template.Template
	dTmpl         *template.Template
	statics       fs.FS
)

func init() {
	// We are not allow to use any additional routers.
	// Replace all /static files to ./.static folder.
	dtmpl = strings.Replace(dtmpl, "/static", "./.static", -1)

	// The templates are embedded constants, so parsing them depends on
	// nothing at runtime. Doing it here rather than in newServer means a
	// handler can be exercised without opening a database.
	// x.html is a go-import document rather than a page: `go get` reads
	// its meta tags and never renders it, so it carries no layout.
	xTmpl = template.Must(template.New("x").Parse(xtmpl))

	// The rest share one layout, so the design lives in a single file
	// instead of five copies of a stylesheet that had drifted apart.
	page := func(name, body string) *template.Template {
		t := template.Must(template.New(name).Parse(basetmpl))
		return template.Must(t.Parse(body))
	}
	waitTmpl = page("wait", waittmpl)
	warnTmpl = page("warn", warntmpl)
	impressumTmpl = page("impressum", impressumtmpl)
	privacyTmpl = page("privacy", privacytmpl)
	contactTmpl = page("contact", contacttmpl)

	dTmpl = template.Must(template.New("dashboard").Parse(dtmpl))
}

func newServer(ctx context.Context) *server {
	var err error
	statics, err = fs.Sub(sasse, "dashboard/build/static")
	if err != nil {
		log.Fatalf("cannot access sub file system: %v", err)
	}

	// Opening the store applies pending migrations, which is more than a
	// connect, so this allows more than the second a connect would need.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	store, err := db.NewStore(ctx, config.Conf.Store)
	if err != nil {
		log.Fatalf("cannot establish connection to %s, details: \n%v",
			db.Redact(config.Conf.Store), err)
	}

	var latere *latereAuth
	if config.Conf.Auth.Enable == config.Latere {
		latere = newLatereAuth(config.Conf.S.Prefix)
	}
	return &server{db: store, cache: cache.NewLRU(true), latere: latere}
}

func (s *server) close() {
	log.Println(s.db.Close())
}

func (s *server) registerHandler() {
	http.Handle("/", s.handler())
}

// handler routes a request by the site it arrived on. A site's own import
// paths come first, so every path below one, even /s/ on poly.red, answers
// for the module there. Then the short links and the repositories under
// /x/, each only on a site that offers it: a site without short links must
// not open a namespace of aliases, nor have checkvcs mint links in it.
func (s *server) handler() http.Handler {
	services := http.NewServeMux()
	log.Println("router /s is enabled.")
	services.Handle(config.Conf.S.Prefix, s.sHandler())
	if config.Conf.X.Enable {
		log.Println("router /x is enabled.")
		services.Handle(config.Conf.X.Prefix, s.xHandler())
	}
	imports := s.importHandler()

	return utils.Logging()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site := config.Conf.SiteFor(r.Host)
		if _, _, _, ok := site.ImportFor(r.URL.Path); ok {
			imports(w, r)
			return
		}
		// The prefix without its slash too, which the mux redirects to it.
		// An unset prefix, as x.prefix may be with x.enable false, is
		// under nothing rather than under everything.
		under := func(prefix string) bool {
			return prefix != "" && strings.HasPrefix(r.URL.Path+"/", prefix)
		}
		if under(config.Conf.S.Prefix) && !site.Links || under(config.Conf.X.Prefix) && !site.Repos {
			http.NotFound(w, r)
			return
		}
		services.ServeHTTP(w, r)
	}))
}

// goImport is the go-import document: the meta tag `go get` reads, and a
// refresh for a browser that lands on it.
type goImport struct {
	ImportRoot string // the import path at the root of the repository
	VCS        string
	VCSRoot    string // the repository
	Doc        string // where a browser is sent
}

func serveGoImport(w http.ResponseWriter, d goImport) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := xTmpl.Execute(w, d); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

// redirectClone sends `git clone` of an import path to the repository,
// and reports whether the request was one.
func redirectClone(w http.ResponseWriter, req *http.Request, repoRoot, suffix string) bool {
	if suffix != "/info/refs" || !strings.HasPrefix(req.URL.Query().Get("service"), "git-") {
		return false
	}
	http.Redirect(w, req, fmt.Sprintf("%s/info/refs?%s", repoRoot, req.URL.RawQuery), http.StatusFound)
	return true
}

// godoc is the documentation of the package at importPath.
func godoc(importPath string) string {
	return cmp.Or(config.Conf.X.GoDocHost, "https://pkg.go.dev/") + importPath
}

// xHandler redirect returns an HTTP handler that redirects requests for
// the tree rooted at importPath to pkg.go.dev pages for those import paths.
// The redirections include headers directing `go get.` to satisfy the
// imports by checking out code from repoPath using the configured VCS.
func (s *server) xHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// The import path comes from the request, and the repository
		// root from the site it belongs to: golang.design/x/foo must
		// resolve to golang-design/foo, not changkun/foo.
		site := config.Conf.SiteFor(req.Host)
		importPath := strings.TrimSuffix(req.Host+config.Conf.X.Prefix, "/")
		path := strings.TrimSuffix(req.Host+req.URL.Path, "/")
		var importRoot, repoRoot, suffix string
		if path == importPath {
			http.Redirect(w, req, godoc(importPath), http.StatusFound)
			return
		}
		elem := path[len(importPath)+1:]
		if i := strings.Index(elem, "/"); i >= 0 {
			elem, suffix = elem[:i], elem[i:]
		}
		importRoot = importPath + "/" + elem
		repoRoot = site.RepoPath + "/" + elem

		// Handling 'git clone https://changkun.de/x/repo'.
		if elem != "" && redirectClone(w, req, repoRoot, suffix) {
			return
		}
		serveGoImport(w, goImport{
			ImportRoot: importRoot,
			VCS:        config.Conf.X.VCS,
			VCSRoot:    repoRoot,
			Doc:        godoc(importRoot + suffix),
		})
	}
}

// importHandler answers for the import paths a site lists one by one, such
// as poly.red, each from its own repository. `go get` gets the go-import
// document; a browser goes to the configured page, or to the package's
// documentation.
func (s *server) importHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		importPath, imp, suffix, ok := config.Conf.SiteFor(req.Host).ImportFor(req.URL.Path)
		if !ok {
			http.NotFound(w, req)
			return
		}
		if redirectClone(w, req, imp.Repo, suffix) {
			return
		}
		doc := cmp.Or(imp.Web, godoc(importPath+suffix))
		if req.URL.Query().Get("go-get") != "1" {
			http.Redirect(w, req, doc, http.StatusFound)
			return
		}
		serveGoImport(w, goImport{
			ImportRoot: importPath,
			VCS:        cmp.Or(config.Conf.X.VCS, "git"),
			VCSRoot:    imp.Repo,
			Doc:        doc,
		})
	}
}
