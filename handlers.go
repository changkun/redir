// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"html/template"
	"log"
	"net/http"
	"strings"
)

type visit struct {
	ip    string
	alias string
}

type server struct {
	db      *store
	cache   *lru
	visitCh chan visit
}

var (
	xTmpl     *template.Template
	statsTmpl *template.Template
)

func newServer(ctx context.Context) *server {
	xTmpl = template.Must(template.ParseFiles("public/x.html"))
	statsTmpl = template.Must(template.ParseFiles("public/stats.html"))

	db, err := newStore(conf.Store)
	if err != nil {
		log.Fatalf("cannot establish connection to database, err: %v", err)
	}
	s := &server{
		db:      db,
		cache:   newLRU(true),
		visitCh: make(chan visit, 100),
	}
	go s.counting(ctx)
	return s
}

func (s *server) close() {
	log.Println(s.db.Close())
}

func (s *server) registerHandler() {
	// short redirector
	http.HandleFunc(conf.S.Prefix, s.shortHandler(kindShort))
	http.HandleFunc(conf.R.Prefix, s.shortHandler(kindRandom))
	// repo redirector
	http.Handle(conf.X.Prefix, s.xHandler(conf.X.VCS, conf.X.ImportPath, conf.X.RepoPath))
}

// xHandler redirect returns an HTTP handler that redirects requests for
// the tree rooted at importPath to pkg.go.dev pages for those import paths.
// The redirections include headers directing `go get.` to satisfy the
// imports by checking out code from repoPath using the given VCS.
// As a special case, if both importPath and repoPath end in /*, then
// the matching element in the importPath is substituted into the repoPath
// specified for `go get.`
func (s *server) xHandler(vcs, importPath, repoPath string) http.Handler {
	wildcard := false
	if strings.HasSuffix(importPath, "/*") && strings.HasSuffix(repoPath, "/*") {
		wildcard = true
		importPath = strings.TrimSuffix(importPath, "/*")
		repoPath = strings.TrimSuffix(repoPath, "/*")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimSuffix(req.Host+req.URL.Path, "/")
		var importRoot, repoRoot, suffix string
		if wildcard {
			if path == importPath {
				http.Redirect(w, req, conf.X.GoDocHost+importPath, http.StatusFound)
				return
			}
			if !strings.HasPrefix(path, importPath+"/") {
				http.NotFound(w, req)
				return
			}
			elem := path[len(importPath)+1:]
			if i := strings.Index(elem, "/"); i >= 0 {
				elem, suffix = elem[:i], elem[i:]
			}
			importRoot = importPath + "/" + elem
			repoRoot = repoPath + "/" + elem
		} else {
			if path != importPath && !strings.HasPrefix(path, importPath+"/") {
				http.NotFound(w, req)
				return
			}
			importRoot = importPath
			repoRoot = repoPath
			suffix = path[len(importPath):]
		}
		d := &struct {
			ImportRoot      string
			VCS             string
			VCSRoot         string
			Suffix          string
			GoogleAnalytics string
		}{
			ImportRoot:      importRoot,
			VCS:             vcs,
			VCSRoot:         repoRoot,
			Suffix:          suffix,
			GoogleAnalytics: conf.GoogleAnalytics,
		}
		var buf bytes.Buffer
		err := xTmpl.Execute(&buf, d)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(buf.Bytes())
	})
}
