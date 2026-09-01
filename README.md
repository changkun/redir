# redir [![Latest relsease](https://img.shields.io/github/v/tag/changkun/redir?label=latest)](https://github.com/changkun/redir/releases) ![](https://changkun.de/urlstat?mode=github&repo=changkun/redir)

Full-featured, self-hosted URL shortener written in Go.

| Features | Description |
|:-------:|:------------|
|**Custom Domain**| Everything is under control with your own domain |
|**Link Shortener**| Support `/s/semantic-name` for short semantic alias for anonymous shortening |
|**Go [Vanity Import](https://golang.org/cmd/go/#hdr-Remote_import_paths)**|Redirect `/x/repo-name` to configured VCS and `pkg.go.dev` for API documentation|
|**Access Control**| 1) Private links won't be listed in public index page; 2) Allow link to be accessible only after a configured time point; 3) Allow warn to visitors about external URL redirects (for liability control)|
|**Public Indexes**| Router `/s` provides a list of avaliable short links |
|**Admin Dashboard**| Dashboard `/s?mode=admin` provides full management ability |
|**Multiple Sites**| One process serves several domains, each with its own links and its own statistics |
|**Visitor Analysis**| PV/UV over time, referring hosts, browsers, systems and devices, with automated traffic classified and excluded |
|**GDPR Compliant**| Including imprint, privacy, contact pages; optional warning about external redirects, etc. |

## Documentations

- [Redir APIs](./docs/api.md)
- [Current Users](./docs/users.md)
- [Troubleshoting](./docs/trouble.md)
- [GDPR requirements](./docs/gdpr.md)

## Web Interfaces

### Operator console

`/s?mode=admin`. Totals first, the links second, one link's detail third.
Automated traffic is classified where a visit is recorded and left out of
every figure, with a count saying how much was left out.

![The redir operator console](./.github/assets/admin.png)

### Public index

`/s`. The links a visitor may follow. It carries no counts and does not
disclose where a link goes, so the index cannot be used to enumerate
targets.

![The public index](./.github/assets/index.png)

### Access control

A link can be held until a time, and a link leaving the site can warn
first.

| Not yet valid | Leaving the site |
|:---:|:---:|
| ![A link that is not available yet](./.github/assets/wait.png) | ![The warning shown before an external redirect](./.github/assets/warn.png) |

## CLI Usage

The `redir` command offers server side operation feature from shell:

```
$ redir

redir is a featured URL shortener. The redir server (run via '-s' option),
will connect to the default database address postgres://redir:xxxxx@localhost:5432/redir?sslmode=disable.
It is possible to reconfig redir using an external configuration file.
See https://changkun.de/s/redir for more details.

Version: dev

GoVersion: go1.27.0

Command line usage:

$ redir [-s] [-f <file>] [-d <file>] [-op <operator> -a <alias> -l <link> -p -trust -vt <time>]

...
```

## Customization

You can configure redir using a configuration file.
The [default configuration](./internal/config/config.yml) is embedded into the binary.

Alternative configuration can be used to replace default config and specified in environtment variable `REDIR_CONF`, for example `REDIR_CONF=/path/to/config.yml redir -s` to run the redir server under given configuration. When `REDIR_CONF` names a file that cannot be read, the server stops rather than falling back to the default, so a deployment cannot come up quietly pointed at the wrong database.

A deployment keeps its own configuration in `data/redirconf.yml`, which is
not tracked and is mounted into the container rather than built into the
image, because it holds credentials. Start one by copying the default:

```sh
$ mkdir -p data && cp internal/config/config.yml data/redirconf.yml
```

## Login

The admin dashboard supports three modes, set by `auth.enable` in the
configuration file:

| Mode | Who can administer |
|---|---|
| `latere` | Anyone signing in through [auth.latere.ai](https://auth.latere.ai) who is on the allowlist |
| `basic`  | The username and password pairs listed under `auth.basic` |
| `none`   | Everyone. Only sensible behind another access control |

`latere` runs the OAuth 2.0 authorization code flow with PKCE and keeps the
result in an encrypted session cookie. It needs an OAuth client registered
for your deployment, whose `redirect_uris` include `<host>/s/.auth/callback`
and whose `allowed_origins` include `<host>`. Every site gets its own
callback, so a deployment serving two domains registers both. Configure it with the
environment variables in [.env.template](./.env.template); copy that file to
`.env` and fill it in.

Signing in proves who a visitor is, not that they may manage your links, so
`AUTH_ALLOWED_PRINCIPALS` decides separately. It is a comma separated list
of emails or principal ids, matched case insensitively. An empty list
rejects every login.

The dashboard then offers Logout at `/s/.auth/logout`, which ends the
session.

## Storage

redir stores its links and visits in PostgreSQL. Create a database and a
role for it, and point `store` at them:

```sql
CREATE ROLE redir LOGIN PASSWORD '...';
CREATE DATABASE redir OWNER redir;
```

```yaml
store: postgres://redir:...@postgres:5432/redir?sslmode=disable
```

The schema is applied at start up from migrations embedded in the binary,
so nothing else has to be run first. Each start applies what is missing
and nothing more.

Versions up to `v0.7.0` also read MongoDB. `v0.7.0` is the last release
that speaks both, and is the one to use if data has to be moved out of it.

## Serving several sites

One process can serve several domains. The store keys links by hostname,
so each site has its own namespace of aliases and its own statistics, and
which site a request belongs to is decided by the `Host` header rather
than by configuration.

Only what differs needs configuring:

```yaml
host: https://changkun.de
x:
  repo_path: https://github.com/changkun
hosts:
  golang.design:
    # /x/ import paths and the VCS probe resolve here instead.
    repo_path: https://github.com/golang-design
    # The impressum and privacy pages name an operator, so they do not
    # follow onto another domain unless it is the same one.
    legal: false
```

A `Host` header naming no configured site falls back to the primary one.
It is client-controlled and reaches the store as part of a link's
identity, so it is matched rather than trusted.

## Deployment

### Download Pre-Builds

Please check the [release](https://github.com/changkun/redir/releases) page.

### Build from Source

You need [Go](https://golang.org) 1.27 or later to build the `redir`
command, and [Node.js](https://nodejs.org) 22 or later to build the
dashboard that gets embedded into it.

Build everything into a single native binary:

```sh
$ make dashboard # build front-end
$ make           # build back-end and embed front-end files into binary

$ redir -s # run the server, require an external database
```

Build and deploy with Docker. The image builds the dashboard and the
server inside itself, so a deploy host needs only Docker:

```sh
$ docker network create traefik_proxy
$ make build && make up
```

The container reads `data/redirconf.yml` from the host and needs to reach
PostgreSQL, which for a shared instance means joining its network as well.

## Contributing

Easiest way to contribute is to provide feedback! We would love to hear
what you like and what you think is missing. PRs are also welcome.

## License

MIT &copy; 2020-2026 [Changkun Ou](https://changkun.de)