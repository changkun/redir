// Copyright 2020 Changkun Ou. All rights reserved.
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
)

var (
	daemon   = flag.Bool("s", false, "run redir service")
	fromfile = flag.String("f", "", "import aliases from a YAML file")
	operate  = flag.String("op", "create", "operators, create/update/delete/fetch")
	alias    = flag.String("a", "", "alias for a new link")
	link     = flag.String("l", "", "actual link for the alias, optional for delete/fetch")
)

func usage() {
	fmt.Fprintf(os.Stderr,
		`usage: redir [-s] [-f <file>] [-op <operator> -a <alias> -l <link>]
options:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
examples:
redir -s                  run the redir service
redir -f ./import.yml     import aliases from a file
redir -a alias -l link    allocate new short link if possible
redir -l link             allocate a random alias for the given link if possible
redir -op fetch -a alias  fetch alias information
`)
	os.Exit(2)
}

func main() {
	log.SetPrefix("redir: ")
	log.SetFlags(log.Lmsgprefix | log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
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
	log.Printf("serving at localhost%s\n", conf.Addr)
	if err := http.ListenAndServe(conf.Addr, nil); err != nil {
		log.Printf("ListenAndServe %s: %v\n", conf.Addr, err)
	}
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
		err := shortCmd(ctx, op(*operate), *alias, *link)
		if err != nil {
			log.Println(err)
		}
		done <- true
	}()

	select {
	case <-ctx.Done():
		log.Fatalf("command timeout!")
		return
	case <-done:
		return
	}
}
