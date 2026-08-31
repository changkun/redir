---
title: Operator console
status: planned
depends_on:
  - 003-enriched-stats
  - 004-unify-golang-design
affects:
  - dashboard
  - internal/db
  - internal/models
  - short.go
effort: large
created: 2026-09-01
updated: 2026-09-01
author: changkun
dispatched_task_id: null
---

# Operator console

## Overview

The dashboard is an `EditableProTable` on a bare page. It answers one
question, "what links exist", and answers it in eleven columns of equal
weight. An operator who wants to know whether anything is happening has to
expand a row, wait for three charts to load, and do it again for the next
alias.

This rebuilds it as a console: what is happening first, what exists
second, and the detail of one link third. It also drops
`@ant-design/pro-components`, which is where both the look and 3.39 MB of
the 3.39 MB bundle come from.

## Current State

```
dashboard/build/static/index-*.js   3,389 KB
```

`Home.jsx` is a `Layout` with a 28px text link for a header, a table, and
a footer reading "© 2021". `RedirTable.jsx` builds eleven columns and puts
`PV/UV` in one of them as the string `"31907/26839"`. Per-alias statistics
live behind row expansion, one alias at a time, as three 300px charts.

Nothing shows a total. Nothing shows a trend. Nothing says which of the
two sites is being administered, though the same process now serves both.

## Decisions

### antd stays, pro-components goes

`EditableProTable`, `ModalForm` and `ProFormText` dictate the layout, the
density and the type, and pull in `pro-layout`, `pro-list`, `pro-form`
and `pro-utils`. antd's own `Table`, `Form`, `Modal` and `DatePicker` do
the same work without deciding what the page looks like, and keep the
parts worth keeping: inline editing, validation, date entry and their
keyboard and screen reader behaviour.

### The visual language is a console, not a template

Dark, dense, restrained. Tabular numerals so figures line up in a column.
Monospace for aliases and URLs, which are identifiers and read better in
one. Hairline separators rather than cards, since a card around every
group triples the ink for no information. One accent colour, used only to
say something is unusual.

### The server sends the shape the page draws

Two additions, because the page cannot ask for what it needs today.

**Overview.** One request returns the site's totals and its daily series,
so the strip at the top is one query rather than none.

```
GET /s/?mode=overview&t0=&t1=
{"links":184,"visits":348465,"people":151006,"bots":197459,
 "series":[{"day":"2026-08-25","pv":812,"uv":403}, ...]}
```

**A series per row.** A sparkline for each alias on the page cannot be one
request per row: a page of twenty rows would be twenty round trips for
twenty numbers each. The index response carries the series for the aliases
it returns, in the same query.

```
GET /s/?mode=index-pro&pn=1&ps=20
{"data":[{"alias":"blog", ..., "series":[3,7,12,9,4,6,8]}], ...}
```

Both count non-bot visits, matching what `003` established the statistics
page shows.

### The public index is the same design, less of it

A visitor needs the link and where it goes. They get the same type,
spacing and hairlines, without totals, sparklines, editing or the account
menu. One design language, two densities.

## Design

```
┌────────────────────────────────────────────────────────────┐
│  redir  changkun.de                    ⌘K        hi@… ▾    │  header
├────────────────────────────────────────────────────────────┤
│  184        348,465       151,006        56.7%             │  overview
│  links      visits        people         automated         │
│  ▁▂▃▅▇▅▃▂▁▂▄▆▇▅▃▂▁▃▅▇▆▄▂▁▂▃▅▇▅▃▂                          │
├────────────────────────────────────────────────────────────┤
│  ALIAS        TARGET               PV       UV    7 DAYS   │  table
│  blog         blog.changkun.de   31,907  26,839  ▁▃▅▇▃▁    │
│  resume       …/resume.pdf       25,182  16,360  ▂▄▁▃▅▂    │
└────────────────────────────────────────────────────────────┘
```

Selecting a row opens the detail: the same charts `003` produces, in a
compact row rather than three stacked 300px blocks.

## Verification

1. The bundle is smaller than 3,389 KB, measured on the built output.
2. `mode=overview` totals equal the same figures queried directly in SQL.
3. A page of twenty rows costs two requests, not twenty-one.
4. Both sites render their own name, links and totals.
5. The public index shows no totals, no sparklines and no edit affordance,
   and discloses no target URL, as `002` established.
6. Creating, editing and deleting a link still work, including validation
   and the date field.

## Testing

- Store tests for the overview and the per-alias series, against a
  fixture with known bot and non-bot rows, asserting bots are excluded.
- A test that the public index response carries neither totals nor series.
- Dashboard tests for the pure functions the charts read.

## Out of Scope

- Changing what any figure means. `003` settled that; this changes only
  how it is shown.
- Light mode. The console is dark; the public page inherits it.
