---
title: Unify the golang.design deployment
status: complete
depends_on:
  - 002-postgres-store
affects:
  - internal/migrate
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
routing change, not a data change.

Almost nothing is actually per-site. `title` is configured but read
nowhere. The `/x/` import path is already built from `req.Host`. The
prefixes, VCS and godoc host are identical. What differs is exactly one
value, `x.repo_path`, which is `github.com/changkun` for one site and
`github.com/golang-design` for the other, plus whether the legal pages
apply.

The configuration therefore gains an override map rather than a second
full config:

```yaml
host: https://changkun.de
x:
  repo_path: https://github.com/changkun
hosts:
  golang.design:
    repo_path: https://github.com/golang-design
    legal: false
```

`config.Conf.Site(r.Host)` resolves one request to the values for its
site. An unrecognised Host header resolves to the primary site, as
`ResolveHost` already does, because the header is client-controlled and
reaches the store as part of a link's identity.

### The legal pages do not follow

changkun.de serves an impressum and a privacy policy naming its operator.
golang.design never had them, and showing one site's legal pages under the
other's domain would misstate who operates it. `legal: false` suppresses
the impressum, privacy and contact links for a site. This is a visible
difference between the two index pages and is intended.

### checkvcs must use the resolved repo path

`checkvcs` runs when `/s/<alias>` finds no link. It requests
`repo_path + "/" + alias` and, on 200 or 301, **creates a link row**. With
two sites that probe must go to the site's own organisation, and the row
it writes must carry the resolved host: a miss on `golang.design/s/foo`
has to probe `github.com/golang-design/foo` and never
`github.com/changkun/foo`. This is the sharpest edge in the change, and it
is covered by a test rather than by inspection.

### Visits are replaced per host, not truncated

`internal/migrate` emptied the whole `visits` table before a copy. That
was safe when one site's data was in it. It is not now: importing
golang.design would have deleted changkun.de's 348,435 rows. The delete is
scoped to the host being imported, matching what `copyLinks` already did.

### The switch window

Between the import finishing and traefik moving the route, the old service
is still serving golang.design and still writing to SQLite. Those visits
are in neither PostgreSQL nor the import that already ran.

Rather than take the gap or a truncate-and-reload that would discard what
the new service recorded, the source takes a `since` filter and the
migration takes an append mode: a second pass copies only the visits
recorded after the first pass, adding to the table instead of replacing
it. `002` had the same window and got away with it on traffic timing;
this closes it instead.

## Implementation note

`internal/migrate` already holds everything this needs except a reader:
batched loading, NUL stripping, the rule that a missing timestamp is
carried rather than replaced, per-host atomic link replacement, `Only`,
`Truncate`, `DryRun` and the count check, with tests against a fake
source. This spec supplies a `migrate.Source` over `redir.db` and a
`cmd/migrate` to drive it.

## Verification

The PV/UV comparison needs a stated baseline, because it cannot be equal
to what the old dashboard showed. `003` excludes bots from the stats page
and the SQLite fork counted everything, so per-alias figures on the stats
page will be **lower**. The two comparisons that must hold are on stored
rows, not on displayed statistics:

```
count(visits where host='golang.design')  ==  count(sqlite visit)
count(links  where host='golang.design')  ==  count(sqlite collink)
```

and per alias, over all traffic rather than the bot-free view:

```
pv_pg(alias)  ==  count(sqlite visit where alias = alias)
```

1. `SELECT COUNT(*) FROM links WHERE host='golang.design'` is 63;
   `visits` is 95,103 plus whatever arrived before the switch.
2. Per-alias PV/UV from PostgreSQL matches the same counts computed
   directly from `redir.db` for all 63 aliases.
3. `changkun.de` counts are unchanged by the import: the totals from
   `002` still hold.
4. A `golang.design/s/` alias redirects with 307 through the unified
   process, and `golang.design/x/` still serves the `go-import` meta tag.
5. The changkun.de service is untouched while the golang.design container
   is removed, confirmed by container id and uptime.

## Outcome

Completed 2026-08-31. `golang.design/s/` and `/x/` are served by
`changkun/redir`, from the same process and store as `changkun.de`.

### Verified

| Check | Result |
| --- | --- |
| Data imported | 63 links, 95,138 visits |
| Per-alias parity | all 63 aliases identical to SQLite on PV and UV, rehearsed on a copy of the real file before touching production |
| changkun.de | untouched: 184 links, 348,449 visits, serving throughout |
| Per-host separation | 104 public links against 63; `email` exists on both hosts with entirely different statistics |
| `/x/` | `go-import` names `golang-design/lockfree`, not `changkun/lockfree` |
| Redirects | all 63 return 307 to their targets |
| Visit total | 95,142 against SQLite's 95,139: consistent and larger, the difference being traffic recorded after the switch |

### What went wrong

**Every external link showed an interstitial.** The fork has no notion of
a trusted link, so its 63 links imported as untrusted, and the current
code shows a warning page before an untrusted external redirect. 62 of 63
links stopped redirecting directly. Caught by checking the status codes
after the cutover rather than only that the pages loaded: they returned
200, not 307. The links were marked trusted, which is the behaviour they
had.

The lesson is the same as the timestamp fault in `002`: a column the
source does not have is not a column whose default is safe. `trust`
absent meant "no such concept", not "false".

**The incremental pass would have duplicated a row.** `created_at` is
TEXT holding a Go timestamp with nanoseconds, so `created_at > ?` compared
the stored string against whatever layout the driver renders a `time.Time`
in. Against the real file the row exactly at the watermark came back as
later than itself. The unit tests missed it because they wrote and read
through the same driver, the one arrangement where the formats agree.

It turned out not to matter: the only row the window held was a request
from `127.0.0.1`, made by this migration's own testing against the old
container.

**The import was more machinery than the job needed.** A `Source`
interface, a SQLite package, a driver dependency and its tests, for one
import that `sqlite3` dumping CSV and `psql` loading it would have done,
with `-rederive` filling the derived columns. The driver also reached the
server binary, which imports the migration package for `-rederive`, adding
5.7 MB of database engine the server never opens. All of it is removed;
`Rederive` moved to `internal/db`, which is the part with a continuing
job.

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
