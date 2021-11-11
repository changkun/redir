
# Troubleshooting

## Private Vanity URL Imports

1. Use `git` instead of `https` protocol
2. Configure `GOPRIVATE` environment variable of the import location, e.g. `changkun.de/x`

```
$ git config --global url."git@github.com:".insteadOf "https://github.com/"
$ echo "export GOPRIVATE=changkun.de/x" >> ~/.zshrc
```

## License

MIT &copy; 2020-2021 [Changkun Ou](https://changkun.de)