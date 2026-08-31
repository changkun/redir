// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"log"

	"changkun.de/x/redir/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rederiveBatchSize is how many visits are read per round trip.
const rederiveBatchSize = 10000

// Rederive recomputes the derived columns of stored visits.
//
// browser, os, device, is_bot and referer_host are a cached pure function
// of ua and referer. When that function improves, the cache is stale, and
// leaving it stale means historical rows are classified by one rule and
// new rows by another, so a chart mixes the two.
//
// It reads only ua and referer, which are the columns the derivation
// takes, and rewrites only the columns it produces. Running it twice
// changes nothing the second time, so it is safe to re-run after a rule
// changes again.
func Rederive(ctx context.Context, uri, host string) error {
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		return fmt.Errorf("cannot connect: %w", err)
	}
	defer pool.Close()

	var lastID, seen, changed int64
	for {
		// Walk by primary key in batches rather than holding one cursor
		// across the whole table, so a long run does not pin a snapshot
		// while live traffic is being written.
		n, err := rederiveBatch(ctx, pool, host, &lastID, &changed)
		if err != nil {
			return err
		}
		seen += n
		if n < rederiveBatchSize {
			break
		}
		log.Printf("re-derived %d visits, %d changed", seen, changed)
	}

	log.Printf("re-derived %d visits for host %v, %d changed", seen, host, changed)
	return nil
}

// rederiveBatch processes one batch and reports how many rows it read.
// Fewer than rederiveBatchSize means the table is exhausted.
func rederiveBatch(
	ctx context.Context,
	pool *pgxpool.Pool,
	host string,
	lastID, changed *int64,
) (int64, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, ua, referer, browser, os, device, is_bot, referer_host
		FROM visits
		WHERE host = $1 AND id > $2
		ORDER BY id
		LIMIT $3`, host, *lastID, rederiveBatchSize)
	if err != nil {
		return 0, fmt.Errorf("cannot read visits: %w", err)
	}

	type update struct {
		id int64
		v  models.Visit
	}
	var (
		seen  int64
		batch []update
	)
	for rows.Next() {
		var (
			id                               int64
			ua, referer                      string
			browser, os, device, refererHost string
			isBot                            bool
		)
		if err := rows.Scan(&id, &ua, &referer,
			&browser, &os, &device, &isBot, &refererHost); err != nil {
			rows.Close()
			return 0, fmt.Errorf("cannot scan visit: %w", err)
		}
		*lastID = id
		seen++

		v := models.Visit{UA: ua, Referer: referer}
		v.Derive()
		// Only rows the rule now classifies differently are written.
		if v.Browser == browser && v.OS == os && v.Device == device &&
			v.IsBot == isBot && v.RefererHost == refererHost {
			continue
		}
		batch = append(batch, update{id: id, v: v})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("cannot read visits: %w", err)
	}

	for _, u := range batch {
		if _, err := pool.Exec(ctx, `
			UPDATE visits SET browser = $2, os = $3, device = $4,
			                  is_bot = $5, referer_host = $6
			WHERE id = $1`,
			u.id, u.v.Browser, u.v.OS, u.v.Device,
			u.v.IsBot, u.v.RefererHost); err != nil {
			return 0, fmt.Errorf("cannot update visit %d: %w", u.id, err)
		}
	}
	*changed += int64(len(batch))
	return seen, nil
}
