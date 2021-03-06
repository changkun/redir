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
	"changkun.de/x/redir/internal/utils"
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

$ redir [-s] [-f <file>] [-d <file>] [-op <operator> -a <alias> -l <link> -p -vt <time>]

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

redir -op update -a changkun -l https://blog.changkun.de -vt 2022-01-01T00:00:00+08:00
	The alias will be accessible starts from 2022-01-01T00:00:00+08:00.

redir -op delete -a changkun
	Delete the alias from database
`)
	os.Exit(2)
}

func main() {
	log.SetPrefix("redir: ")
	log.SetFlags(log.Lmsgprefix | log.LstdFlags | log.Lshortfile)
	flag.Usage = usage
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
		if *link == "" {
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

	done := make(chan bool, 1)
	go func() {
		defer func() { done <- true }()

		kind := models.KindShort
		if *alias == "" {
			kind = models.KindRandom
			// This might conflict with existing ones, it should be fine
			// at the moment, the user of redir can always the command twice.
			if config.Conf.R.Length <= 0 {
				config.Conf.R.Length = 6
			}
			*alias = utils.Randstr(config.Conf.R.Length)
		}

		t, err := time.Parse(time.RFC3339, *validt)
		if err != nil {
			return
		}

		err = short.Cmd(ctx, short.Op(*operate), &models.Redir{
			Alias:     *alias,
			Kind:      kind,
			URL:       *link,
			Private:   *private,
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
