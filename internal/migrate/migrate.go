// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Package migrate loads links and visits from an older store into
// PostgreSQL.
//
// It was written to carry redir's data off MongoDB, and the MongoDB
// reader is gone with the rest of that backend. What remains is the half
// that was never MongoDB-specific: batched loading, the rules about which
// values may be discarded and which must be carried unchanged, and the
// count check that decides whether a migration worked.
//
// A Source supplies the rows. See specs/004-unify-golang-design.md, which
// adds one over the SQLite database of the golang.design deployment.
package migrate

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"changkun.de/x/redir/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"uuid"
)

// BatchSize is how many visit rows are sent per CopyFrom. A visit table
// runs to hundreds of thousands of rows, so this trades a bounded amount
// of memory for far fewer round trips.
const BatchSize = 10000

// Link is one short link as the source holds it.
//
// The zero value of a timestamp is meaningful and is stored as it is
// found; see keep.
type Link struct {
	Alias     string
	URL       string
	Private   bool
	Trust     bool
	ValidFrom time.Time
	CreatedBy string
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Visit is one recorded visit as the source holds it.
//
// VisitorID is a string rather than a UUID because a source may hold
// anything there: redir stored the visitor cookie verbatim before it was
// validated, so the column collected scanner probes.
type Visit struct {
	VisitorID string
	Alias     string
	IP        string
	UA        string
	Referer   string
	Time      time.Time
}

// Source is a store to read from. Implementations only read: the source
// of a migration is the thing to fall back to if the migration is wrong,
// so it must come out of this unchanged.
type Source interface {
	// Links returns every link. The error stops the migration.
	Links(ctx context.Context) ([]Link, error)
	// Visits calls fn for each visit in turn, so that a table too large
	// to hold in memory can still be copied.
	Visits(ctx context.Context, fn func(Visit) error) error
	// Counts returns how many links and visits the source holds, used to
	// check the migration moved all of them.
	Counts(ctx context.Context) (links, visits int64, err error)
}

// Options controls one run.
type Options struct {
	// Host is recorded on every migrated row. Links are keyed by
	// (host, alias), and a source predating that has no host of its own.
	Host string
	// Truncate empties the visit table first. Without it a non-empty
	// target is refused, because running a migration twice doubles every
	// visit count and a doubled count looks plausible.
	Truncate bool
	// DryRun reads and counts, and writes nothing.
	DryRun bool
	// Only limits which tables are written: "all", "links" or "visits".
	Only string
}

// Valid reports whether o names a known set of tables.
func (o Options) Valid() error {
	switch o.Only {
	case "", "all", "links", "visits":
	default:
		return fmt.Errorf("only %q: want all, links or visits", o.Only)
	}
	if o.Host == "" {
		return fmt.Errorf("host is required: every row is keyed by it")
	}
	return nil
}

func (o Options) wants(table string) bool {
	return o.Only == "" || o.Only == "all" || o.Only == table
}

// Run copies src into the PostgreSQL database at uri.
func Run(ctx context.Context, src Source, uri string, opts Options) error {
	if err := opts.Valid(); err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		return fmt.Errorf("cannot connect to target: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("cannot connect to target: %w", err)
	}

	if err := prepareTarget(ctx, pool, opts); err != nil {
		return err
	}

	var links, visits int64
	if opts.wants("links") {
		if links, err = copyLinks(ctx, src, pool, opts); err != nil {
			return err
		}
	}
	if opts.wants("visits") {
		if visits, err = copyVisits(ctx, src, pool, opts); err != nil {
			return err
		}
	}

	log.Printf("copied %d links and %d visits for host %v",
		links, visits, opts.Host)
	return verify(ctx, src, pool, links, visits, opts)
}

// prepareTarget refuses to add to a visit table that already holds rows.
//
// Links are not checked here: copyLinks replaces them for one host inside
// a transaction, so they neither block a run nor need emptying first.
func prepareTarget(ctx context.Context, pool *pgxpool.Pool, opts Options) error {
	if !opts.wants("visits") {
		return nil
	}

	var visits int64
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM visits`).Scan(&visits); err != nil {
		return fmt.Errorf("cannot inspect target: %w", err)
	}
	if visits == 0 {
		return nil
	}
	if !opts.Truncate {
		return fmt.Errorf(
			"target already holds %d visits; re-run with truncate to replace them",
			visits)
	}
	if opts.DryRun {
		log.Printf("dry run: would truncate %d visits", visits)
		return nil
	}
	if _, err := pool.Exec(ctx,
		`TRUNCATE visits RESTART IDENTITY`); err != nil {
		return fmt.Errorf("cannot truncate visits: %w", err)
	}
	log.Printf("truncated %d visits", visits)
	return nil
}

func copyLinks(
	ctx context.Context,
	src Source,
	pool *pgxpool.Pool,
	opts Options,
) (int64, error) {
	links, err := src.Links(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot read links: %w", err)
	}

	rows := make([][]any, 0, len(links))
	for _, l := range links {
		rows = append(rows, []any{
			opts.Host, sanitize(l.Alias), sanitize(l.URL), l.Private, l.Trust,
			keep(l.ValidFrom), sanitize(l.CreatedBy), sanitize(l.UpdatedBy),
			keep(l.CreatedAt), keep(l.UpdatedAt),
		})
	}
	if opts.DryRun {
		return int64(len(rows)), nil
	}

	// Replace this host's links inside one transaction. Emptying the
	// table first and filling it afterwards would leave a window in which
	// every alias looks missing: redir creates links for unresolved
	// aliases, so a request landing in that window could insert a row
	// that then makes this copy fail on the unique constraint. Within a
	// transaction, readers see the old rows until the new ones commit.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot begin link copy: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx,
		`DELETE FROM links WHERE host = $1`, opts.Host); err != nil {
		return 0, fmt.Errorf("cannot clear links for %v: %w", opts.Host, err)
	}
	n, err := tx.CopyFrom(ctx,
		pgx.Identifier{"links"},
		[]string{"host", "alias", "url", "private", "trust", "valid_from",
			"created_by", "updated_by", "created_at", "updated_at"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("cannot write links: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("cannot commit link copy: %w", err)
	}
	return n, nil
}

func copyVisits(
	ctx context.Context,
	src Source,
	pool *pgxpool.Pool,
	opts Options,
) (int64, error) {
	var (
		total int64
		batch [][]any
	)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if opts.DryRun {
			total += int64(len(batch))
			batch = batch[:0]
			return nil
		}
		n, err := pool.CopyFrom(ctx,
			pgx.Identifier{"visits"},
			[]string{"host", "alias", "visitor_id", "ip", "ua", "referer",
				"referer_host", "browser", "os", "device", "is_bot", "time"},
			pgx.CopyFromRows(batch))
		if err != nil {
			return fmt.Errorf("cannot write visits: %w", err)
		}
		total += n
		batch = batch[:0]
		log.Printf("copied %d visits", total)
		return nil
	}

	err := src.Visits(ctx, func(v Visit) error {
		// Derive through the code the server uses, so a migrated row and
		// a row written by a live redirect are identical.
		m := &models.Visit{
			Host:    opts.Host,
			Alias:   sanitize(v.Alias),
			IP:      sanitize(v.IP),
			UA:      sanitize(v.UA),
			Referer: sanitize(v.Referer),
			Time:    keep(v.Time),
		}
		m.Derive()

		batch = append(batch, []any{
			m.Host, m.Alias, VisitorID(v.VisitorID), m.IP, m.UA, m.Referer,
			m.RefererHost, m.Browser, m.OS, m.Device, m.IsBot, m.Time,
		})
		if len(batch) >= BatchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("cannot read visits: %w", err)
	}
	if err := flush(); err != nil {
		return 0, err
	}
	return total, nil
}

// VisitorID returns the stored identifier as a UUID, or nil.
//
// A value that does not parse becomes NULL. redir stored the visitor
// cookie verbatim before it was validated, so 505 rows held scanner
// probes and 10,708 held nothing at all. An absent identifier is honest;
// an invented one is not.
func VisitorID(s string) *uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// sanitize removes what a PostgreSQL text column will not accept. Older
// stores permit NUL bytes and invalid encoding in a string; text does not.
func sanitize(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\x00", "")
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}

// keep returns the stored timestamp unchanged.
//
// It does not substitute the current time for a missing one. When redir
// moved off MongoDB, 124 links had no valid_from, 131 no created_at and
// 106 no updated_at, and the zero value is meaningful: valid_from gates
// the redirect, so "always valid" must not become "valid from the day of
// the migration", and updated_at orders the admin index, which stamping
// every undated link with one instant would scramble.
//
// A migration may drop a value it cannot parse. It may never replace one
// with a plausible substitute.
func keep(t time.Time) time.Time {
	return t.UTC()
}

// verify re-counts both sides. A copy that reports success but moved the
// wrong number of rows is the failure worth catching.
func verify(
	ctx context.Context,
	src Source,
	pool *pgxpool.Pool,
	links, visits int64,
	opts Options,
) error {
	srcLinks, srcVisits, err := src.Counts(ctx)
	if err != nil {
		return fmt.Errorf("cannot count source: %w", err)
	}

	if opts.wants("links") && srcLinks != links {
		return fmt.Errorf("copied %d links, source holds %d", links, srcLinks)
	}
	if opts.wants("visits") && srcVisits != visits {
		return fmt.Errorf("copied %d visits, source holds %d", visits, srcVisits)
	}
	if opts.DryRun {
		log.Printf("dry run: source holds %d links and %d visits",
			srcLinks, srcVisits)
		return nil
	}

	var dstLinks, dstVisits int64
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM links  WHERE host = $1),
		       (SELECT COUNT(*) FROM visits WHERE host = $1)`, opts.Host,
	).Scan(&dstLinks, &dstVisits); err != nil {
		return fmt.Errorf("cannot count target: %w", err)
	}
	if opts.wants("links") && dstLinks != srcLinks {
		return fmt.Errorf("target holds %d links, source holds %d",
			dstLinks, srcLinks)
	}
	if opts.wants("visits") && dstVisits != srcVisits {
		return fmt.Errorf("target holds %d visits, source holds %d",
			dstVisits, srcVisits)
	}

	log.Printf("verified %d links and %d visits", dstLinks, dstVisits)
	return nil
}
