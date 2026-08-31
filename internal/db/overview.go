// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"time"

	"changkun.de/x/redir/internal/models"
)

// StatOverview totals one site over [start, end).
//
// Visits counts everything recorded, so the figure reconciles with the
// store; People and Bots split it the way the statistics page does, so
// the strip at the top of the console and the charts below it are
// counting the same population.
//
// Only visits to aliases that are links are counted, for the reason
// nonBotVisits gives: a fifth of the rows belong to the index page or to
// aliases that never resolved.
func (db *pgStore) StatOverview(
	ctx context.Context,
	host string,
	start, end time.Time,
) (models.Overview, error) {
	var o models.Overview

	err := db.pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM links WHERE host = $1),
		  COUNT(v.id),
		  COUNT(v.id) FILTER (WHERE NOT v.is_bot),
		  COUNT(v.id) FILTER (WHERE v.is_bot)
		FROM links l
		JOIN visits v ON v.host = l.host AND v.alias = l.alias
		WHERE l.host = $1 AND v.time >= $2 AND v.time < $3`,
		host, start, end,
	).Scan(&o.Links, &o.Visits, &o.People, &o.Bots)
	if err != nil {
		return models.Overview{}, fmt.Errorf("failed to total visits: %w", err)
	}

	// The series is drawn as a shape rather than read as numbers, so it
	// is bucketed by day and left with gaps where a day had no traffic;
	// filling them with zeroes is the caller's business, since only it
	// knows how wide the chart is.
	rows, err := db.pool.Query(ctx, `
		SELECT to_char(date_trunc('day', v.time), 'YYYY-MM-DD') AS day,
		       COUNT(*) AS pv,
		       COUNT(DISTINCT v.ip) AS uv
		FROM links l
		JOIN visits v ON v.host = l.host AND v.alias = l.alias
		WHERE l.host = $1 AND v.time >= $2 AND v.time < $3 AND NOT v.is_bot
		GROUP BY 1 ORDER BY 1`, host, start, end)
	if err != nil {
		return models.Overview{}, fmt.Errorf("failed to read the series: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var d models.DayCount
		if err := rows.Scan(&d.Day, &d.PV, &d.UV); err != nil {
			return models.Overview{}, fmt.Errorf("failed to read the series: %w", err)
		}
		o.Series = append(o.Series, d)
	}
	if err := rows.Err(); err != nil {
		return models.Overview{}, fmt.Errorf("failed to read the series: %w", err)
	}
	return o, nil
}

// StatDaily returns a daily non-bot count for each of the given aliases,
// as one query rather than one per alias.
//
// The console draws a sparkline on every row. Asking per row would make a
// page of twenty links twenty round trips for a handful of numbers each,
// which is the kind of cost that only shows up once the page is in front
// of someone on a slow connection.
func (db *pgStore) StatDaily(
	ctx context.Context,
	host string,
	aliases []string,
	start, end time.Time,
) (map[string][]models.DayCount, error) {
	if len(aliases) == 0 {
		return map[string][]models.DayCount{}, nil
	}

	rows, err := db.pool.Query(ctx, `
		SELECT l.alias,
		       to_char(date_trunc('day', v.time), 'YYYY-MM-DD') AS day,
		       COUNT(*) AS pv,
		       COUNT(DISTINCT v.ip) AS uv
		FROM links l
		JOIN visits v ON v.host = l.host AND v.alias = l.alias
		WHERE l.host = $1 AND l.alias = ANY($2)
		  AND v.time >= $3 AND v.time < $4 AND NOT v.is_bot
		GROUP BY l.alias, 2
		ORDER BY l.alias, 2`, host, aliases, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to read daily counts: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]models.DayCount, len(aliases))
	for rows.Next() {
		var (
			alias string
			d     models.DayCount
		)
		if err := rows.Scan(&alias, &d.Day, &d.PV, &d.UV); err != nil {
			return nil, fmt.Errorf("failed to read daily counts: %w", err)
		}
		out[alias] = append(out[alias], d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read daily counts: %w", err)
	}
	return out, nil
}
