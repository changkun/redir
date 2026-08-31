// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"changkun.de/x/redir/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"uuid"
)

// pgStore is the PostgreSQL backend.
//
// The instance is shared with another service, so this connects to its own
// database with its own role and applies its own schema; see
// specs/001-shared-postgres.md.
type pgStore struct {
	pool *pgxpool.Pool
}

// ErrAliasExists reports that (host, alias) is taken. It replaces the
// MongoDB backend's bare "alias already existed" string, which callers
// could only match on by comparing text.
var ErrAliasExists = errors.New("alias already existed")

// uniqueViolation is the SQLSTATE for a unique constraint breach.
const uniqueViolation = "23505"

func newPostgresStore(ctx context.Context, uri string) (*pgStore, error) {
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}

	// A store without its schema must not serve traffic, so a failure
	// here is returned rather than logged and stepped over.
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	log.Printf("connected to %v", Redact(uri))
	return &pgStore{pool: pool}, nil
}

func (db *pgStore) Close() error {
	db.pool.Close()
	return nil
}

func (db *pgStore) StoreAlias(ctx context.Context, r *models.Redir) error {
	now := time.Now().UTC()
	validFrom := r.ValidFrom
	if validFrom.IsZero() {
		validFrom = now
	}

	var id int64
	err := db.pool.QueryRow(ctx, `
		INSERT INTO links
			(host, alias, url, private, trust, valid_from,
			 created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id`,
		r.Host, r.Alias, r.URL, r.Private, r.Trust, validFrom,
		r.CreatedBy, r.UpdatedBy, now,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return ErrAliasExists
		}
		return fmt.Errorf("failed to insert given redirect: %w", err)
	}
	r.ID = strconv.FormatInt(id, 10)
	return nil
}

func (db *pgStore) UpdateAlias(ctx context.Context, r *models.Redir) error {
	if r.ID == "" {
		return errors.New("missing document ID")
	}
	id, err := strconv.ParseInt(r.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("unparseable id %q: %w", r.ID, err)
	}

	tag, err := db.pool.Exec(ctx, `
		UPDATE links SET
			alias = $2, url = $3, private = $4, trust = $5,
			valid_from = $6, updated_by = $7, updated_at = $8
		WHERE id = $1`,
		id, r.Alias, r.URL, r.Private, r.Trust,
		r.ValidFrom, r.UpdatedBy, time.Now().UTC(),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return ErrAliasExists
		}
		return fmt.Errorf("failed to update alias %s: %v", r.Alias, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("failed to update alias %s: no such link", r.Alias)
	}
	return nil
}

// DeleteAlias removes the link and leaves its visits in place. The visits
// happened, and the alias may be created again later; the stat queries
// join links, so an orphaned history reappears if it is.
func (db *pgStore) DeleteAlias(ctx context.Context, host, alias string) error {
	_, err := db.pool.Exec(ctx,
		`DELETE FROM links WHERE host = $1 AND alias = $2`, host, alias)
	if err != nil {
		return fmt.Errorf("delete alias %s failed: %w", alias, err)
	}
	return nil
}

func (db *pgStore) FetchAlias(ctx context.Context, host, alias string) (*models.Redir, error) {
	var (
		r  models.Redir
		id int64
	)
	err := db.pool.QueryRow(ctx, `
		SELECT id, host, alias, url, private, trust, valid_from,
		       created_by, updated_by, created_at, updated_at
		FROM links WHERE host = $1 AND alias = $2`, host, alias,
	).Scan(&id, &r.Host, &r.Alias, &r.URL, &r.Private, &r.Trust,
		&r.ValidFrom, &r.CreatedBy, &r.UpdatedBy, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("cannot find alias %s: %v", alias, err)
	}
	r.ID = strconv.FormatInt(id, 10)
	return &r, nil
}

