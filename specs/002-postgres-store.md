---
title: PostgreSQL store
status: planned
depends_on:
  - 001-shared-postgres
affects:
  - internal/db
  - internal/models
  - internal/short
  - cmd/migrate
  - migrations
  - short.go
  - docker/docker-compose.yml
  - .github/workflows/redir.yml
effort: large
created: 2026-08-31
updated: 2026-08-31
author: changkun
dispatched_task_id: null
---

# PostgreSQL store

## Overview

redir keeps short links and visit records in MongoDB 4.4. This spec adds a
PostgreSQL store on the shared instance from `001-shared-postgres.md`,
copies the data across, and makes PostgreSQL the store redir runs on.

Two constraints shape everything below:

- **MongoDB stays untouched.** The container keeps running, its data is
  never written to, and the Mongo store code stays in the binary. Rollback
  is one line of configuration and a restart, not a rebuild.
- **The migration is verified by comparing numbers, not by inspection.**
  Every stat the dashboard renders today must come out of PostgreSQL with
  the same value it comes out of MongoDB.

The second constraint is why this spec deliberately does **not** change
what the numbers mean. It ports the model and the storage. Changing how
visits are counted and displayed is `003-enriched-stats.md`, which lands
after the port is proven. If both changed at once, a mismatch would have
two possible causes and neither could be ruled out.

The schema is nonetheless the full enriched one, and the migration fills
the enriched columns in the same pass. Those columns are derived, so they
cannot affect the counts being compared, and writing them now avoids a
second pass over the table later.

## Current State

### Data

Measured on production, 2026-08-31, read-only:

| Quantity | Value |
| --- | --- |
| `links` documents | 184 |
| `visit` documents | 348,356 |
| distinct `ip` | 73,215 |
| distinct `visitor_id` | 277,318 |
| visits whose alias is not in `links` | 72,118 |
| of those, alias `""` | 68,963 |
| aliases with zero visits | 0 |
| documents missing `visitor_id` | 10,708 |
| documents whose `visitor_id` is not a UUID | 505 |
| documents with empty `ua` | 8,205 |
| documents with empty `referer` | 187,175 |

Every document has `ua`, `referer` and `time`. No document has an empty,
`unknown`, or hashed `ip`, so `hide_ip` has never been enabled here.

### Code

`internal/db` is four files against `mongo-driver` v1: `db.go` holds the
`*mongo.Client`, `alias.go` the link CRUD, `visit.go` the single insert,
`stats.go` four aggregation pipelines. `internal/models` carries `bson`
tags. `internal/short/cmd.go:49` takes the concrete `*db.Store`.

Visits are recorded at `short.go:168` **before** the alias is resolved, and
`sIndex` records a visit with `alias == ""` for the index page itself
(`short.go:492`).

## Decisions

### No foreign key from visits to links

The obvious model gives `visits` a foreign key into `links`. Production
data rejects it: **72,118 visit rows, 21% of the table, have an alias that
is not in `links`.** They are not corruption. They are three legitimate
kinds of traffic:

| Kind | Rows | Cause |
| --- | --- | --- |
| Index page | 68,963 | `sIndex` records `alias == ""`, which is not a link |
| Unresolved alias | 3,155 | recorded before resolution, so 404s count |
| Deleted alias | included above | `DeleteAlias` removes the link, not its history |

A foreign key would force us to discard a fifth of the visit history or to
invent link rows for aliases that never existed. `alias` is therefore a
plain indexed `TEXT` column, and referential integrity is not claimed.

The consequence is that a query over `visits` alone is **not** equivalent
to today's stats. Every Mongo pipeline starts from `links` and `$lookup`s
the visits, so orphan rows are invisible to it. The PostgreSQL queries must
`JOIN links USING (host, alias)` to reproduce that. This is the single
easiest way to get a wrong verification result.

### UV stays distinct IP

The plan of record was to switch UV to `visitor_id` with an IP fallback.
The data says that is wrong:

```
distinct visitor_id / total visits = 277,318 / 348,356 = 80%
distinct ip         / total visits =  73,215 / 348,356 = 21%
```

`recognizeVisitor` sets the cookie with `w.Header().Set` and no `Max-Age`,
so it is a session cookie, and `RecordVisit` mints a fresh UUID whenever
the request carries none. Four visits in five therefore invent a new
`visitor_id`. It measures visits, not people, and counting it would inflate
UV roughly fourfold. **UV remains `COUNT(DISTINCT ip)`.**

`visitor_id` is still stored, and `003` may revisit it once the cookie is
persistent, which is a change to the cookie, not to the schema.

