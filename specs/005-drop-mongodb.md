---
title: Drop MongoDB
status: planned
depends_on:
  - 002-postgres-store
affects:
  - internal/db
  - internal/models
  - cmd/migrate
  - docker/docker-compose.yml
  - .github/workflows/redir.yml
  - go.mod
effort: medium
created: 2026-08-31
updated: 2026-08-31
author: changkun
dispatched_task_id: null
---

# Drop MongoDB

## Overview

`002-postgres-store.md` moved redir onto PostgreSQL and deliberately kept
the MongoDB backend in the binary, so that a rollback was one line of
configuration. That safety net has a cost: two backends, two sets of
behaviour to keep in step, and a driver dependency for a store nothing
uses.

This spec removes MongoDB from the codebase and stops the container. The
rollback path does not disappear, it moves: **`v0.7.0` is the last release
that speaks both**, and the data stays on disk. Rolling back after this
becomes checking out that tag, rebuilding, and pointing the configuration
back at `mongodb://`, rather than editing one line.

That is a deliberate trade. Rollback goes from a minute to roughly a
quarter of an hour, and in exchange the code has one store, one set of
tests, and no driver for a database that is switched off.

## Current State

- PostgreSQL serves `changkun.de`. MongoDB has taken no writes since the
  cutover at 2026-08-31 12:33 UTC.
- `internal/db` holds both backends behind the `Store` interface:
  `mongo.go`, `mongo_alias.go`, `mongo_stats.go`, `mongo_visit.go`.
- `cmd/migrate` reads MongoDB and writes PostgreSQL.
- `internal/models` carries `bson` tags.
- CI runs a `mongo:4.4` service, and every store test runs against both
  backends.
- `docker/docker-compose.yml` declares the `mongo` service and redir
  `depends_on` it, so redir's start is coupled to a store it no longer
  reads.
- The container publishes `0.0.0.0:27018` with no authentication and no
  users. It is not reachable from outside, but only because of a
  provider-side firewall: `ufw` is inactive and Docker's rules bypass it
  in any case.

## Decisions

### The visit history is not portable backwards

MongoDB froze at 348,367 documents. Every visit since exists only in
PostgreSQL, and nothing copies them back automatically. Before the
container is stopped, the delta is copied into MongoDB once, so that the
frozen state is complete as of shutdown and `v0.7.0` is a lossless
rollback rather than one that silently drops a day of traffic.

After that point the two diverge again and permanently. A rollback taken
later keeps the links and loses the visits recorded in the meantime. This
is stated in the release notes rather than left to be discovered.

### cmd/migrate keeps its writer

Only the MongoDB reader is removed. The tool's PostgreSQL side is not
MongoDB-specific and is what `004-unify-golang-design.md` needs: batched
`CopyFrom`, NUL stripping, zero-timestamp preservation, per-host atomic
link replacement, `-only`, `-truncate`, `-dry-run`, and count
verification, with tests. 004 adds a SQLite reader against that.

Deleting the tool would mean rewriting all of it against a new source
with no tests, which is how the timestamp fault in 002 got in.

### The Store interface stays

One implementation does not usually justify an interface. This one earns
it: `visit_test.go` substitutes a stub to test visit recording and cookie
handling without a database, and `004` will benefit from the seam. The
interface stays; the dispatch behind it does not.

### An attempt to use MongoDB names the way back

`NewStore` rejects a `mongodb://` URI with a message naming `v0.7.0`,
rather than a generic "unsupported scheme". Someone reading that message
is an operator part-way through a rollback, and the message should tell
them what to do.

### Behaviour keeps its tests

Removing the second backend must not remove what the shared tests
asserted. These are behaviours the MongoDB comparison established, and
they survive as PostgreSQL tests in their own right:

| Test | Behaviour |
| --- | --- |
| `TestZeroVisitAliasCountsZero` | an unvisited alias reports 0/0 |
| `TestOrphanVisitsAreNotCounted` | stats join links, so 404 and index rows do not count |
| `TestStatRefererAndUA` | an empty referer or agent reports as `unknown` |
| `TestStatRangeExcludesEnd` | the range is half-open |
| `TestZeroValidFromIsPreserved` | a missing `valid_from` stays "valid since always" |
| `TestHistBucketsAreUTC` | buckets do not follow the session time zone |
| `TestFetchAliasAllPaging` | pages do not repeat links |
| `TestHostsAreSeparate` | `(host, alias)` is the identity |

Coverage of `internal/db` is 84.1% before this change and must not fall.

## Design

### Removed

```
internal/db/mongo.go
internal/db/mongo_alias.go
internal/db/mongo_stats.go
internal/db/mongo_visit.go
cmd/migrate: openMongo, mongoLink, mongoVisit, the source counts in verify
internal/models: every bson tag
go.mod: go.mongodb.org/mongo-driver
.github/workflows: the mongo service and REDIR_TEST_MONGO
docker/docker-compose.yml: the mongo service and redir's depends_on
```

### Kept

```
internal/db/store.go      the Store interface, minus the dispatch
internal/db/postgres.go   unchanged
internal/db/migrate.go    unchanged
cmd/migrate               the PostgreSQL writer and its tests
data/mongo on the host    535 MB, untouched
```

## Steps

1. Copy the post-cutover visits from PostgreSQL into MongoDB, so the
   frozen state is complete.
2. Remove the `mongo` service and redir's `depends_on` from compose,
   recreate redir, then stop `redirdb` explicitly. Never `make down` or
   `--remove-orphans`: `/root/golang.design/redir` shares the Compose
   project name `docker` and either would destroy its container.
3. Remove the code, the dependency and the CI service.
4. Tag the release.

## Verification

1. redir starts and serves with `redirdb` stopped: `/s/`, a redirect,
   the index and `/x/`.
2. `go build ./...` has no MongoDB module in `go.sum`.
3. `internal/db` coverage is at least 84.1%, and every test in the table
   above still exists and passes.
4. A `mongodb://` store URI is rejected with a message naming `v0.7.0`.
5. `data/mongo` is still 535 MB, and `docker start redirdb` alone brings
   the container back.
6. Link and visit counts in PostgreSQL are unchanged across the whole
   operation, apart from live traffic.

## Rollback

Check out `v0.7.0`, rebuild the image, set `store` back to
`mongodb://redirdb:27017`, `docker start redirdb`, and restart redir.
The data directory was never moved or deleted.

Deleting `data/mongo` is not part of this spec and should wait until
PostgreSQL has run long enough that returning to a months-old snapshot
would be pointless anyway.
