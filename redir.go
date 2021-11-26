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
	"runtime"
	"time"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/short"
	"changkun.de/x/redir/internal/version"
)

var (
	daemon   = flag.Bool("s", false, "Run redir server")
	fromfile = flag.String("f", "", "Import aliases from a YAML file")
	dump     = flag.String("d", "", "Dump aliases from database and export as a YAML file")
	operate  = flag.String("op", "create", "Operators, create/update/delete/fetch")
	alias    = flag.String("a", "", "Alias for a new link")
	link     = flag.String("l", "", "Actual link for the alias, optional for delete/fetch")
	private  = flag.Bool("p", false, "The link is private and will not be listed in the index page, avaliable for operator create/update")
	trust    = flag.Bool("trust", false, "The link is either trusted to not show privacy warning page or untrusted to show privacy warning page for external redirects")
	validt   = flag.String("vt", "", "the alias will start working from the specified time, format in RFC3339, e.g. 2006-01-02T15:04:05+07:00. Avaliable for operator create/update")
)

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
`, config.Conf.Store, version.Version, runtime.Version())
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

		// FIXME: This can be problematic for update.
		//
		// For example, if a link is set to private, but an update
		// did's specify -p (private), then the *private will be false,
		// then result the link to be public. This is apparently an
		// undesired behavior.
		err = short.Cmd(ctx, short.Op(*operate), &models.Redir{
			Alias:     *alias,
			URL:       *link,
			Private:   *private,
			Trust:     *trust,
			ValidFrom: t.UTC(),
		})
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
