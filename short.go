// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// op is a short link operator
type op string

const (
	// opCreate represents a create operation for short link
	opCreate op = "create"
	// opDelete represents a delete operation for short link
	opDelete = "delete"
	// opUpdate represents a update operation for short link
	opUpdate = "update"
	// opFetch represents a fetch operation for short link
	opFetch = "fetch"
)

func (o op) valid() bool {
	switch o {
	case opCreate, opDelete, opUpdate, opFetch:
		return true
	default:
		return false
	}
}

func importFile(fname string) {
	b, err := os.ReadFile(fname)
	if err != nil {
		log.Fatalf("cannot read import file: %v\n", err)
	}

	var d struct {
		Short  map[string]string `yaml:"short"`
		Random []string          `yaml:"random"`
	}
	err = yaml.Unmarshal(b, &d)
	if err != nil {
		log.Fatalf("cannot unmarshal the imported file: %v\n", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	for alias, link := range d.Short {
		err = shortCmd(ctx, opUpdate, alias, link)
		if err != nil {
			err = shortCmd(ctx, opCreate, alias, link)
			if err != nil {
				log.Printf("cannot import alias %v: %v\n", alias, err)
			}
		}
	}
	for _, link := range d.Random {
		err = shortCmd(ctx, opUpdate, "", link)
		if err != nil {
			for i := 0; i < 10; i++ { // try 10x maximum
				err = shortCmd(ctx, opCreate, "", link)
				if err != nil {
					log.Printf("cannot create alias %v: %v\n", alias, err)
					continue
				}
				break
			}
		}
	}
	return
}

// shortCmd processes the given alias and link with a specified op.
func shortCmd(ctx context.Context, operate op, alias, link string) (err error) {
	s, err := newDB(conf.Store)
	if err != nil {
		err = fmt.Errorf("cannot create a new alias: %w", err)
		return
	}
	defer s.Close()

	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot %v alias to data store: %w", operate, err)
		}
	}()

	switch operate {
	case opCreate:
		kind := kindShort
		if alias == "" {
			// This might conflict with existing ones, it should be fine
			// at the moment, the user of redir can always the command twice.
			if conf.R.Length <= 0 {
				conf.R.Length = 6
			}
			alias = randstr(conf.R.Length)
			kind = kindRandom
		}
		err = s.StoreAlias(ctx, &redirect{
			Alias:   alias,
			Kind:    kind,
			URL:     link,
			Private: false,
		})
		if err != nil {
			return
		}
		log.Printf("alias %v has been created:\n", alias)

		var prefix string
		switch kind {
		case kindShort:
			prefix = conf.S.Prefix
		case kindRandom:
			prefix = conf.R.Prefix
		}
		fmt.Printf("%s%s%s\n", conf.Host, prefix, alias)
	case opUpdate:
		_, err = s.UpdateAlias(ctx, alias, link)
		if err != nil {
			return
		}
		log.Printf("alias %v has been updated.\n", alias)
	case opDelete:
		err = s.DeleteAlias(ctx, alias)
		if err != nil {
			return
		}
		log.Printf("alias %v has been deleted.\n", alias)
	case opFetch:
		var r *redirect
		r, err = s.FetchAlias(ctx, alias)
		if err != nil {
			return
		}
		log.Println(r.URL)
	}
	return
}

// shortHandler redirects the current request to a known link if the alias is
// found in the redir store.
func (s *server) shortHandler(kind aliasKind) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var err error
		defer func() {
			if err != nil {
				// Just tell the user we could not find the record rather than
				// throw 50x. The server should be able to identify the issue.
				log.Printf("stats err: %v\n", err)
				// Use 307 redirect to 404 page
				http.Redirect(w, r, "/404.html", http.StatusTemporaryRedirect)
			}
		}()

		// statistic page
		var prefix string
		switch kind {
		case kindShort:
			prefix = conf.S.Prefix
		case kindRandom:
			prefix = conf.R.Prefix
		}

		alias := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), "/")
		if alias == "" {
			err = s.stats(ctx, kind, w, r)
			return
		}

		// figure out redirect location
		url, ok := s.cache.Get(alias)
		if !ok {
			url, err = s.checkdb(ctx, alias)
			if err != nil {
				url, err = s.checkvcs(ctx, alias)
				if err != nil {
					return
				}
			}
			s.cache.Put(alias, url)
		}

		// redirect the user immediate, but run pv/uv count in background
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)

		// count visit in another goroutine so it won't block the redirect.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			ip := readIP(r)
			err := s.db.RecordVisit(ctx, &visit{
				Alias:   alias,
				Kind:    kind,
				IP:      ip,
				UA:      r.UserAgent(),
				Referer: r.Referer(),
				Time:    time.Now().UTC(),
			})
			if err != nil {
				log.Printf("cannot record %s visit: %v", alias, err)
			}
		}()
	})
}

// checkdb checks whether the given alias is exsited in the redir database
func (s *server) checkdb(ctx context.Context, alias string) (string, error) {
	a, err := s.db.FetchAlias(ctx, alias)
	if err != nil {
		return "", err
	}
	return a.URL, nil
}

// checkvcs checks whether the given alias is an repository on VCS, if so,
// then creates a new alias and returns url of the vcs repository.
func (s *server) checkvcs(ctx context.Context, alias string) (string, error) {

	// construct the try path and make the request to vcs
	repoPath := conf.X.RepoPath
	if strings.HasSuffix(repoPath, "/*") {
		repoPath = strings.TrimSuffix(repoPath, "/*")
	}
	tryPath := fmt.Sprintf("%s/%s", repoPath, alias)
	resp, err := http.Get(tryPath)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("%s is not a repository", tryPath)
	}

	// figure out the new location
	if resp.StatusCode == http.StatusMovedPermanently {
		tryPath = resp.Header.Get("Location")
	}

	// store such a try path
	err = s.db.StoreAlias(ctx, &redirect{
		Alias:   alias,
		Kind:    kindShort,
		URL:     tryPath,
		Private: false,
	})
	if err != nil {
		if errors.Is(err, errExistedAlias) {
			return s.checkdb(ctx, alias)
		}
		return "", err
	}

	return tryPath, nil
}

type records struct {
	Title           string
	Host            string
	Prefix          string
	Records         []record
	GoogleAnalytics string
}

func (s *server) stats(ctx context.Context, kind aliasKind, w http.ResponseWriter, r *http.Request) error {
	var prefix string
	switch kind {
	case kindShort:
		prefix = conf.S.Prefix
	case kindRandom:
		prefix = conf.R.Prefix
	}

	ars := records{
		Title:           conf.Title,
		Host:            r.Host,
		Prefix:          prefix,
		Records:         nil,
		GoogleAnalytics: conf.GoogleAnalytics,
	}
	rs, err := s.db.CountVisit(ctx, kind)
	if err != nil {
		return err
	}
	ars.Records = rs

	return statsTmpl.Execute(w, ars)
}
