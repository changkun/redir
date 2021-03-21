// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	_ "embed"
	"html/template"
	"log"
	"net/http"
	"strings"

	"changkun.de/x/redir/internal/cache"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/utils"
)

type server struct {
	db    *db.Store
	cache *cache.LRU
}

var (
	//go:embed public/x.html
	xtmpl string
	//go:embed public/wait.html
	stmpl string
)

var (
	xTmpl     *template.Template
	sTmpl     *template.Template
	statsTmpl *template.Template
)

func newServer(ctx context.Context) *server {
	xTmpl = template.Must(template.New("xtmpl").Parse(xtmpl))
	sTmpl = template.Must(template.New("stmpl").Parse(stmpl))

	db, err := db.NewStore(conf.Store)
	if err != nil {
		log.Fatalf("cannot establish connection to database: %v", err)
	}
	return &server{
		db:    db,
		cache: cache.NewLRU(true),
	}
}

func (s *server) close() {
	log.Println(s.db.Close())
}

func (s *server) registerHandler() {
	l := logging()

	// semantic shortener (default)
	log.Println("router /s is enabled.")
	http.Handle(conf.S.Prefix, l(s.shortHandler(models.KindShort)))

	// random shortener
	if conf.R.Enable {
		log.Println("router /r is enabled.")
		http.Handle(conf.R.Prefix, l(s.shortHandler(models.KindRandom)))
	}

	// repo redirector
	if conf.X.Enable {
		log.Println("router /x is enabled.")
		http.Handle(conf.X.Prefix, l(s.xHandler()))
	}
}

func logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				log.Println(utils.ReadIP(r), r.Method, r.URL.Path, r.URL.RawQuery)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// xHandler redirect returns an HTTP handler that redirects requests for
// the tree rooted at importPath to pkg.go.dev pages for those import paths.
// The redirections include headers directing `go get.` to satisfy the
// imports by checking out code from repoPath using the configured VCS.
func (s *server) xHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		importPath := strings.TrimSuffix(req.Host+conf.X.Prefix, "/")
		path := strings.TrimSuffix(req.Host+req.URL.Path, "/")
		var importRoot, repoRoot, suffix string
		if path == importPath {
			http.Redirect(w, req, conf.X.GoDocHost+importPath, http.StatusFound)
			return
		}
		elem := path[len(importPath)+1:]
		if i := strings.Index(elem, "/"); i >= 0 {
			elem, suffix = elem[:i], elem[i:]
		}
		importRoot = importPath + "/" + elem
		repoRoot = conf.X.RepoPath + "/" + elem

		d := &struct {
			ImportRoot string
			VCS        string
			VCSRoot    string
			Suffix     string
		}{
			ImportRoot: importRoot,
			VCS:        conf.X.VCS,
			VCSRoot:    repoRoot,
			Suffix:     suffix,
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		err := xTmpl.Execute(w, d)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	})
}
