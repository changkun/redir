// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
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
	db    *db.Store
	cache *cache.LRU
}

var (
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
}

func newServer(ctx context.Context) *server {
	xTmpl = template.Must(template.New("xTmpl").Parse(xtmpl))
	waitTmpl = template.Must(template.New("waitTmpl").Parse(waittmpl))
	warnTmpl = template.Must(template.New("warnTmpl").Parse(warntmpl))
	impressumTmpl = template.Must(template.New("impressumTmpl").Parse(impressumtmpl))
	privacyTmpl = template.Must(template.New("privacyTmpl").Parse(privacytmpl))
	contactTmpl = template.Must(template.New("contactTmpl").Parse(contacttmpl))
	dTmpl = template.Must(template.New("sTmpl").Parse(dtmpl))

	var err error
	statics, err = fs.Sub(sasse, "dashboard/build/static")
	if err != nil {
		log.Fatalf("cannot access sub file system: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	db, err := db.NewStore(ctx, config.Conf.Store)
	if err != nil {
		log.Fatalf("cannot establish connection to %s, details: \n%v",
			config.Conf.Store, err)
	}
	log.Printf("connected to %s", config.Conf.Store)
	return &server{db: db, cache: cache.NewLRU(true)}
}

func (s *server) close() {
	log.Println(s.db.Close())
}

func (s *server) registerHandler() {
	l := utils.Logging()

	// semantic shortener (default)
	log.Println("router /s is enabled.")
	http.Handle(config.Conf.S.Prefix, l(s.sHandler()))

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
func (s *server) xHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
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

		// Handling 'git clone https://changkun.de/x/repo'.
		if suffix == "/info/refs" && strings.HasPrefix(req.URL.Query().Get("service"), "git-") && elem != "" {
			http.Redirect(w, req, fmt.Sprintf("%s/info/refs?%s", repoRoot, req.URL.RawQuery), http.StatusFound)
			return
		}

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
	}
}
