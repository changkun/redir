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
	"time"

	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/utils"
)

var (
	daemon   = flag.Bool("s", false, "Run redir server")
	fromfile = flag.String("f", "", "Import aliases from a YAML file")
	operate  = flag.String("op", "create", "Operators, create/update/delete/fetch")
	alias    = flag.String("a", "", "Alias for a new link")
	link     = flag.String("l", "", "Actual link for the alias, optional for delete/fetch")
	private  = flag.Bool("p", false, "The link is private and will not be listed in the index page, avaliable for operator create/update")
	validt   = flag.String("vt", "", "the alias will start working from the specified time, format in RFC3339, e.g. 2006-01-02T15:04:05+07:00. Avaliable for operator create/update")
)

func usage() {
	fmt.Fprintf(os.Stderr,
		`usage: redir [-s] [-f <file>] [-op <operator> -a <alias> -l <link> -p -vt <time>]
options:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
examples:
redir -s
	Run the redir server

redir -f ./import.yml
	Import aliases from a file

redir -a changkun -l https://changkun.de
	Allocate new short link if possible

redir -l https://changkun.de
	Allocate a random alias for the given link if possible

redir -op fetch -a changkun
	Fetch alias information

redir -op update -a changkun -l https://blog.changkun.de -p
	The alias will not be listed in the index page
	$ redir -op update -a changkun -l https://blog.changkun.de -p

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
	log.Printf("serving at %s\n", conf.Addr)
	if err := http.ListenAndServe(conf.Addr, nil); err != nil {
		log.Printf("ListenAndServe %s: %v\n", conf.Addr, err)
	}
	s.close()
}

func runCmd() {
	if *fromfile != "" {
		importFile(*fromfile)
		return
	}

	if !op(*operate).valid() {
		flag.Usage()
		return
	}

	switch o := op(*operate); o {
	case opCreate:
		if *link == "" {
			flag.Usage()
			return
		}
	case opUpdate, opDelete, opFetch:
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
			if conf.R.Length <= 0 {
				conf.R.Length = 6
			}
			*alias = utils.Randstr(conf.R.Length)
		}

		t, err := time.Parse(time.RFC3339, *validt)
		if err != nil {
			return
		}

		err = shortCmd(ctx, op(*operate), &models.Redir{
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