### visitor_id is validated on the way in

The cookie is currently trusted verbatim. Production holds values such as

```
-1%20OR%202%2B471-471-1=0%2B0%2B0%2B1%20--%20
if(now()=sysdate()%2Csleep(15)%2C0)
```

which are scanner probes echoed straight back to the client by
`w.Header().Set("Set-Cookie", ...)`. This is not header injection: Go's
cookie parser rejects control characters, verified against `net/http`. It
is unvalidated input reaching the database, and it is what stops
`visitor_id` from being a `UUID` column.

The store parses the cookie as a UUID and mints a new one when it does not
parse. Historical values that do not parse migrate as `NULL`; they are not
replaced with fabricated UUIDs, because a made-up identifier is worse than
an absent one.

### host is derived from the request

`004-unify-golang-design.md` retires the second deployment, so one process
will serve `changkun.de` and `golang.design`. Taking `host` from
configuration would need a re-migration then. It comes from `r.Host` with
the port stripped and lowercased, and the CLI takes `--host`, defaulting to
the configured primary host. Uniqueness is `(host, alias)`, never `alias`.

### Both stores stay in the binary

`config.Conf.Store` is already a URI. `db.NewStore` dispatches on its
scheme, so `mongodb://` and `postgres://` both work and rollback is a
configuration edit. `db.Store` becomes an interface; `internal/short/cmd.go`
takes that interface instead of the concrete type.

### ip is TEXT, visitor_id is nullable

urlstat's `visits` table is the obvious template. Two of its column types
do not survive contact with redir's data and must not be "simplified" back:

| urlstat | redir | Reason |
| --- | --- | --- |
| `ip INET NOT NULL` | `ip TEXT NOT NULL` | `hide_ip: true` makes `utils.ReadIP` return a SHA-1 hex string, or the literal `unknown` when hashing fails. Neither is an address. |
| `visitor_id UUID NOT NULL` | `visitor_id UUID` | 11,213 historical rows have no parseable UUID. |

## Design

### Schema is a migration file, applied by the server

The schema lives in `migrations/`, one numbered SQL file per change,
starting with `001_initial.sql`. It is **not** applied by the postgres
image's `/docker-entrypoint-initdb.d` hook. `001-shared-postgres.md`
records why: that hook runs only when `PGDATA` is empty, and the shared
instance was initialised long ago by urlstat, so files placed there are a
silent no-op. urlstat still bind-mounts its `migrations/` directory and has
been getting nothing from it.

redir therefore applies its own migrations. The files are embedded with
`go:embed`, and the server runs them at startup inside a transaction,
tracking what has been applied:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    BIGINT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Each file runs once, in version order, in a single transaction with its
`schema_migrations` insert, so a failure leaves nothing half-applied.
Startup fails loudly if a migration fails; a store that is missing its
schema must not serve traffic.

The runner takes `pg_advisory_lock` on a fixed key for the duration, so two
instances starting together cannot both apply the same file.

The database and role are created once, by hand, as the postgres superuser,
because they are outside what the application may grant itself:

```sql
CREATE ROLE redir LOGIN PASSWORD '...';
CREATE DATABASE redir OWNER redir;
```

`001_initial.sql`:

```sql
CREATE TABLE links (
    id         BIGSERIAL PRIMARY KEY,
    host       TEXT NOT NULL,
    alias      TEXT NOT NULL,
    url        TEXT NOT NULL,
    private    BOOLEAN NOT NULL DEFAULT FALSE,
    trust      BOOLEAN NOT NULL DEFAULT FALSE,
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (host, alias)
);

CREATE TABLE visits (
    id           BIGSERIAL PRIMARY KEY,
    host         TEXT NOT NULL,
    alias        TEXT NOT NULL,          -- '' is the index page; no FK, by design
    visitor_id   UUID,                   -- NULL when the historical value was not a UUID
    ip           TEXT NOT NULL,          -- may be a SHA-1 hex digest under hide_ip
    ua           TEXT NOT NULL DEFAULT '',
    referer      TEXT NOT NULL DEFAULT '',
    referer_host TEXT NOT NULL DEFAULT '',  -- derived
    browser      TEXT NOT NULL DEFAULT '',  -- derived
    os           TEXT NOT NULL DEFAULT '',  -- derived
    device       TEXT NOT NULL DEFAULT '',  -- derived
    is_bot       BOOLEAN NOT NULL DEFAULT FALSE, -- derived
    time         TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_visits_host_alias_time ON visits (host, alias, time DESC);
CREATE INDEX idx_visits_host_alias_ip   ON visits (host, alias, ip);
CREATE INDEX idx_links_host_updated     ON links  (host, updated_at DESC);
```

