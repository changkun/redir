// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/short"
	"changkun.de/x/redir/internal/version"
)

var (
	daemon   = flag.Bool("s", false, "Run redir server")
	fromfile = flag.String("f", "", "Import aliases from a YAML file")
	dump     = flag.String("d", "", "Dump aliases from database and export as a YAML file")
	operate  = flag.String("op", "create", "What to do: create, update, delete or fetch")
	alias    = flag.String("a", "", "The alias to act on")
	link     = flag.String("l", "", "Where the alias points. Not needed for delete or fetch")
	private  = flag.Bool("p", false, "Keep the link off the public index. It still works. For create and update")
	trust    = flag.Bool("trust", false, "Redirect directly instead of warning before leaving the site. Same-origin links always redirect")
	validt   = flag.String("vt", "", "Hold the link until this time, in RFC 3339, e.g. 2026-01-02T15:04:05+07:00. For create and update")
	rederive = flag.Bool("rederive", false, "Recompute the browser, os, device, bot and referer host of stored visits, then exit")
)

// runRederive recomputes the columns derived from a visit's user agent
// and referer.
//
// Those columns cache a pure function of ua and referer, so improving the
// function leaves the stored values stale. Without this, visits recorded
// before a rule changed are classified by the old rule and everything
// since by the new one, and a single chart mixes the two.
//
// It covers every configured site. A rule change applies to all of them,
// and re-deriving only the primary one leaves the others reading by the
// old rule with nothing to say so.
func runRederive() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, os.Kill)
	defer cancel()

	hosts := []string{config.Conf.Hostname()}
	for name := range config.Conf.Hosts {
		if h := config.NormalizeHost(name); h != "" && h != hosts[0] {
			hosts = append(hosts, h)
		}
	}

	log.Printf("re-deriving %v in %v", hosts, db.Redact(config.Conf.Store))
	for _, host := range hosts {
		if err := db.Rederive(ctx, config.Conf.Store, host); err != nil {
			log.Fatalf("cannot re-derive visits for %v: %v", host, err)
		}
	}
}

// givenFlags reports which of the link fields were named on the command
// line, so an update can leave the rest alone.
func givenFlags() short.Given {
	var g short.Given
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "l":
			g.URL = true
		case "p":
			g.Private = true
		case "trust":
			g.Trust = true
		case "vt":
			g.ValidFrom = true
		}
	})
	return g
}

func usage() {
	fmt.Fprintf(os.Stderr, `redir is a featured URL shortener. The redir server (run via '-s' option),
will connect to the default database address %s.
It is possible to reconfig redir using an external configuration file.
See https://changkun.de/s/redir for more details.

Version: %s
GoVersion: %s

Command line usage:

$ redir [-s] [-f <file>] [-d <file>] [-op <operator> -a <alias> -l <link> -p -t -vt <time>]

options:
`, db.Redact(config.Conf.Store), version.Version, runtime.Version())
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
examples:
redir -s
	Run the redir server

redir -f ./template/import.yml
	Import aliases from a file

redir -d ./template/export.yml
	Dump all aliases from database and export in YAML format.

redir -a changkun -l https://changkun.de
	Allocate new short link if possible

redir -l https://changkun.de
	Allocate a random alias for the given link if possible

redir -op fetch -a changkun
	Fetch alias information

redir -op update -a changkun -l https://blog.changkun.de -p
	The alias will not be listed in the index page

redir -op update -a changkun -l https://blog.changkun.de -p -t
	The alias will not be listed in the index page and will always do the redirect without showing privacy warning

redir -op update -a changkun -l https://blog.changkun.de -vt 2022-01-01T00:00:00+08:00
	The alias will be accessible starts from 2022-01-01T00:00:00+08:00

redir -op delete -a changkun
	Delete the alias from database
`)
	os.Exit(2)
}

func main() {
	log.SetPrefix("redir: ")
	log.SetFlags(log.Lmsgprefix | log.LstdFlags | log.Lshortfile)
	flag.CommandLine.Usage = usage
	flag.Parse()

	if len(os.Args) < 2 {
		flag.Usage()
		return
	}

	if *rederive {
		runRederive()
		return
	}
	if *daemon {
		runServer()
		return
	}
	runCmd()
}

func runServer() {
	s := newServer(context.Background())
	s.registerHandler()
	log.Printf("serving at %s\n", config.Conf.Addr)
	if err := http.ListenAndServe(config.Conf.Addr, nil); err != nil {
		log.Printf("ListenAndServe %s: %v\n", config.Conf.Addr, err)
	}
	s.close()
}

func runCmd() {
	if *fromfile != "" {
		short.ImportFile(*fromfile)
		return
	}

	if *dump != "" {
		short.DumpFile(*dump)
		return
	}

	if !short.Op(*operate).Valid() {
		flag.Usage()
		return
	}

	switch o := short.Op(*operate); o {
	case short.OpCreate:
		if *alias == "" || *link == "" {
			flag.Usage()
			return
		}
	case short.OpUpdate, short.OpDelete, short.OpFetch:
		if *alias == "" {
			flag.Usage()
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan bool)
	go func() {
		defer func() { done <- true }()
		var t time.Time
		var err error
		if *validt != "" {
			t, err = time.Parse(time.RFC3339, *validt)
			if err != nil {
				log.Fatalf("invalid time format %s: %v", *validt, err)
				return
			}
		}

		// An update applies only the flags that were actually passed.
		// A boolean flag left out is false, which used to be read as
		// "make this public" rather than as silence, so changing a
		// link's URL also unset its visibility and its trust.
		err = short.Cmd(ctx, short.Op(*operate), &models.Redir{
			Alias:     *alias,
			URL:       *link,
			Private:   *private,
			Trust:     *trust,
			ValidFrom: t.UTC(),
		}, givenFlags())
		if err != nil {
			log.Println(err)
		}
	}()

	select {
	case <-ctx.Done():
		log.Fatalf("command timeout!")
		return
	case <-done:
		return
	}
}
