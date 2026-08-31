// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"changkun.de/x/redir/internal/models"
)

// Store is the persistence redir runs on.
//
// It has one implementation, over PostgreSQL. The interface stays because
// it is the seam the handler tests substitute, so recording a visit and
// setting its cookie can be tested without a database.
//
// A link is identified by the pair (host, alias): one process serves
// several hosts and the same alias means different things on each. Methods
// that take a whole model read the host from it; the others take it as an
// argument.
type Store interface {
	// Close releases the connection.
	Close() error

	// StoreAlias stores r if (r.Host, r.Alias) is free, and reports an
	// error if it is taken.
	StoreAlias(ctx context.Context, r *models.Redir) error
	// UpdateAlias updates the link identified by r.ID.
	UpdateAlias(ctx context.Context, r *models.Redir) error
	// DeleteAlias removes a link. Its visits are kept: they happened.
	DeleteAlias(ctx context.Context, host, alias string) error
	// FetchAlias returns one link.
	FetchAlias(ctx context.Context, host, alias string) (*models.Redir, error)
	// FetchAliasAll returns a page of links and the total count. The
	// public form omits the target URL and the visit counts.
	FetchAliasAll(ctx context.Context, host string, public bool, pageSize, pageNum int64) ([]models.RedirIndex, int64, error)

	// RecordVisit records one visit and returns the visitor id to set as
	// a cookie.
	RecordVisit(ctx context.Context, v *models.Visit) (string, error)

	// StatReferer counts visits per referer within [start, end).
	StatReferer(ctx context.Context, host, alias string, start, end time.Time) ([]models.RefStat, error)
	// StatUA counts visits per user agent within [start, end).
	StatUA(ctx context.Context, host, alias string, start, end time.Time) ([]models.UAStat, error)
	// StatVisitHist counts PV and UV per hour within [start, end).
	StatVisitHist(ctx context.Context, host, alias string, start, end time.Time) ([]models.TimeHist, error)
	// StatVisit counts PV and UV for the given aliases.
	StatVisit(ctx context.Context, host string, aliases []string) ([]models.VisitRecord, error)
}

// NewStore opens the store named by uri.
func NewStore(ctx context.Context, uri string) (Store, error) {
	scheme, _, ok := strings.Cut(uri, "://")
	if !ok {
		return nil, fmt.Errorf("store %q has no scheme, want postgres://", Redact(uri))
	}

	switch scheme {
	case "postgres", "postgresql":
		return newPostgresStore(ctx, uri)
	case "mongodb", "mongodb+srv":
		// Whoever sees this is part way through a rollback, so the
		// message says where the way back is rather than only what is
		// wrong. See specs/005-drop-mongodb.md.
		return nil, fmt.Errorf(
			"the MongoDB backend was removed after v0.7.0: check out "+
				"v0.7.0 and rebuild to use %q, or point the store at postgres://",
			scheme+"://")
	default:
		return nil, fmt.Errorf("unsupported store scheme %q, want postgres://", scheme)
	}
}

// Redact removes the password from a store URI before it reaches a log
// line or an error message, both of which are read by people who should
// not be shown the database credentials.
func Redact(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return "invalid store uri"
	}
	if u.User != nil {
		if _, ok := u.User.Password(); ok {
			u.User = url.UserPassword(u.User.Username(), "xxxxx")
		}
	}
	return u.String()
}
