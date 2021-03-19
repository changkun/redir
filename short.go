// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/utils"
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
		Short map[string]struct {
			URL       string `yaml:"url"`
			Private   bool   `yaml:"private"`
			ValidFrom string `yaml:"valid_from"`
		} `yaml:"short"`
		Random []struct {
			URL       string `yaml:"url"`
			Private   bool   `yaml:"private"`
			ValidFrom string `yaml:"valid_from"`
		} `yaml:"random"`
	}
	err = yaml.Unmarshal(b, &d)
	if err != nil {
		log.Fatalf("cannot unmarshal the imported file: %v\n", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	for alias, info := range d.Short {
		var t time.Time
		if info.ValidFrom != "" {
			t, err = time.Parse(time.RFC3339, info.ValidFrom)
			if err != nil {
				log.Fatalf("incorrect time format, expect RFC3339, but: %v, err: %v", info.ValidFrom, err)
			}
		} else {
			t = time.Now().UTC()
		}

		r := &models.Redirect{
			Alias:     alias,
			URL:       info.URL,
			Kind:      models.KindShort,
			Private:   info.Private,
			ValidFrom: t,
		}

		err = shortCmd(ctx, opUpdate, r)
		if err != nil {
			err = shortCmd(ctx, opCreate, r)
			if err != nil {
				log.Printf("cannot import alias %v: %v\n", alias, err)
			}
		}
	}
	for _, info := range d.Random {

		t, err := time.Parse(time.RFC3339, info.ValidFrom)
		if err != nil {
			log.Fatalf("incorrect time format, expect RFC3339, but: %v, err: %v", info.ValidFrom, err)
		}
		// This might conflict with existing ones, it should be fine
		// at the moment, the user of redir can always the command twice.
		if conf.R.Length <= 0 {
			conf.R.Length = 6
		}
		alias := utils.Randstr(conf.R.Length)

		r := &models.Redirect{
			Alias:     alias,
			URL:       info.URL,
			Kind:      models.KindRandom,
			Private:   info.Private,
			ValidFrom: t,
		}
		err = shortCmd(ctx, opUpdate, r)
		if err != nil {
			for i := 0; i < 10; i++ { // try 10x maximum
				err = shortCmd(ctx, opCreate, r)
				if err != nil {
					log.Printf("cannot create alias %v: %v\n", alias, err)
					continue
				}
				break
			}
		}
	}
}

// shortCmd processes the given alias and link with a specified op.
func shortCmd(ctx context.Context, operate op, r *models.Redirect) (err error) {
	s, err := db.NewStore(conf.Store)
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
		err = s.StoreAlias(ctx, r)
		if err != nil {
			return
		}
		log.Printf("alias %v has been created:\n", r.Alias)

		var prefix string
		switch r.Kind {
		case models.KindShort:
			prefix = conf.S.Prefix
		case models.KindRandom:
			prefix = conf.R.Prefix
		}
		log.Printf("%s%s%s\n", conf.Host, prefix, r.Alias)
	case opUpdate:
		var rr *models.Redirect

		// fetch the old values if possible, we don't care
		// if here returns an error.
		rr, err = s.FetchAlias(ctx, r.Alias)
		if err == nil {
			// use old values if not presents
			if r.URL == "" {
				r.URL = rr.URL
			}
			tt := time.Time{}
			if r.ValidFrom == tt {
				r.ValidFrom = rr.ValidFrom
			}
			if rr.Kind != r.Kind {
				r.Kind = rr.Kind
			}
		}
		err = nil

		// do update
		err = s.UpdateAlias(ctx, r)
		if err != nil {
			return
		}
		log.Printf("alias %v has been updated.\n", r.Alias)
	case opDelete:
		err = s.DeleteAlias(ctx, r.Alias)
		if err != nil {
			return
		}
		log.Printf("alias %v has been deleted.\n", r.Alias)
	case opFetch:
		var r *models.Redirect
		r, err = s.FetchAlias(ctx, r.Alias)
		if err != nil {
			return
		}
		log.Println(r.URL)
	}
	return
}

