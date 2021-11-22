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
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/short"
	"changkun.de/x/redir/internal/utils"
)

// shortHandler redirects the current request to a known link if the alias is
// found in the redir store.
func (s *server) shortHandler(kind models.AliasKind) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// for development.
		if config.Conf.CORS {
			log.Println("CORS is activated.")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}

		switch r.Method {
		case http.MethodOptions:
			// nothing, really.
		case http.MethodPost:
			s.shortHandlerPost(kind, w, r)
		case http.MethodGet:
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Cache-Control", "max-age=0")
			s.shortHandlerGet(kind, w, r)
		default:
			err := fmt.Errorf("%s is not supported", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
		}
	})
}

type shortInput struct {
	Op    short.Op    `json:"op"`
	Alias string      `json:"alias"`
	Data  interface{} `json:"data"`
}

type shortOutput struct {
	Message string `json:"message"`
}

// shortHandlerPost handles all kinds of operations.
// This is not a RESTful style, because we don't have that much router space
// to use. We are currently limited the single index router, which is the /s.
func (s *server) shortHandlerPost(kind models.AliasKind, w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			b, _ := json.Marshal(shortOutput{
				Message: err.Error(),
			})
			w.Write(b)
			w.WriteHeader(http.StatusBadRequest)
		}
	}()

	// All post request must be authenticated.
	user, err := s.handleAuth(w, r)
	if err != nil {
		return
	}

	w.Header().Add("Content-Type", "application/json")

	// Decode request body and determine what is the operator
	d := json.NewDecoder(r.Body)
	var red shortInput
	err = d.Decode(&red)
	if err != nil {
		return
	}

	// Validating the operator and decode the redir data
	if !short.Op(red.Op).Valid() {
		err = errors.New("unsupported operator")
		return
	}

	b, err := json.Marshal(red.Data)
	if err != nil {
		return
	}

	var redir models.Redir
	err = json.Unmarshal(b, &redir)
	if err != nil {
		return
	}
	redir.UpdatedBy = user

	// Edit redirect data.
	err = short.Edit(r.Context(), s.db, short.Op(red.Op), red.Alias, &redir)
	if err == nil {
		// Flush the cache so that the changes can be effected immediately.
		s.cache.Flush()
	}
}

// shortHandlerGet is the core of redir service. It redirects a given
// alias to the actual destination.
func (s *server) shortHandlerGet(kind models.AliasKind, w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil && !errors.Is(err, errUnauthorized) {
			// Just redirect the user we could not find the record rather than
			// throw 50x. The server logs should be able to identify the issue.
			log.Printf("request err: %v\n", err)
			http.Redirect(w, r, "/404.html", http.StatusTemporaryRedirect)
		}
	}()

	ctx := r.Context()

	// statistic page
	var prefix string
	switch kind {
	case models.KindShort:
		prefix = config.Conf.S.Prefix
	case models.KindRandom:
		prefix = config.Conf.R.Prefix
	}

	// URLs with /s/.* is reserved for internal usage.
	if strings.HasPrefix(r.URL.Path, prefix+".") {
		err = s.serveStatic(ctx, w, r, prefix)
		return
	}

	// Identify the alias of the short link.
	alias := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), "/")

	// If alias is empty, then process index page request.
	if alias == "" {
		err = s.sIndex(ctx, w, r, kind)
		return
	}

	// Only allow valid aliases.
	if !short.Validity.MatchString(alias) {
		err = short.ErrInvalidAlias
		return
	}

	// Process visitor information, wait maximum 5 seconds.
	if config.Conf.Stats.Enable {
		recordCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		s.recognizeVisitor(recordCtx, w, r, alias, kind)
	}

	// Figure out redirect location
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

	// Send a wait page if time does not permitting
	if time.Now().UTC().Sub(red.ValidFrom.UTC()) < 0 {
		err = waitTmpl.Execute(w, &pageInfo{
			ValidFrom:     red.ValidFrom.UTC().Format("2006-01-02T15:04:05"),
			ShowImpressum: config.Conf.GDPR.Impressum.Enable,
			ShowPrivacy:   config.Conf.GDPR.Privacy.Enable,
			ShowContact:   config.Conf.GDPR.Contact.Enable,
		})
		return
	}

	// Send a warn page if the redirected link is an external link
	//
	// If the link configuring person thinks the redirected link is trustable,
	// do the redirects always.
	if !red.Trust {
		// Figure out if the user allow redirects always
		allowRedir := false
		cookie, _ := r.Cookie(redirAllowCookie)
		if cookie != nil {
			n, _ := strconv.Atoi(cookie.Value)
			if n == 1 {
				allowRedir = true
			}
		}

		// If a redirect is accidentally configured as non-trustable,
		// but still an internal website, then we don't show the warn page.
		if !allowRedir && !strings.Contains(red.URL, r.Host) {
			err = warnTmpl.Execute(w, &pageInfo{
				OwnerName:     config.Conf.GDPR.Owner.Name,
				OwnerDomain:   config.Conf.GDPR.Owner.Domain,
				URL:           red.URL,
				ShowImpressum: config.Conf.GDPR.Impressum.Enable,
				ShowPrivacy:   config.Conf.GDPR.Privacy.Enable,
				ShowContact:   config.Conf.GDPR.Contact.Enable,
			})
			return
		}
	}

	// Finally, let's redirect!
	http.Redirect(w, r, red.URL, http.StatusTemporaryRedirect)
}

