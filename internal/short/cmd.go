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
	"time"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"gopkg.in/yaml.v3"
)

var (
	Validity        = regexp.MustCompile(`^[\w\-][\w\-. \/]+$`)
	ErrInvalidAlias = errors.New("invalid alias pattern")
)

// Cmd processes the given alias and link with a specified op.
func Cmd(ctx context.Context, operate Op, r *models.Redir) (err error) {
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

	err = Edit(ctx, s, operate, r.Alias, r)
	return
}

// Edit edits the datastore for a given alias in a given operation.
// if the operation is create, then the alias is not necessary.
// if the operation is update/fetch/delete, then the alias is used to
// match the existing aliases, meaning that alias can be changed.
func Edit(ctx context.Context, s *db.Store, operate Op, a string, r *models.Redir) (err error) {
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
		rr, err = s.FetchAlias(ctx, a)
		if err != nil {
			err = nil
		} else {
			// use old values if not presents
			if r.URL == "" {
				r.URL = rr.URL
			}
			tt := time.Time{}
			if r.ValidFrom == tt {
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
		err = s.DeleteAlias(ctx, a)
		if err != nil {
			return
		}
		log.Printf("alias %v has been deleted.\n", a)
	case OpFetch:
		var r *models.Redir
		r, err = s.FetchAlias(ctx, a)
		if err != nil {
			return
		}
		b, _ := yaml.Marshal(r)
		log.Printf("\n%v\n", string(b))
	}
	return
}
