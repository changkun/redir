// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
)

type server struct {
	db    *database
	cache *lru
}

var (
	xTmpl     *template.Template
	statsTmpl *template.Template
)

func newServer(ctx context.Context) *server {
	xTmpl = template.Must(template.ParseFiles("public/x.html"))
	statsTmpl = template.Must(template.ParseFiles("public/stats.html"))

	db, err := newDB(conf.Store)
	if err != nil {
		log.Fatalf("cannot establish connection to database: %v", err)
	}
	return &server{
		db:    db,
		cache: newLRU(true),
	}
}

func (s *server) close() {
	log.Println(s.db.Close())
}

func (s *server) registerHandler() {
	l := logging()

	// short redirector
	http.Handle(conf.S.Prefix, l(s.shortHandler(kindShort)))
	http.Handle(conf.R.Prefix, l(s.shortHandler(kindRandom)))
	// repo redirector
	http.Handle(conf.X.Prefix, l(s.xHandler()))
}

func logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				log.Println(readIP(r), r.Method, r.URL.Path)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// readIP implements a best effort approach to return the real client IP,
// it parses X-Real-IP and X-Forwarded-For in order to work properly with
// reverse-proxies such us: nginx or haproxy. Use X-Forwarded-For before
// X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
//
// This implementation is derived from gin-gonic/gin.
func readIP(r *http.Request) string {
	clientIP := r.Header.Get("X-Forwarded-For")
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])
	if clientIP == "" {
		clientIP = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	}
	if clientIP != "" {
		return clientIP
	}
	if addr := r.Header.Get("X-Appengine-Remote-Addr"); addr != "" {
		return addr
	}
	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return "unknown" // use unknown to guarantee non empty string
	}
	return ip
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
			ImportRoot      string
			VCS             string
			VCSRoot         string
			Suffix          string
			GoogleAnalytics string
		}{
			ImportRoot:      importRoot,
			VCS:             conf.X.VCS,
			VCSRoot:         repoRoot,
			Suffix:          suffix,
			GoogleAnalytics: conf.GoogleAnalytics,
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		err := xTmpl.Execute(w, d)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	})
}