type pageInfo struct {
	OwnerName     string
	OwnerDomain   string
	URL           string
	ValidFrom     string
	Body          template.HTML
	Email         string
	ShowImpressum bool
	ShowPrivacy   bool
	ShowContact   bool
}

func (s *server) serveStatic(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	prefix string,
) error {
	var (
		t *template.Template
		d *pageInfo
	)
	switch {
	case strings.HasPrefix(r.URL.Path, prefix+".static"):
		// Serve static files under ./.static/*. This should not conflict
		// with all existing aliases, meaning that alias should not start
		// with a dot.
		ext := filepath.Ext(r.URL.Path)
		switch ext {
		case ".css":
			w.Header().Add("Content-Type", "text/css")
		case ".js":
			w.Header().Add("Content-Type", "text/javascript")
		}

		f, err := statics.Open(strings.TrimPrefix(r.URL.Path, prefix+".static/"))
		if err != nil {
			return err
		}
		b, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	case strings.HasPrefix(r.URL.Path, prefix+".impressum"):
		if config.Conf.GDPR.Impressum.Enable {
			t = impressumTmpl
		}
		d = &pageInfo{
			Body:          template.HTML(config.Conf.GDPR.Impressum.Content),
			ShowImpressum: config.Conf.GDPR.Impressum.Enable,
			ShowPrivacy:   config.Conf.GDPR.Privacy.Enable,
			ShowContact:   config.Conf.GDPR.Contact.Enable,
		}
	case strings.HasPrefix(r.URL.Path, prefix+".privacy"):
		if config.Conf.GDPR.Privacy.Enable {
			t = privacyTmpl
		}
		d = &pageInfo{
			Body:          template.HTML(config.Conf.GDPR.Privacy.Content),
			ShowImpressum: config.Conf.GDPR.Impressum.Enable,
			ShowPrivacy:   config.Conf.GDPR.Privacy.Enable,
			ShowContact:   config.Conf.GDPR.Contact.Enable,
		}
	case strings.HasPrefix(r.URL.Path, prefix+".contact"):
		if config.Conf.GDPR.Contact.Enable {
			t = contactTmpl
		}
		d = &pageInfo{
			Email:         config.Conf.GDPR.Contact.Email,
			ShowImpressum: config.Conf.GDPR.Impressum.Enable,
			ShowPrivacy:   config.Conf.GDPR.Privacy.Enable,
			ShowContact:   config.Conf.GDPR.Contact.Enable,
		}
	}
	if t != nil {
		return t.Execute(w, d)
	}
	return nil
}

const (
	redirVidCookie   = "redir_vid"
	redirAllowCookie = "redir_allow"
)

// recognizeVisitor implements a best effort visitor recording.
//
// If the redir's cookie is presented, then we use cookie id.
// If the cookie does not present any data, we read the IP address, and
// allocates a new visitor id for the visitor.
//
// We don't care if any error happens inside.
func (s *server) recognizeVisitor(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	alias string,
	kind models.AliasKind,
) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var cookieVid string
	c, err := r.Cookie(redirVidCookie)
	if err != nil {
		cookieVid = ""
	} else {
		cookieVid = c.Value
	}

	// count visit and set cookie.
	vid, err := s.db.RecordVisit(ctx, &models.Visit{
		VisitorID: cookieVid,
		Alias:     alias,
		Kind:      kind,
		IP:        utils.ReadIP(r),
		UA:        r.UserAgent(),
		Referer:   r.Referer(),
		Time:      time.Now().UTC(),
	})
	if err != nil {
		log.Printf("cannot record alias %s's visit: %v", alias, err)
	} else {
		w.Header().Set("Set-Cookie", redirVidCookie+"="+vid)
	}
}

