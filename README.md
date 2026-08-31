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
|**Visitor Analysis**| Statistics visualization regarding PV, UV, Referrer, Devices, Location, etc |
|**GDPR Compliant**| Including imprint, privacy, contact pages; optional warning about external redirects, etc. |

## Documentations

- [Redir APIs](./docs/api.md)
- [Current Users](./docs/users.md)
- [Troubleshoting](./docs/trouble.md)
- [GDPR requirements](./docs/gdpr.md)

## Web Interfaces

There are three major pages available in redir.

| Admin Dashboard | Access Control | Public Indexes |
|:---------------:|:--------------:|:--------------:|
| Router: `/s?mode=admin` for management:<br/>![](./.github/assets/admin.png) | Control a link should only be available after a certain time:<br/>![](./.github/assets/wait.png) | Router `/s` provides public accessibility to see all public links:<br/>![](./.github/assets/index.png) |

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
and whose `allowed_origins` include `<host>`. Configure it with the
environment variables in [.env.template](./.env.template); copy that file to
`.env` and fill it in.

Signing in proves who a visitor is, not that they may manage your links, so
`AUTH_ALLOWED_PRINCIPALS` decides separately. It is a comma separated list
of emails or principal ids, matched case insensitively. An empty list
rejects every login.

The dashboard then offers Logout at `/s/.auth/logout`, which ends the
session.

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

```
$ docker network create traefik_proxy
$ make build && make up
```

## Contributing

Easiest way to contribute is to provide feedback! We would love to hear
what you like and what you think is missing. PRs are also welcome.

## License

MIT &copy; 2020-2021 [Changkun Ou](https://changkun.de)