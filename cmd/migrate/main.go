// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Command migrate loads an older redir store into PostgreSQL.
//
// It reads the SQLite file written by the fork that served golang.design
// and writes the rows under a hostname, so that one process can serve both
// sites. The source is opened read-only: it is the fallback if the
// migration is wrong.
//
// Usage:
//
//	migrate -from redir.db -to postgres://... -host golang.design
//
// Running it twice would double every visit count, so a non-empty target
// is refused unless -truncate is given. To close the window between an
// import and moving traffic, run it again with -append and -since set to
// the time the first pass covered.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/migrate"
	"changkun.de/x/redir/internal/migrate/sqlite"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("migrate: ")

	var (
		from     = flag.String("from", "", "SQLite file to read from")
		to       = flag.String("to", "", "PostgreSQL URI to write to")
		host     = flag.String("host", "", "hostname to record for every migrated row")
		only     = flag.String("only", "all", "which tables to copy: all, links or visits")
		truncate = flag.Bool("truncate", false, "replace this host's rows")
		appnd    = flag.Bool("append", false, "add to what is already there, for a second pass")
		since    = flag.String("since", "", "only copy visits after this time, RFC3339")
		dryRun   = flag.Bool("dry-run", false, "read and count, write nothing")
	)
	flag.Parse()

	if *from == "" || *to == "" || *host == "" {
		flag.Usage()
		log.Fatal("-from, -to and -host are required")
	}

	var after time.Time
	if *since != "" {
		t, err := time.Parse(time.RFC3339, *since)
		if err != nil {
			log.Fatalf("-since %q: %v", *since, err)
		}
		after = t.UTC()
	}

	// Ctrl-C during a copy should stop it rather than leave a half
	// written batch behind.
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, os.Kill)
	defer stop()

	src, err := sqlite.Open(*from, after)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()

	links, visits, err := src.Counts(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("reading %v: %d links, %d visits%v",
		*from, links, visits, sinceNote(after))
	log.Printf("writing %v as host %v", db.Redact(*to), *host)

	if err := migrate.Run(ctx, src, *to, migrate.Options{
		Host:     *host,
		Only:     *only,
		Truncate: *truncate,
		Append:   *appnd,
		DryRun:   *dryRun,
	}); err != nil {
		log.Fatal(err)
	}
}

func sinceNote(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return " recorded after " + t.Format(time.RFC3339)
}