// checkdb checks whether the given alias is exsited in the redir database
func (s *server) checkdb(ctx context.Context, alias string) (*models.Redir, error) {
	a, err := s.db.FetchAlias(ctx, alias)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// checkvcs checks whether the given alias is an repository on VCS, if so,
// then creates a new alias and returns url of the vcs repository.
func (s *server) checkvcs(ctx context.Context, alias string) (*models.Redir, error) {

	// construct the try path and make the request to vcs
	repoPath := config.Conf.X.RepoPath
	repoPath = strings.TrimSuffix(repoPath, "/*")
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
	r := &models.Redir{
		Alias:     alias,
		Kind:      models.KindShort,
		URL:       tryPath,
		Private:   false,
		Trust:     false,
		ValidFrom: time.Now().UTC(),
	}
	err = s.db.StoreAlias(ctx, r)
	if err != nil {
		return s.checkdb(ctx, alias)
	}

	return r, nil
}

var (
	errInvalidStatParam = errors.New("invalid stat parameter")
	errMissingStatParam = errors.New("missing stat parameter")
)

// sIndex serves two types of index page, and serves statistics data.
//
// If there are no supplied value of a `mode` query parameter, the method
// returns a public visible index page that contains all publicly visible
// short urls.
//
// If the query parameter contains mode=admin, then it requires basic
// auth to access the admin dashboard where one can manage all short urls.
//
// If the query parameter contaisn mode=stat, then it returns application/json
// data, which contains data for data visualizations in the index page.
func (s *server) sIndex(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	kind models.AliasKind,
) error {
	e := struct {
		AdminView     bool
		StatsMode     bool
		DevMode       bool
		ShowImpressum bool
		ShowPrivacy   bool
		ShowContact   bool
	}{
		AdminView:     false,
		StatsMode:     config.Conf.Stats.Enable,
		DevMode:       config.Conf.Development,
		ShowImpressum: config.Conf.GDPR.Impressum.Enable,
		ShowPrivacy:   config.Conf.GDPR.Privacy.Enable,
		ShowContact:   config.Conf.GDPR.Contact.Enable,
	}

	mode := r.URL.Query().Get("mode")
	switch mode {
	case "stats": // stats data is public to everyone
		if config.Conf.Stats.Enable {
			err := s.statData(ctx, w, r, kind)
			if !errors.Is(err, errInvalidStatParam) {
				return err
			}
			log.Println(err)
		}
	case "index": // public visible index data
		return s.indexData(ctx, w, r, kind, true)
	case "index-pro": // data with statistics
		return s.indexData(ctx, w, r, kind, false)
	case "admin":
		_, err := s.handleAuth(w, r)
		if err != nil {
			return err
		}
		e.AdminView = true
	default:
		// Process visitor information for public index, wait maximum 5 seconds.
		recordCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		s.recognizeVisitor(recordCtx, w, r, "", kind)
	}

	// Serve the index page.
	w.Header().Add("Content-Type", "text/html")
	return dTmpl.Execute(w, e)
}

type indexOutput struct {
	Data  []models.RedirIndex `json:"data"`
	Page  int64               `json:"page"`
	Total int64               `json:"total"`
}

// index on all aliases, require admin access.
func (s *server) indexData(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	k models.AliasKind,
	public bool,
) error {
	if !public {
		_, err := s.handleAuth(w, r)
		if err != nil {
			return err
		}
	}
	w.Header().Add("Content-Type", "application/json")

	// get page size and number
	ps := r.URL.Query().Get("ps")
	pageSize, err := strconv.ParseUint(ps, 10, 0)
	if err != nil {
		pageSize = 5
	}
	pn := r.URL.Query().Get("pn")
	pageNum, err := strconv.ParseUint(pn, 10, 0)
	if err != nil || pageNum <= 0 {
		pageNum = 1
	}

	rs, total, err := s.db.FetchAliasAll(ctx, public, k, int64(pageSize), int64(pageNum))
	if err != nil {
		return err
	}

	b, err := json.Marshal(indexOutput{
		Data:  rs,
		Page:  int64(pageNum),
		Total: total,
	})
	if err != nil {
		return err
	}

	w.Write(b)
	return nil
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
		results, err = s.db.StatReferer(ctx, a, k, start, end)
		if err != nil {
			retErr = err
			return
		}
	case "ua":
		results, err = s.db.StatUA(ctx, a, k, start, end)
		if err != nil {
			retErr = err
			return
		}
	case "time":
		results, err = s.db.StatVisitHist(ctx, a, k, start, end)
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
