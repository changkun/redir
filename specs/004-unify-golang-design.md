---
title: Unify the golang.design deployment
status: planned
depends_on:
  - 002-postgres-store
affects:
  - cmd/migrate
  - internal/config
  - docker/docker-compose.yml
effort: medium
created: 2026-08-31
updated: 2026-08-31
author: changkun
dispatched_task_id: null
---

# Unify the golang.design deployment

## Overview

A second redir runs on the same host for `golang.design`, from a separate
checkout of an older fork of this codebase. It is a duplicate that has
drifted, and it is the reason the schema in `002-postgres-store.md` carries
a `host` column from the start. This spec migrates its data into the shared
PostgreSQL store, serves `golang.design` from the same process, and retires
the second service.

## Current State

`/root/golang.design/redir` on the deploy host: a fork predating the
MongoDB move, storing everything in **SQLite** at `data/redir.db`, 17 MB.

```sql
CREATE TABLE `collink` (
  `id` integer NOT NULL PRIMARY KEY AUTOINCREMENT,
  `alias` varchar(50) NOT NULL DEFAULT '',
  `kind` integer NOT NULL DEFAULT '0',
  `url` varchar(1024) NOT NULL DEFAULT '',
  `private` integer NOT NULL DEFAULT '0',
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  UNIQUE (`alias`)
);
CREATE TABLE `visit` (
  `id` integer NOT NULL PRIMARY KEY AUTOINCREMENT,
  `alias` varchar(50) NOT NULL DEFAULT '',
  `kind` integer NOT NULL DEFAULT '0',
  `ip` varchar(50) DEFAULT NULL,
  `ua` varchar(1000) DEFAULT NULL,
  `referer` varchar(500) DEFAULT NULL,
  `created_at` datetime NOT NULL
);
```

Measured 2026-08-31, read-only:

| Quantity | Value |
| --- | --- |
| links, `kind=0` | 62 |
| links, `kind=1` | 1 |
| visits, `kind=0` | 95,058 |
| visits, `kind=1` | 45 |
| visits with an alias not in `collink` | 0 |
| visits with an empty alias | 0 |
| visits with a NULL ip | 0 |

The fork's data is cleaner than changkun.de's: no orphans, no index-page
rows, no NULL addresses. It has no `visitor_id`, no `valid_from`, no
`trust`, and no `created_by`/`updated_by`.

Both checkouts use the Compose project name `docker`. Running `make down`
or `--remove-orphans` in either destroys the other's container. Retiring
this service must be done by removing its container explicitly.

## Decisions

### kind is dropped

`kind` distinguished short links (`0`) from Go import paths (`1`) when both
were stored. Current redir does not store import paths at all: `xHandler`
(`server.go:119`) answers `/x/` from a template and never touches the
store. The column has no counterpart and one row uses it.

All 63 links migrate as ordinary links and all 95,103 visits as ordinary
visits, under host `golang.design`. The single `kind=1` link and its 45
visits are carried across rather than discarded; they land as a link whose
URL points at the import path, which is what it always was.

### Missing columns take defaults, not invented values

| Column | Value for migrated rows |
| --- | --- |
| `visitor_id` | `NULL`, never fabricated |
| `valid_from` | `created_at` |
| `trust` | `false` |
| `created_by`, `updated_by` | `''` |
| `time` | `created_at` |

### The cache must be keyed by host first

Fixed in 002 rather than left here, because 002 is where the store became
host-aware while the cache above it was not. `short.go` consults the LRU
before it reaches the store, and the cache keyed entries by alias alone.
With one site that is invisible; the moment this spec adds a second, an
alias present on both would serve whichever target was looked up first,
and the wrong redirect would appear in no log, because from the server's
side it is an ordinary cache hit.

### One process, two hosts

`host` is already taken from the request (`002`), so serving both is a
routing change, not a data change. The configuration gains a per-host
section for the title, prefixes and the `x` import path settings, which
differ between the two sites.

## Verification

1. `SELECT COUNT(*) FROM links WHERE host='golang.design'` is 63;
   `visits` is 95,103.
2. Per-alias PV/UV from PostgreSQL matches the same counts computed
   directly from `redir.db` for all 63 aliases.
3. `changkun.de` counts are unchanged by the import: the totals from
   `002` still hold.
4. A `golang.design/s/` alias redirects with 307 through the unified
   process, and `golang.design/x/` still serves the `go-import` meta tag.
5. The changkun.de service is untouched while the golang.design container
   is removed, confirmed by container id and uptime.

## Rollback

`redir.db` is copied, never moved, and the old checkout stays on disk with
its container stopped rather than deleted. Rolling back is starting that
container again and restoring its traefik route. The imported rows are
deleted by `DELETE FROM visits WHERE host='golang.design'`, which is why
`host` is indexed.

## Out of Scope

- Merging the fork's code. It is a duplicate; nothing in it is kept.
- Deleting `/root/golang.design/redir` from the host, until the unified
  service has run long enough to trust.
