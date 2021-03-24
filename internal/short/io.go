// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package short

import (
	"context"
	"log"
	"os"
	"time"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/utils"
	"gopkg.in/yaml.v3"
)

type iofmt struct {
	Short  []models.RedirIndex `yaml:"short"`
	Random []models.RedirIndex `yaml:"random"`
}

// ImportFile parses and imports the given file into redir database.
func ImportFile(fname string) {
	b, err := os.ReadFile(fname)
	if err != nil {
		log.Fatalf("cannot read import file: %v\n", err)
	}

	var d iofmt

	err = yaml.Unmarshal(b, &d)
	if err != nil {
		log.Fatalf("cannot unmarshal the imported file: %v\n", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	for _, info := range d.Short {
		r := &models.Redir{
			Alias:     info.Alias,
			URL:       info.URL,
			Kind:      models.KindShort,
			Private:   info.Private,
			ValidFrom: info.ValidFrom,
		}

		err = Cmd(ctx, OpUpdate, r)
		if err != nil {
			err = Cmd(ctx, OpCreate, r)
			if err != nil {
				log.Printf("cannot import alias %v: %v\n", info.Alias, err)
			}
		}
	}
	for _, info := range d.Random {
		// This might conflict with existing ones, it should be fine
		// at the moment, the user of redir can always the command twice.
		if config.Conf.R.Length <= 0 {
			config.Conf.R.Length = 6
		}
		alias := utils.Randstr(config.Conf.R.Length)

		r := &models.Redir{
			Alias:     alias,
			URL:       info.URL,
			Kind:      models.KindRandom,
			Private:   info.Private,
			ValidFrom: info.ValidFrom,
		}
		err = Cmd(ctx, OpUpdate, r)
		if err != nil {
			for i := 0; i < 10; i++ { // try 10x maximum
				err = Cmd(ctx, OpCreate, r)
				if err != nil {
					log.Printf("cannot create alias %v: %v\n", alias, err)
					continue
				}
				break
			}
		}
	}
}

// DumpFile dumps the redir data into a given file in YAML format.
func DumpFile(fname string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	s, err := db.NewStore(ctx, config.Conf.Store)
	if err != nil {
		log.Println("cannot create a new store: %w", err)
		return
	}
	defer s.Close()

	var idxes iofmt

	pageNum := int64(1)
	pageSize := int64(100)
	for {
		idx, _, err := s.FetchAliasAll(ctx, false, models.KindShort, pageSize, pageNum)
		if err != nil {
			log.Printf("cannot fetch aliases, page num: %d, page siz: %d", pageNum, pageSize)
			return
		}
		if len(idx) == 0 {
			break
		}
		idxes.Short = append(idxes.Short, idx...)
		pageNum++
	}

	pageNum = 1
	pageSize = 100
	for {
		idx, _, err := s.FetchAliasAll(ctx, false, models.KindRandom, pageSize, pageNum)
		if err != nil {
			log.Printf("cannot fetch aliases, page num: %d, page siz: %d", pageNum, pageSize)
			return
		}
		if len(idx) == 0 {
			break
		}
		idxes.Random = append(idxes.Random, idx...)
		pageNum++
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
