// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"log"
	"os"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/utils"
	"gopkg.in/yaml.v3"
)

func importFile(fname string) {
	b, err := os.ReadFile(fname)
	if err != nil {
		log.Fatalf("cannot read import file: %v\n", err)
	}

	var d struct {
		Short map[string]struct {
			URL       string `yaml:"url"`
			Private   bool   `yaml:"private"`
			ValidFrom string `yaml:"valid_from"`
		} `yaml:"short"`
		Random []struct {
			URL       string `yaml:"url"`
			Private   bool   `yaml:"private"`
			ValidFrom string `yaml:"valid_from"`
		} `yaml:"random"`
	}
	err = yaml.Unmarshal(b, &d)
	if err != nil {
		log.Fatalf("cannot unmarshal the imported file: %v\n", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	for alias, info := range d.Short {
		var t time.Time
		if info.ValidFrom != "" {
			t, err = time.Parse(time.RFC3339, info.ValidFrom)
			if err != nil {
				log.Fatalf("incorrect time format, expect RFC3339, but: %v, err: %v", info.ValidFrom, err)
			}
		} else {
			t = time.Now().UTC()
		}

		r := &models.Redir{
			Alias:     alias,
			URL:       info.URL,
			Kind:      models.KindShort,
			Private:   info.Private,
			ValidFrom: t,
		}

		err = shortCmd(ctx, opUpdate, r)
		if err != nil {
			err = shortCmd(ctx, opCreate, r)
			if err != nil {
				log.Printf("cannot import alias %v: %v\n", alias, err)
			}
		}
	}
	for _, info := range d.Random {

		t, err := time.Parse(time.RFC3339, info.ValidFrom)
		if err != nil {
			log.Fatalf("incorrect time format, expect RFC3339, but: %v, err: %v", info.ValidFrom, err)
		}
		// This might conflict with existing ones, it should be fine
		// at the moment, the user of redir can always the command twice.
		if conf.R.Length <= 0 {
			conf.R.Length = 6
		}
		alias := utils.Randstr(conf.R.Length)

		r := &models.Redir{
			Alias:     alias,
			URL:       info.URL,
			Kind:      models.KindRandom,
			Private:   info.Private,
			ValidFrom: t,
		}
		err = shortCmd(ctx, opUpdate, r)
		if err != nil {
			for i := 0; i < 10; i++ { // try 10x maximum
				err = shortCmd(ctx, opCreate, r)
				if err != nil {
					log.Printf("cannot create alias %v: %v\n", alias, err)
					continue
				}
				break
			}
		}
	}
}

func dumpFile(fname string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	s, err := db.NewStore(conf.Store)
	if err != nil {
		log.Println("cannot create a new store: %w", err)
		return
	}
	defer s.Close()

	idxes := []models.RedirIndex{}

	for _, k := range []models.AliasKind{models.KindShort, models.KindRandom} {
		pageNum := 1
		pageSize := 100
		for {
			idx, _, err := s.FetchAliasAll(ctx, false, k, int64(pageSize), int64(pageNum))
			if err != nil {
				log.Printf("cannot fetch aliases, page num: %d, page siz: %d", pageNum, pageSize)
				return
			}
			if len(idx) == 0 {
				break
			}
			idxes = append(idxes, idx...)
			pageNum++
		}
	}

	b, err := yaml.Marshal(idxes)
	if err != nil {
		log.Printf("cannot marshal aliases into yaml format: %v", err)
		return
	}

	err = os.WriteFile(fname, b, os.ModePerm)
	if err != nil {
		log.Printf("cannot write to file %s: %v", fname, err)
		return
	}
	log.Println("Done.")
}
