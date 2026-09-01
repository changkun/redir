# Why redir is GDPR compliant?

There are several features are supported for GDPR:

1. It is possible to hide IP addresses from collected access statistics.
   With `gdpr.hide_ip`, a visit stores a digest keyed with
   `REDIR_IP_HASH_KEY` rather than the address, and the access log omits
   the address too. The key is required: an unkeyed digest of an address
   can be reversed by computing all four billion of them, so the server
   refuses to start rather than offer protection it does not provide.
2. Public pages provides imprint (/s/.impressum), privacy (/s/.privacy), and contact (/s/.contact) pages. The content of all these pages can be customized.
3. When shortening a link, it is possible to set if a link is trustable or not. Any external domain links are by default untrusted. A configured trusted link will do direct redirects, and untrusted links will show a warning page to visitors then explicitly ask for permission of redirect.

## License

MIT &copy; 2020-2026 [Changkun Ou](https://changkun.de)