# redir [![PkgGoDev](https://pkg.go.dev/badge/changkun.de/x/redir)](https://pkg.go.dev/changkun.de/x/redir) ![](https://changkun.de/urlstat?mode=github&repo=changkun/redir)

The request redirector for changkun.de.

![](./demo.png)

## CLI Usage

```
redir -a changkun -l https://changkun.de
redir -l https://changkun.de
redir -f import.yml
redir -op fetch -a changkun
redir -op update -a changkun https://blog.changkun.de
redir -op delete -a changkun
```

## Troubleshooting

```
git config --global url."git@github.com:".insteadOf "https://github.com/"
echo "export GOPRIVATE=changkun.de/x" >> ~/.zshrc
```

## License

MIT &copy; 2020-2021 [Changkun Ou](https://changkun.de)