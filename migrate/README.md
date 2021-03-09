# Migrate from early version of redir

Caution: Because of the limitation of early version of `redir`, i.e.
it did not record visit pattern per-click. Therefore, after the migration,
the PV will be losted and will receive the same number of UV.

## Step 1: Dump Data from Redis

```sh
cd dump && mkdir ips alias # now at redir/migrate/dump
go run redis-dump.go       # require Go 1.16
```

Note: change `redis://0.0.0.0:6379/9` to the target address.

## Step 2: Restore Data to MongoDB

Get mongodb instance running, then restore the data via:

```sh
cd ..                   # now at redir/migrate
go run mongo-restore.go # require Go 1.16
```

Note: change `mongodb://0.0.0.0:27017` to the target address.

## License

MIT &copy; 2021 [Changkun Ou](https://changkun.de)