# redir APIs

Everything is served under two prefixes, `/s` and `/x`, both configurable.
`/s` carries the redirects, the pages and the JSON the console reads,
because a deployment often shares a domain with something else and cannot
claim more of it.

Responses are scoped to the host the request arrived on. One process
serves several sites, and each has its own aliases and its own
statistics, so the same path answers differently on each.

## GET /s/{alias}

Redirects, or serves a page instead when the link is not ready:

| Situation | Response |
| --- | --- |
| The link is valid | `307` to its target |
| `valid_from` is in the future | a countdown page |
| The target is external and the link is not trusted | a warning page |
| The target ends in `.pdf` | the document, served rather than redirected |

## GET /s

Without `mode`, serves the public index.

### `mode=admin`

The console. Requires a session; see [Login](../README.md#login).

### `mode=index` and `mode=index-pro`

A page of links. `index` is public and `index-pro` requires a session.

| Parameter | Meaning |
| --- | --- |
| `ps` | page size |
| `pn` | page number, from 1 |

```jsonc
{
  "data": [
    {
      "alias": "blog",
      "url": "https://blog.changkun.de/",  // empty in the public listing
      "private": false,
      "trust": true,
      "valid_from": "0001-01-01T00:00:00Z", // the zero time means always
      "created_by": "changkun",
      "updated_by": "changkun",
      "pv": 31907,                          // zero in the public listing
      "uv": 26839
    }
  ],
  "page": 1,
  "total": 184,
  // Only in index-pro: fourteen days of non-bot page views per alias, one
  // value per day. It travels with the page because a sparkline on every
  // row cannot be one request per row.
  "series": { "blog": [3, 7, 12, 9, 4, 6, 8, 2, 0, 5, 9, 11, 6, 4] }
}
```

The public listing discloses neither the target URL nor the counts.

### `mode=overview`

Totals for one site. Requires a session.

| Parameter | Meaning |
| --- | --- |
| `t0`, `t1` | range, `YYYY-MM-DD`; defaults to the last seven days |

```jsonc
{
  "links": 184,
  "visits": 348465,   // everything recorded, so it reconciles with the store
  "people": 151006,   // visits + bots split the total
  "bots": 197459,
  "series": [{ "day": "2026-08-30", "pv": 812, "uv": 403 }]
}
```

### `mode=stats`

One alias, one breakdown.

| Parameter | Meaning |
| --- | --- |
| `a` | the alias |
| `stat` | `time`, `referer`, `browser`, `os`, `device`, `bots`, `ua` |
| `t0`, `t1` | range, `YYYY-MM-DD`; defaults to the last seven days |

**Automated traffic is excluded from every mode except `bots` and `ua`.**
Roughly half of all recorded visits are crawlers, so counting them would
report machines as readers.

| `stat` | Response |
| --- | --- |
| `time` | `[{"time":"2026-08-30T14:00:00Z","pv":12,"uv":7}]`, bucketed by hour in UTC |
| `referer` | `[{"name":"news.ycombinator.com","count":81}]`, grouped by host; an absent referrer is `Direct` |
| `browser`, `os`, `device` | `[{"name":"Chrome","count":1519}]`; an unidentified value is `Unknown` |
| `bots` | `{"pv":653,"uv":417}`, what the other modes leave out |
| `ua` | `[{"ua":"Mozilla/5.0 …","count":12}]`, raw strings including bots |

`ua` exists to inspect a suspicious entry in the grouped modes. It is the
only mode that returns unaggregated strings, and nothing on the console
draws it.

## POST /s

Creates, updates and deletes links. Requires a session.

```jsonc
{
  "op": "create",           // create, update, delete, fetch
  "alias": "awesome-link",  // which link to act on; for update it is the
                            // existing alias, and data.alias may rename it
  "data": {
    "alias": "awesome-link",
    "url": "https://github.com/changkun",
    "private": false,       // hidden from the public index
    "trust": true,          // false shows a warning before leaving the site
    "valid_from": "2026-01-01T00:00:00+00:00" // omit for always
  }
}
```

A refusal answers with a status and `{"message": "..."}` explaining it.

## GET /x/{path}

Serves a `go-import` meta tag so `go get` resolves `<host>/x/<repo>` to
the VCS, and redirects a browser to `pkg.go.dev`. The organisation comes
from the site the request arrived on, so each site resolves to its own.

## License

MIT &copy; 2020-2026 [Changkun Ou](https://changkun.de)
