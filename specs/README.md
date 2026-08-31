# Specs

Design specs for redir. Each spec is scoped to land and be verified on its
own; larger work is split rather than fused.

The migration is finished: redir serves changkun.de and golang.design
from one process, one PostgreSQL database and one codebase.

## Postgres migration

redir stored short links and visit records in MongoDB. It now runs on
PostgreSQL, reusing the instance that [urlstat](https://github.com/changkun/urlstat)
already runs on the same host rather than adding a second database
container. The move was split so that a failure in any one step had a
single possible cause.

The split between 002 and 003 is deliberate. 002 changed the storage and
kept every number identical, so its verification was a diff that had to
come out empty. 003 changes what the numbers mean. Fused, a mismatch
would have had two candidate causes and neither could be ruled out.

| Spec | Status | Deliverable |
| --- | --- | --- |
| [001-shared-postgres.md](001-shared-postgres.md) | Complete | Extract the PostgreSQL instance out of the urlstat compose project into shared infrastructure both services use as equal clients |
| [002-postgres-store.md](002-postgres-store.md) | Complete | Replace the MongoDB store with PostgreSQL and copy the data across, with every stat unchanged and MongoDB untouched |
| [003-enriched-stats.md](003-enriched-stats.md) | Complete | Spend the enriched columns: group user agents and referrers in SQL, exclude bots consistently, drop the client-side parser |
| [004-unify-golang-design.md](004-unify-golang-design.md) | Complete | Fold the diverged golang.design/redir deployment into this codebase as a second host and retire its service |
| [005-drop-mongodb.md](005-drop-mongodb.md) | Complete | Remove the MongoDB backend and stop the container, with v0.7.0 as the release to return to |

## Dashboard

| Spec | Status | Deliverable |
| --- | --- | --- |
| [006-operator-console.md](006-operator-console.md) | Complete | Rebuild the dashboard as a console: totals first, a dense listing second, one link's detail third |

## Status

- ● Complete
- ◐ In progress
- ○ Not started

| Spec | State |
| --- | --- |
| 001-shared-postgres | ● |
| 002-postgres-store | ● |
| 003-enriched-stats | ● |
| 004-unify-golang-design | ● |
| 005-drop-mongodb | ● |
| 006-operator-console | ● |

## Dependencies

```
                              ┌──> 003-enriched-stats
001-shared-postgres ──> 002-postgres-store ──> 005-drop-mongodb
                              └──> 004-unify-golang-design
```

002 needs an instance both services can reach. 003, 004 and 005 all need
what 002 introduces and are independent of each other: 003 changes how
visits are counted, 004 adds a second host, 005 removes the old backend.
Any may land first.

005 gives up the one-line rollback 002 built, in exchange for one store
and one set of tests. The rollback moves to the `v0.7.0` release rather
than disappearing.
