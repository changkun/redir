// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"changkun.de/x/redir/internal/cache"
	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/utils"
)

type server struct {
	db    *db.Store
	cache *cache.LRU
}

var (
	//go:embed templates/x.html
	xtmpl string
	//go:embed templates/wait.html
	wtmpl string
	//go:embed dashboard/build/index.html
	dtmpl string
	//go:embed dashboard/build/static/*
	sasse embed.FS
)

var (
	xTmpl   *template.Template
	wTmpl   *template.Template
	dTmpl   *template.Template
	statics fs.FS
)

func init() {
	// We are not allow to use any additional routers.
	// Replace all /static files to ./.static folder.
	dtmpl = strings.Replace(dtmpl, "/static", "./.static", -1)
}

func newServer(ctx context.Context) *server {
	xTmpl = template.Must(template.New("xtmpl").Parse(xtmpl))
	wTmpl = template.Must(template.New("wtmpl").Parse(wtmpl))
	dTmpl = template.Must(template.New("stmpl").Parse(dtmpl))
	var err error
	statics, err = fs.Sub(sasse, "dashboard/build/static")
	if err != nil {
		log.Fatalf("cannot access sub file system: %v", err)
	}

	db, err := db.NewStore(context.Background(), config.Conf.Store)
	if err != nil {
		log.Fatalf("cannot establish connection to database: %v", err)
	}
	return &server{db: db, cache: cache.NewLRU(true)}
}

func (s *server) close() {
	log.Println(s.db.Close())
}

func (s *server) registerHandler() {
	l := utils.Logging()

	// semantic shortener (default)
	log.Println("router /s is enabled.")
	http.Handle(config.Conf.S.Prefix, l(s.shortHandler(models.KindShort)))

	// random shortener
	if config.Conf.R.Enable {
		log.Println("router /r is enabled.")
		http.Handle(config.Conf.R.Prefix, l(s.shortHandler(models.KindRandom)))
	}

	// repo redirector
	if config.Conf.X.Enable {
		log.Println("router /x is enabled.")
		http.Handle(config.Conf.X.Prefix, l(s.xHandler()))
	}
}

// xHandler redirect returns an HTTP handler that redirects requests for
// the tree rooted at importPath to pkg.go.dev pages for those import paths.
// The redirections include headers directing `go get.` to satisfy the
// imports by checking out code from repoPath using the configured VCS.
func (s *server) xHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		importPath := strings.TrimSuffix(req.Host+config.Conf.X.Prefix, "/")
		path := strings.TrimSuffix(req.Host+req.URL.Path, "/")
		var importRoot, repoRoot, suffix string
		if path == importPath {
			http.Redirect(w, req, config.Conf.X.GoDocHost+importPath, http.StatusFound)
			return
		}
		elem := path[len(importPath)+1:]
		if i := strings.Index(elem, "/"); i >= 0 {
			elem, suffix = elem[:i], elem[i:]
		}
		importRoot = importPath + "/" + elem
		repoRoot = config.Conf.X.RepoPath + "/" + elem

		d := &struct {
			ImportRoot string
			VCS        string
			VCSRoot    string
			Suffix     string
		}{
			ImportRoot: importRoot,
			VCS:        config.Conf.X.VCS,
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
