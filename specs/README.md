# Specs

Design specs for redir. Each spec is scoped to land and be verified on its
own; larger work is split rather than fused.

## Postgres migration

redir stores short links and visit records in MongoDB. It is moving to
PostgreSQL, reusing the instance that [urlstat](https://github.com/changkun/urlstat)
already runs on the same host rather than adding a second database
container. The move is split into three specs so that a failure in any one
of them has a single possible cause.

| Spec | Status | Deliverable |
| --- | --- | --- |
| [001-shared-postgres.md](001-shared-postgres.md) | Complete | Extract the PostgreSQL instance out of the urlstat compose project into shared infrastructure both services use as equal clients |
| 002-postgres-store.md | Planned | Replace the MongoDB store with PostgreSQL, enriching the data model at write time and copying the data across while MongoDB stays untouched |
| 003-unify-golang-design.md | Planned | Fold the diverged golang.design/redir deployment into this codebase as a second host and retire its service |

## Status

- ● Complete
- ◐ In progress
- ○ Not started

| Spec | State |
| --- | --- |
| 001-shared-postgres | ● |
| 002-postgres-store | ○ |
| 003-unify-golang-design | ○ |

## Dependencies

```
001-shared-postgres ──> 002-postgres-store ──> 003-unify-golang-design
```

002 needs an instance both services can reach. 003 needs the multi-tenant
schema 002 introduces.