// shortHandler redirects the current request to a known link if the alias is
// found in the redir store.
func (s *server) shortHandler(kind models.AliasKind) http.Handler {
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
		case models.KindShort:
			prefix = conf.S.Prefix
		case models.KindRandom:
			prefix = conf.R.Prefix
		}

		alias := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), "/")
		if alias == "" {
			err = s.stats(ctx, kind, w, r)
			return
		}

		// figure out redirect location
		red, ok := s.cache.Get(alias)
		if !ok {
			red, err = s.checkdb(ctx, alias)
			if err != nil {
				red, err = s.checkvcs(ctx, alias)
				if err != nil {
					return
				}
			}
			s.cache.Put(alias, red)
		}

		// redirect the user immediate, but run pv/uv count in background
		if time.Now().UTC().Sub(red.ValidFrom.UTC()) > 0 {
			http.Redirect(w, r, red.URL, http.StatusTemporaryRedirect)
		} else {
			err = sTmpl.Execute(w, &struct {
				ValidFrom time.Time
			}{red.ValidFrom})
			if err != nil {
				return
			}
		}

		// count visit in another goroutine so it won't block the redirect.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := s.db.RecordVisit(ctx, &models.Visit{
				Alias:   alias,
				Kind:    kind,
				IP:      readIP(r),
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
func (s *server) checkdb(ctx context.Context, alias string) (*models.Redirect, error) {
	a, err := s.db.FetchAlias(ctx, alias)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// checkvcs checks whether the given alias is an repository on VCS, if so,
// then creates a new alias and returns url of the vcs repository.
func (s *server) checkvcs(ctx context.Context, alias string) (*models.Redirect, error) {

	// construct the try path and make the request to vcs
	repoPath := conf.X.RepoPath
	if strings.HasSuffix(repoPath, "/*") {
		repoPath = strings.TrimSuffix(repoPath, "/*")
	}
	tryPath := fmt.Sprintf("%s/%s", repoPath, alias)
	resp, err := http.Get(tryPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusMovedPermanently {
		return nil, fmt.Errorf("%s is not a repository", tryPath)
	}

	// figure out the new location
	if resp.StatusCode == http.StatusMovedPermanently {
		tryPath = resp.Header.Get("Location")
	}

	// store such a try path
	r := &models.Redirect{
		Alias:     alias,
		Kind:      models.KindShort,
		URL:       tryPath,
		Private:   false,
		ValidFrom: time.Now().UTC(),
	}
	err = s.db.StoreAlias(ctx, r)
	if err != nil {
		if errors.Is(err, db.ErrExistedAlias) {
			return s.checkdb(ctx, alias)
		}
		return nil, err
	}

	return r, nil
}

var (
	errInvalidStatParam = errors.New("invalid stat parameter")
	errMissingStatParam = errors.New("missing stat parameter")
)

type records struct {
	Title   string
	Host    string
	Prefix  string
	Records []models.VisitRecord
}

const (
	username = "x"
	password = "x"
)

func (s *server) handleAuth(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("WWW-Authenticate", `Basic realm=""`)

	u, p, ok := r.BasicAuth()
	if !ok {
		w.WriteHeader(401)
		return errors.New("failed to parsing basic auth")
	}
	if u != username {
		w.WriteHeader(401)
		return fmt.Errorf("username is invalid: %s", u)
	}
	if p != password {
		w.WriteHeader(401)
		return fmt.Errorf("password is invalid: %s", p)
	}
	w.WriteHeader(200)
	return nil
}

func (s *server) stats(ctx context.Context, kind models.AliasKind, w http.ResponseWriter, r *http.Request) error {
	mode := r.URL.Query().Get("mode")
	switch mode {
	case "admin":
		err := s.handleAuth(w, r)
		if err != nil {
			return err
		}
	case "stat":
		err := s.statData(ctx, w, r, kind)
		if !errors.Is(err, errInvalidStatParam) {
			return err
		}
		log.Println(err)
	}

	w.Header().Add("Content-Type", "text/html")

	var prefix string
	switch kind {
	case models.KindShort:
		prefix = conf.S.Prefix
	case models.KindRandom:
		prefix = conf.R.Prefix
	}

	ars := records{
		Title:   conf.Title,
		Host:    r.Host,
		Prefix:  prefix,
		Records: nil,
	}
	rs, err := s.db.CountVisit(ctx, kind)
	if err != nil {
		return err
	}
	ars.Records = rs
	statsTmpl = template.Must(template.ParseFiles("public/stats.html"))
	return statsTmpl.Execute(w, ars)
}

func (s *server) statData(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	k models.AliasKind,
) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("%w: %v", errInvalidStatParam, retErr)
		}
	}()

	params := r.URL.Query()
	a := params.Get("a")
	if a == "" {
		retErr = fmt.Errorf("%s: alias (a)", errMissingStatParam)
		return
	}

	stat := params.Get("stat")
	if stat == "" {
		retErr = fmt.Errorf("%s: stat mode (stat)", errMissingStatParam)
		return
	}

	start, end, err := parseDuration(params)
	if err != nil {
		retErr = fmt.Errorf("%s: %v", errInvalidStatParam, err)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	var results interface{}
	switch stat {
	case "referer":
		results, err = s.db.CountReferer(ctx, a, k, start, end)
		if err != nil {
			retErr = err
			return
		}
	case "ua":
		results, err = s.db.CountUA(ctx, a, k, start, end)
		if err != nil {
			retErr = err
			return
		}
	case "time":
		results, err = s.db.CountVisitHist(ctx, a, k, start, end)
		if err != nil {
			retErr = err
			return
		}
	default:
		retErr = fmt.Errorf("%s stat mode is not supported", stat)
		return
	}

	b, err := json.Marshal(results)
	if err != nil {
		retErr = err
		return
	}
	w.Write(b)
	return
}

func parseDuration(p url.Values) (start, end time.Time, err error) {
	t0 := p.Get("t0")
	if t0 != "" {
		start, err = time.Parse("2006-01-02", t0)
		if err != nil {
			return
		}
	} else {
		start = time.Now().UTC().Add(-time.Hour * 24 * 7) // last week
	}
	t1 := p.Get("t1")
	if t1 != "" {
		end, err = time.Parse("2006-01-02", t1)
		if err != nil {
			return
		}
	} else {
		end = time.Now().UTC()
	}
	return
}
