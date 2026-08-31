---
title: Enriched stats
status: planned
depends_on:
  - 002-postgres-store
affects:
  - internal/db/stats.go
  - internal/models
  - dashboard/src/components/Stats.jsx
  - dashboard/package.json
  - short.go
effort: medium
created: 2026-08-31
updated: 2026-08-31
author: changkun
dispatched_task_id: null
---

# Enriched stats

## Overview

`002-postgres-store.md` ports the store without changing a single number,
so that a mismatch during its verification has exactly one possible cause.
It writes the enriched columns but reads none of them. This spec spends
them: the dashboard stops parsing user agents in the browser, bots stop
being counted as visitors, and referrers group by host instead of by full
URL.

Every change here alters displayed numbers on purpose. None of them can
run before `002` is verified.

## Current State

After `002`, `visits` carries `browser`, `os`, `device`, `is_bot` and
`referer_host`, derived at write time and backfilled for all 348,356
historical rows. Nothing queries them.

The dashboard still does the work the columns exist to replace
(`Stats.jsx:9-13, 68-79`):

```js
import UAParser from 'ua-parser-js'
const uaparser = new UAParser()
...
for (const entry of uaData) {
  if (entry.ua.includes('bot') || entry.ua.includes('unknown')) continue
  const r = uaparser.setUA(entry.ua).getResult()
  ...
}
```

This has four defects:

1. `/s/?mode=stats&stat=ua` ships every distinct user agent string to the
   browser, then discards most of them.
2. Bot detection is `ua.includes('bot')`, a substring test that misses
   every crawler not spelling "bot" in its name.
3. Bots are excluded from the browser and device charts but **included** in
   PV, UV, referrer and time series, so the charts on one page disagree.
4. `entry.ua.includes('unknown')` filters the synthetic `unknown` string
   that only exists because the old Mongo pipeline invented it.

`StatReferer` groups by the full referer URL, so one referring page with
query parameters becomes many rows.

## Design

### Server-side grouping

Two stat modes replace the raw `ua` dump:

| Mode | Query |
| --- | --- |
| `browser` | `GROUP BY browser` over non-bot rows |
| `os` | `GROUP BY os` over non-bot rows |

`stat=ua` is kept, returning the raw strings, because it is the only way to
inspect what the parser did with a suspicious entry. It is not what the
charts call.

`StatReferer` groups by `referer_host`, with the empty host reported as
`direct` rather than `unknown`, since an absent referrer is a direct visit
and the dashboard already relabels it that way (`Stats.jsx:156`).

### Bots

`is_bot` is decided at write time. Bot rows stay in the table and are
excluded from every chart, not just two, so the page is internally
consistent. A `bots` mode reports how many were excluded, so the exclusion
is visible instead of silent.

This is the change with the largest visible effect: PV and UV both drop.
The verification below quantifies the drop before it ships, so it is not
mistaken for data loss.

### Dashboard

`ua-parser-js` is removed from `dashboard/package.json`. `Stats.jsx` reads
the grouped responses directly. The `unknown` string handling disappears
from both the queries and the component in the same change.

### UV

`002` establishes that UV must stay `COUNT(DISTINCT ip)` because
`visitor_id` is a session cookie that 80% of visits mint fresh. If the
cookie is given a `Max-Age` and `SameSite`, `visitor_id` becomes a real
identifier, but only for visits recorded after that change: the historical
rows cannot be repaired. Any switch is therefore a new metric alongside the
old one, not a replacement, and is out of scope until the cookie has run
long enough to be worth reading.

## Verification

1. Bot share is quantified per alias before and after:
   `pv_after == pv_before - bot_rows(alias)`, checked for the ten
   highest-PV aliases.
2. `browser` and `os` totals equal what the current client-side parser
   produces for the same range, within the difference explained by bot
   exclusion.
3. Referrer rows collapse: the row count for `referer_host` is lower than
   for `referer`, and their count sums are equal.
4. The stats response for a busy alias is smaller than before, measured in
   bytes.

## Testing

- Store tests for `browser`, `os`, `bots` and host-grouped referrers
  against a fixture with known bot and non-bot rows.
- A UA parsing table test: known crawler strings classify as bots, known
  browser strings do not.
- Dashboard test that the charts render from the grouped payload with no
  parser present.

## Out of Scope

- The golang.design deployment: `004`.
- Persistent visitor cookies, per the reasoning above.
