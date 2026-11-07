---
title: Sites choose their services, and serve import paths of their own
status: complete
depends_on:
  - 004-unify-golang-design
affects:
  - internal/config
  - server.go
  - auth_latere.go
  - templates/x.html
effort: small
created: 2026-11-07
updated: 2026-11-07
author: changkun
dispatched_task_id: null
---

# Sites choose their services, and serve import paths of their own

## Overview

poly.red is a Go module path: `polyred`'s `go.mod` says `module poly.red`.
A container of its own answers for it, the static server in
changkun/infra run once more with three environment variables, so that
`go get poly.red` finds the `go-import` tag. Serving it is what redir does
for `changkun.de/x/` and `golang.design/x/`, but redir assumes every site
offers short links under `/s/` and a tree of repositories under `/x/`.
golang.design does only by coincidence. This spec lets a site choose its
services and list import paths of its own, and moves poly.red onto redir.

## Current State

- Routing is by path alone: `/s/` and `/x/` are registered on the default
  mux and answer for every Host header.
- A site's repositories are `<host>/x/<repo>`, resolved to
  `repo_path/<repo>`. Nothing serves an import path outside that tree.
- The poly.red container answers `?go-get=1` at any path with
  `go-import` and `go-source` tags for github.com/changkun/polyred, and
  sends any other request to the repository with a 301. Its log showed 112
  `go-get` requests between Oct 7 and Oct 23, 2026, the latest from
  Google, which is how the module proxy refreshes the module.

## Decisions

### A site turns off what it does not offer

Adding poly.red to `hosts` as things were would hand it the shortener:
`poly.red/s/foo` would open a namespace of aliases for poly.red, and on a
miss `checkvcs` would probe `github.com/changkun/foo` and store a link. 004
names that probe the sharpest edge of serving several sites.

`SiteOverride` gains `links` and `repos`. Unset, a site offers both, so
every existing configuration keeps what it served. The prefixes stay
shared: the login flow lives under `s.prefix` and its callbacks are
registered per site, and no site needs a different prefix yet.

### Import paths a site lists

`imports` maps an import path of the site to the repository that holds
it. The path and every package below it answer with that repository.
The longest listed path wins, so a module can be nested in another. An
import path that does not start with the site's hostname, ends with a
slash, or has no repository URL stops the server at start up, since it
would otherwise never be served or would answer `go get` with nothing.

Imports work with `x.enable` false: they are the site's own, not part of
the `/x/` tree.

### One handler routes by site

The two registrations become one handler that resolves the site from
the Host header first, as everything host-derived does, so an unknown
header still falls back to the primary site. Go 1.22 host patterns would
skip that fallback.

1. A site's import paths, so `poly.red/s/foo?go-get=1` answers for the
   module, as the static server did.
2. `s.prefix`, if the site offers links.
3. `x.prefix`, if it offers repositories.
4. Anything else is a 404.

An inner mux keeps the redirect from `/s` to `/s/`, which traefik routes.

### What a browser gets

For `go get`, an import path answers with the `go-import` document `/x/`
uses. A browser is redirected to the import's `web` page if one is set,
and otherwise to the package's documentation on `godoc_host`. poly.red
sets `web` to the repository, which keeps what it did. The redirect is a
302 rather than the 301 it was, so that a later change of `web` is not
held in browser caches.

The `go-source` tag is dropped. pkg.go.dev derives source links from a
GitHub repository URL without it, and nothing else reads it now that
godoc.org is gone.

The document's refresh now uses `godoc_host`, as the redirect at
`/x/` itself already did, rather than a hardcoded pkg.go.dev. The
deployed value is pkg.go.dev, so nothing served changes.

### No login on a site without links

`callbackURLs` built a relying party for every configured site. A site
without short links has no console, so it gets none, and its callback
need not be registered with the authorization server.

## Cutover

The order matters. With traefik sending poly.red to redir before redir
knows the site, `SiteFor` resolves poly.red to changkun.de and the
domain serves changkun.de's short links.

1. Deploy redir, then add the `poly.red` entry to `hosts` in
   `data/redirconf.yml` and restart it.
2. From inside the proxy network, before traefik changes, check what
   redir answers for poly.red: `?go-get=1` at the root and at a package,
   and a browser request's `Location`.
3. In changkun/infra, send `to-polyred-vanity` to `redir` and drop the
   `polyred-vanity` service. traefik reads the change as the file is
   pulled.
4. From outside, `curl 'https://poly.red/?go-get=1'`, and
   `GOPROXY=direct go list -m poly.red@latest`, which follows the real
   `go get` path.
5. Then remove the container and its service from
   `infra/static/docker-compose.yml`.

## Verification

1. The tests pin that the configured sites resolve as before, that poly.red
   answers `go-get` at any path and redirects a browser, that a site
   without links answers 404 below `/s/` and stores nothing, and that it
   gets no login.
2. changkun.de/x/ and golang.design/x/ answer as before the change.
3. `go list -m poly.red@latest` resolves through poly.red, not the proxy.

## Outcome

Completed 2026-11-07. poly.red is served by redir, as a site with
neither short links nor `/x/`, and the container that served only it is
removed, along with the static server's vanity mode in changkun/infra.

### Verified

| Check | Result |
| --- | --- |
| Before the switch, redir against the old container, both asked directly | identical `go-import` at `/` and `/geometry/primitive`; the same GitHub page for a browser, 302 where it was 301 |
| `poly.red/s/foo`, `/x/foo` | the module, as before, not the shortener |
| Login | set up for 2 sites; poly.red has none |
| changkun.de and golang.design through redir | `/x/` names the same repositories; `/s/` serves |
| After the switch, through traefik | the same `go-import`; every poly.red request reached redir |
| `GOPROXY=direct go list -m poly.red@latest` | `v0.0.1` from github.com/changkun/polyred |
| `poly.red/study` | still the study page, on its longer rule |
