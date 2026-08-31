// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// A pure Go driver, so the binary that runs the migration still
	// builds with CGO disabled like the rest of the image.
	_ "modernc.org/sqlite"
)

// SQLite reads the store of the redir fork that served golang.design: a
// SQLite file written by a version predating the host column, the visitor
// cookie and the derived statistics columns.
//
// Its schema is not this project's:
//
//	collink(id, alias, kind, url, private, created_at, updated_at)
//	visit  (id, alias, kind, ip, ua, referer, created_at)
//
// kind separated short links from Go import paths when both were stored.
// Import paths are answered from a template now and never reach the
// store, so the column has no counterpart and is dropped. One link and 45
// visits carry kind=1; they are migrated as ordinary rows rather than
// discarded, since that is what they always were.
type SQLite struct {
	db *sql.DB
	// since narrows the visits to those recorded after it, for the second
	// pass that closes the window between an import and a route switch.
	// The zero value reads everything.
	since time.Time
}

// OpenSQLite opens path read-only. The file is the fallback if the
// migration is wrong, so it must come out unchanged; the connection is
// opened in a mode that cannot write to it.
func OpenSQLite(path string, since time.Time) (*SQLite, error) {
	db, err := sql.Open("sqlite",
		"file:"+path+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return nil, fmt.Errorf("cannot open %v: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot open %v: %w", path, err)
	}
	return &SQLite{db: db, since: since}, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

func (s *SQLite) Links(ctx context.Context) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT alias, url, private, created_at, updated_at FROM collink`)
	if err != nil {
		return nil, fmt.Errorf("cannot read collink: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var (
			l                    Link
			private              int
			createdAt, updatedAt sql.NullTime
		)
		if err := rows.Scan(&l.Alias, &l.URL, &private,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("cannot scan collink: %w", err)
		}
		l.Private = private != 0
		l.CreatedAt = createdAt.Time
		l.UpdatedAt = updatedAt.Time
		// The fork had no valid_from, trust, or authorship. A missing
		// value is carried as the zero value rather than invented: a
		// zero valid_from means "valid since always", which is what
		// these links have been.
		l.ValidFrom = createdAt.Time
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *SQLite) Visits(ctx context.Context, fn func(Visit) error) error {
	// The filter is applied in Go, not in SQL.
	//
	// created_at is TEXT holding a Go timestamp such as
	// "2026-08-31 19:15:35.022133382+00:00", and binding a time.Time
	// makes the driver render it in its own layout, so `created_at > ?`
	// is a string comparison between two different formats. Against real
	// data that silently included the boundary row, which would have
	// duplicated a visit on an incremental pass. Scanning and comparing
	// the parsed values is exact.
	rows, err := s.db.QueryContext(ctx, `
		SELECT alias, ip, ua, referer, created_at
		FROM visit
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("cannot read visit: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			v               Visit
			ip, ua, referer sql.NullString
			createdAt       sql.NullTime
		)
		if err := rows.Scan(&v.Alias, &ip, &ua, &referer, &createdAt); err != nil {
			return fmt.Errorf("cannot scan visit: %w", err)
		}
		// The columns are nullable in the fork's schema and NOT NULL
		// here, so a null becomes the empty string rather than a
		// fabricated value.
		v.IP = ip.String
		v.UA = ua.String
		v.Referer = referer.String
		v.Time = createdAt.Time
		if !s.after(v.Time) {
			continue
		}
		// The fork had no visitor cookie, so every migrated row has no
		// visitor id. VisitorID leaves it null.
		if err := fn(v); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *SQLite) Counts(ctx context.Context) (links, visits int64, err error) {
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM collink`).Scan(&links); err != nil {
		return 0, 0, fmt.Errorf("cannot count collink: %w", err)
	}
	// Counted with the same filter the copy uses, so the verification
	// compares like with like on an incremental pass. With no filter this
	// is a plain count; with one it has to walk the rows, since the
	// comparison cannot be done in SQL.
	if s.since.IsZero() {
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM visit`).Scan(&visits); err != nil {
			return 0, 0, fmt.Errorf("cannot count visit: %w", err)
		}
		return links, visits, nil
	}

	rows, err := s.db.QueryContext(ctx, `SELECT created_at FROM visit`)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot count visit: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t sql.NullTime
		if err := rows.Scan(&t); err != nil {
			return 0, 0, fmt.Errorf("cannot count visit: %w", err)
		}
		if s.after(t.Time) {
			visits++
		}
	}
	return links, visits, rows.Err()
}

// after reports whether t is strictly later than the cutoff. An unset
// cutoff takes everything.
func (s *SQLite) after(t time.Time) bool {
	return s.since.IsZero() || t.After(s.since)
}

var _ Source = (*SQLite)(nil)