// FetchAliasAll returns a page of links.
//
// The public form omits the URL and the counts, as the MongoDB backend
// does, so that the public index cannot be used to enumerate targets or to
// read traffic figures.
//
// The counts come from a LEFT JOIN, so a link with no visits reports
// pv=0, uv=0. MongoDB reports pv=1, uv=1 for that case, because
// preserveNullAndEmptyArrays keeps a synthetic null row and the pipeline
// counts it. Production has no such link, so the two agree there; where
// they differ, this one is right.
func (db *pgStore) FetchAliasAll(
	ctx context.Context,
	host string,
	public bool,
	pageSize, pageNum int64,
) ([]models.RedirIndex, int64, error) {
	offset := (pageNum - 1) * pageSize

	var total int64
	countQuery := `SELECT COUNT(*) FROM links WHERE host = $1`
	if public {
		countQuery += ` AND NOT private`
	}
	if err := db.pool.QueryRow(ctx, countQuery, host).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("cannot count links: %w", err)
	}

	query := `
		SELECT l.id, l.host, l.alias, l.url, l.private, l.trust,
		       l.valid_from, l.created_by, l.updated_by,
		       l.created_at, l.updated_at,
		       COUNT(v.id) AS pv,
		       COUNT(DISTINCT v.ip) AS uv
		FROM links l
		LEFT JOIN visits v ON v.host = l.host AND v.alias = l.alias
		WHERE l.host = $1
		GROUP BY l.id
		ORDER BY l.updated_at DESC
		LIMIT $2 OFFSET $3`
	if public {
		// No join at all: the public index shows neither URL nor counts,
		// so aggregating over the visit table would be work whose result
		// is discarded.
		query = `
		SELECT id, host, alias, '', private, trust,
		       valid_from, created_by, updated_by,
		       created_at, updated_at, 0, 0
		FROM links
		WHERE host = $1 AND NOT private
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`
	}

	rows, err := db.pool.Query(ctx, query, host, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot fetch links: %w", err)
	}
	defer rows.Close()

	var rs []models.RedirIndex
	for rows.Next() {
		var (
			r  models.RedirIndex
			id int64
		)
		if err := rows.Scan(&id, &r.Host, &r.Alias, &r.URL, &r.Private,
			&r.Trust, &r.ValidFrom, &r.CreatedBy, &r.UpdatedBy,
			&r.CreatedAt, &r.UpdatedAt, &r.PV, &r.UV); err != nil {
			return nil, 0, fmt.Errorf("cannot scan link: %w", err)
		}
		r.ID = strconv.FormatInt(id, 10)
		rs = append(rs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("cannot fetch links: %w", err)
	}
	return rs, total, nil
}