The four derived columns are computed once at write time from `ua` and
`referer`. Nothing in this spec reads them; `003` does.

### Parity of the stat queries

Each Mongo pipeline maps to one statement. All of them join `links`, for
the reason given above.

| Method | PostgreSQL |
| --- | --- |
| `StatVisit` | `GROUP BY l.alias`, `COUNT(*)` as PV, `COUNT(DISTINCT v.ip)` as UV |
| `StatReferer` | `GROUP BY` referer, empty mapped to `unknown`, `ORDER BY count DESC` |
| `StatUA` | `GROUP BY` ua, empty mapped to `unknown`, `ORDER BY count DESC` |
| `StatVisitHist` | `GROUP BY date_trunc('hour', v.time)`, PV and distinct-IP UV |
| `FetchAliasAll` | `LEFT JOIN` visits, PV/UV per link, `ORDER BY updated_at DESC` |

The `unknown` mapping is preserved even though it is ugly, because the
dashboard filters on that exact string (`Stats.jsx:72`, `Stats.jsx:156`).
`003` removes it from both sides at once.

`LEFT JOIN` in `FetchAliasAll` is what makes a zero-visit alias report
`pv=0, uv=0`. Mongo's `preserveNullAndEmptyArrays` reports `pv=1, uv=1` for
such an alias, counting the synthetic null row. Production has no
zero-visit alias today, so the two agree, and verification must not be run
after creating one.

### Migration tool

`cmd/migrate` streams MongoDB into PostgreSQL with `pgx.CopyFrom` in
batches of 10,000. It opens Mongo read-only, never writes to it, and
refuses to start if the target tables are non-empty unless `--truncate` is
given.

Per row it: fills `host` from `--host`; parses `visitor_id`, `NULL` on
failure; strips NUL bytes and invalid UTF-8, which PostgreSQL `TEXT`
rejects and BSON permits; derives the four enriched columns with the same
code path the server uses, so a row written by the migration and a row
written by the server are identical.

## Verification

Run against production data, both stores live, before cutover.

1. **Row counts.** `links` 184, `visits` 348,356.
2. **Per-alias PV/UV.** Dump `StatVisit` from both stores for all 184
   aliases and diff. Expected: byte-identical.
   ```
   pv_pg(alias) == pv_mongo(alias)   for every alias
   uv_pg(alias) == uv_mongo(alias)   for every alias
   ```
3. **Totals.** `SELECT COUNT(*) FROM visits` equals `db.visit.count()`,
   including the 72,118 orphans, which no stat query returns but the
   migration must still carry.
4. **Per-alias stats.** For the ten highest-PV aliases, diff `referer`,
   `ua` and hourly `time` output between stores over the default range.
5. **Derived columns.** No enriched column is `NULL`; `is_bot` is true for
   a hand-checked sample of known crawler UA strings.
6. **Migrations.** A second startup against a migrated database applies
   nothing and logs nothing; `schema_migrations` holds one row per file.
7. **End to end.** With the store URI on PostgreSQL, a redirect returns
   307 and records exactly one row; the dashboard renders identical charts.

## Testing

CI gains a `postgres:16-alpine` service container, so the store is tested
against real PostgreSQL rather than a mock. The Mongo tests keep their
skip-on-no-connection guard.

- Store tests for every method, run against both backends through the
  `db.Store` interface, so parity is asserted by the test names matching.
- `visitor_id` validation: a non-UUID cookie yields a fresh UUID and stores
  a valid one. This is a regression test for the values found in
  production and must fail against the current code.
- Migration runner: applying to an empty database creates the schema;
  applying twice is a no-op; a failing file rolls back and returns an error.
- Data migration: a seeded Mongo fixture including an orphan visit, a missing
  `visitor_id`, a non-UUID `visitor_id` and an invalid UTF-8 string
  migrates with the row count preserved.
- Stat parity: the same fixture yields identical `StatVisit` output from
  both stores.

Coverage target for `internal/db` is 90%.

## Rollback

Set `store` back to the `mongodb://` URI and restart. MongoDB has been
running and unmodified throughout. The PostgreSQL data is left in place;
re-running the migration needs `--truncate`.

## Out of Scope

- Changing what any number means, including the UV metric: `003`.
- Removing `ua-parser-js` from the dashboard: `003`.
- The golang.design deployment: `004`.
- Dropping MongoDB. It stays until PostgreSQL has run unattended for a
  period the operator judges sufficient.
