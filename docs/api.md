# Redir APIs

All possible routers: `/s`, `/r`, and `/x`. The `/s` is the most
complicated router because we are limited to use these prefixes
(for many reasons, e.g. deploy to an existing domain that served a lot
different routers. The prefix is configurable).

Thus, all kinds of data, pages, static files are served under this router.

## GET /s

The GET request query parameters of `/s` and `/r` are listed as follows:

- `mode`, possible options: `stats`, `index`, `index-pro`
  + `admin`, access admin dashboard
  + `index-pro` mode, admin only
    - `ps`, page size
    - `pn`, page number
  + `index` mode
    - `ps`, page size
    - `pn`, page number
  + `stats` mode
    - `a`, alias for stat data
    - `stat`, possible options: `referer`, `ua`, `time`
      - `t0`, start time
      - `t1`, end time

## POST /s

The POST request body of `/s` and `/r` is in the following format:

```json
{
    "op": "create",
    "alias": "awesome-link",
    "data": {
        "alias": "awesome-link",
        "url": "https://github.com/changkun",
        "private": true,
        "valid_from": "2022-01-01T00:00:00+00:00"
    }
}
```

## License

MIT &copy; 2020-2021 [Changkun Ou](https://changkun.de)