// RecordVisit stores one visit and returns the visitor id to set as a
// cookie.
//
// The incoming id is validated as a UUID rather than trusted. Production
// holds values such as "-1%20OR%202%2B471-471-1=0%2B0%2B0%2B1%20--%20",
// which are scanner probes that were stored verbatim and echoed back in
// the Set-Cookie header.
func (db *pgStore) RecordVisit(ctx context.Context, v *models.Visit) (string, error) {
	vid, err := uuid.Parse(v.VisitorID)
	if err != nil {
		vid = uuid.New()
	}
	v.VisitorID = vid.String()
	v.Derive()

	when := v.Time
	if when.IsZero() {
		when = time.Now().UTC()
	}

	_, err = db.pool.Exec(ctx, `
		INSERT INTO visits
			(host, alias, visitor_id, ip, ua, referer,
			 referer_host, browser, os, device, is_bot, time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		v.Host, v.Alias, vid, v.IP, v.UA, v.Referer,
		v.RefererHost, v.Browser, v.OS, v.Device, v.IsBot, when)
	if err != nil {
		return "", fmt.Errorf("failed to insert record: %w", err)
	}
	return v.VisitorID, nil
}

// joinVisits is the FROM clause every stat query shares.
//
// The join to links is not decoration. 21% of the visit rows have an alias
// that is not a link: the index page records an empty alias, and a visit
// is recorded before the alias is resolved, so 404s are counted too. Every
// MongoDB pipeline starts from links and looks the visits up, so it never
// sees those rows. Querying visits alone would return different numbers.
const joinVisits = `
	FROM links l
	JOIN visits v ON v.host = l.host AND v.alias = l.alias
	WHERE l.host = $1 AND l.alias = $2 AND v.time >= $3 AND v.time < $4`

func (db *pgStore) StatReferer(
	ctx context.Context,
	host, alias string,
	start, end time.Time,
) ([]models.RefStat, error) {
	// The empty referer is reported as "unknown" because that is the
	// string the MongoDB pipeline invents and the dashboard matches on.
	rows, err := db.pool.Query(ctx, `
		SELECT CASE WHEN v.referer = '' THEN 'unknown' ELSE v.referer END AS referer,
		       COUNT(*) AS count`+joinVisits+`
		GROUP BY 1 ORDER BY count DESC`, host, alias, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to count referer: %w", err)
	}
	defer rows.Close()

	var rs []models.RefStat
	for rows.Next() {
		var r models.RefStat
		if err := rows.Scan(&r.Referer, &r.Count); err != nil {
			return nil, fmt.Errorf("failed to fetch referer results: %w", err)
		}
		rs = append(rs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to fetch referer results: %w", err)
	}
	return rs, nil
}

func (db *pgStore) StatUA(
	ctx context.Context,
	host, alias string,
	start, end time.Time,
) ([]models.UAStat, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT CASE WHEN v.ua = '' THEN 'unknown' ELSE v.ua END AS ua,
		       COUNT(*) AS count`+joinVisits+`
		GROUP BY 1 ORDER BY count DESC`, host, alias, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to count ua: %w", err)
	}
	defer rows.Close()

	var rs []models.UAStat
	for rows.Next() {
		var r models.UAStat
		if err := rows.Scan(&r.UA, &r.Count); err != nil {
			return nil, fmt.Errorf("failed to fetch ua results: %w", err)
		}
		rs = append(rs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to fetch ua results: %w", err)
	}
	return rs, nil
}

// StatVisitHist buckets by hour, which is what the MongoDB pipeline does
// by grouping on year, month, day and hour.
func (db *pgStore) StatVisitHist(
	ctx context.Context,
	host, alias string,
	start, end time.Time,
) ([]models.TimeHist, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT date_trunc('hour', v.time) AS bucket,
		       COUNT(*) AS pv,
		       COUNT(DISTINCT v.ip) AS uv`+joinVisits+`
		GROUP BY bucket ORDER BY bucket`, host, alias, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to count time hist: %w", err)
	}
	defer rows.Close()

	var rs []models.TimeHist
	for rows.Next() {
		var r models.TimeHist
		if err := rows.Scan(&r.Time, &r.PV, &r.UV); err != nil {
			return nil, fmt.Errorf("failed to fetch time hist results: %w", err)
		}
		rs = append(rs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to fetch time hist results: %w", err)
	}
	return rs, nil
}

// StatVisit counts PV and UV for the given aliases.
//
// UV is a distinct count over ip, not over visitor_id. The cookie carrying
// visitor_id has no Max-Age and RecordVisit mints one whenever a request
// arrives without it, so 277,318 of 348,356 production visits invented a
// visitor. It counts visits, not people.
func (db *pgStore) StatVisit(
	ctx context.Context,
	host string,
	aliases []string,
) ([]models.VisitRecord, error) {
	if len(aliases) == 0 {
		return nil, nil
	}

	rows, err := db.pool.Query(ctx, `
		SELECT l.alias,
		       COUNT(v.id) AS pv,
		       COUNT(DISTINCT v.ip) AS uv
		FROM links l
		LEFT JOIN visits v ON v.host = l.host AND v.alias = l.alias
		WHERE l.host = $1 AND l.alias = ANY($2)
		GROUP BY l.alias
		ORDER BY pv DESC, uv DESC`, host, aliases)
	if err != nil {
		return nil, fmt.Errorf("failed to count visit: %w", err)
	}
	defer rows.Close()

	var rs []models.VisitRecord
	for rows.Next() {
		var r models.VisitRecord
		if err := rows.Scan(&r.Alias, &r.PV, &r.UV); err != nil {
			return nil, fmt.Errorf("failed to fetch visit results: %w", err)
		}
		rs = append(rs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to fetch visit results: %w", err)
	}
	return rs, nil
}

var _ Store = (*pgStore)(nil)
