// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package short

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"gopkg.in/yaml.v3"
)

var (
	Validity        = regexp.MustCompile(`^[\w\-][\w\-. \/]+$`)
	ErrInvalidAlias = errors.New("invalid alias pattern")
)

// Given names the fields a caller actually supplied.
//
// A boolean flag that was not passed is indistinguishable from one passed
// as false, so an update had no way to tell "make this public" from "I
// said nothing about visibility". It chose the first, which quietly made
// private links public and untrusted links trusted whenever someone
// changed a URL.
type Given struct {
	URL       bool
	Private   bool
	Trust     bool
	ValidFrom bool
}

// Cmd processes the given alias and link with a specified op.
func Cmd(ctx context.Context, operate Op, r *models.Redir, given Given) (err error) {
	s, err := db.NewStore(ctx, config.Conf.Store)
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

	err = Edit(ctx, s, operate, r.Alias, r, given)
	return
}

// Edit edits the datastore for a given alias in a given operation.
// if the operation is create, then the alias is not necessary.
// if the operation is update/fetch/delete, then the alias is used to
// match the existing aliases, meaning that alias can be changed.
func Edit(ctx context.Context, s db.Store, operate Op, a string, r *models.Redir, given Given) (err error) {
	// Links are keyed by (host, alias). An admin command carries no
	// request to read the host from, so it acts on the configured one.
	if r.Host == "" {
		r.Host = config.Conf.Hostname()
	}
	host := r.Host

	switch operate {
	case OpCreate:
		if !Validity.MatchString(r.Alias) {
			err = ErrInvalidAlias
			return
		}
		r.CreatedBy = r.UpdatedBy
		err = s.StoreAlias(ctx, r)
		if err != nil {
			return
		}
		log.Printf("alias %v has been created:\n", r.Alias)

		prefix := config.Conf.S.Prefix
		log.Printf("%s%s%s\n", config.Conf.Host, prefix, r.Alias)
	case OpUpdate:
		var rr *models.Redir

		// fetch the old values if possible, we don't care
		// if here returns an error.
		//
		// Note that this is not atomic, meaning that we might run
		// into concurrent inconsistent issue. But for small scale
		// use, it is fine for now.
		rr, err = s.FetchAlias(ctx, host, a)
		if err == nil {
			// Every field the caller did not supply keeps the value it
			// has. An update that mentions only the URL must not decide
			// anything about visibility or trust on the caller's behalf.
			if !given.URL {
				r.URL = rr.URL
			}
			if !given.Private {
				r.Private = rr.Private
			}
			if !given.Trust {
				r.Trust = rr.Trust
			}
			if !given.ValidFrom {
				r.ValidFrom = rr.ValidFrom
			}
			r.ID = rr.ID
		}

		if r.ID == "" {
			err = fmt.Errorf("cannot find alias %s for update", a)
			return
		}

		// do update
		err = s.UpdateAlias(ctx, r)
		if err != nil {
			return
		}
		log.Printf("alias %v has been updated.\n", a)
	case OpDelete:
		err = s.DeleteAlias(ctx, host, a)
		if err != nil {
			return
		}
		log.Printf("alias %v has been deleted.\n", a)
	case OpFetch:
		var r *models.Redir
		r, err = s.FetchAlias(ctx, host, a)
		if err != nil {
			return
		}
		b, _ := yaml.Marshal(r)
		log.Printf("\n%v\n", string(b))
	}
	return
}
