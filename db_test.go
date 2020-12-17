// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

const kalias = "alias"

func prepare(ctx context.Context, t *testing.T) *store {
	s, err := newStore("redis://localhost:6379/8")
	if err != nil {
		t.Fatalf("cannot connect to data store")
	}

	err = s.StoreAlias(ctx, kalias, "link", kindShort)
	if err != nil {
		t.Fatalf("cannot store alias to data store, err: %v\n", err)
	}
	t.Cleanup(func() {
		err := s.DeleteAlias(ctx, kalias)
		if err != nil {
			t.Fatalf("DeleteAlias failure, err: %v", err)
		}
		s.Close()
	})
	return s
}

func check(ctx context.Context, t *testing.T, s *store, key string, rr interface{}) {
	var (
		r   string
		err error
	)
	if strings.HasPrefix(key, prefixalias) {
		r, err = s.FetchAlias(ctx, strings.TrimPrefix(key, prefixalias))
	} else {
		r, err = s.FetchIP(ctx, strings.TrimPrefix(key, prefixip))
	}
	if err != nil {
		t.Fatalf("Fetch failure, err: %v\n", err)
	}
	err = json.Unmarshal(str2b(r), rr)
	if err != nil {
		t.Fatalf("Unmarshal failure, err: %v\n", err)
	}
}

func TestUpdateAlias(t *testing.T) {
	want := "link2"

	ctx := context.Background()
	s := prepare(ctx, t)

	err := s.UpdateAlias(ctx, kalias, want)
	if err != nil {
		t.Fatalf("UpdateAlias failed with err: %v", err)
	}

	r := arecord{}
	check(ctx, t, s, prefixalias+kalias, &r)

	if r.URL != want {
		t.Fatalf("Incorrect UpdateAlias implementaiton, want %v, got %v", want, r.URL)
	}
}

func TestUpdateIP(t *testing.T) {
	ctx := context.Background()
	s := prepare(ctx, t)
	ip := "an-ip"
	t.Cleanup(func() {
		s.deleteIP(ctx, ip)
	})

	err := s.StoreIP(ctx, ip, kalias)
	if err != nil {
		t.Fatalf("Cannot store IP for visiting kalias, err: %v", err)
	}

	err = s.UpdateIP(ctx, ip, kalias)
	if err != nil {
		t.Fatalf("Cannot update IP for visiting kalias, err: %v", err)
	}

	r := irecord{}
	check(ctx, t, s, prefixip+ip, &r)

	found := 0
	for a := range r.Aliases {
		if a == kalias {
			found++
			break
		}
	}
	if found > 1 {
		t.Fatal("Incorrect StoreIP/UpdateIP implementation, found duplicated alias")
	}

	rr := arecord{}
	check(ctx, t, s, prefixalias+kalias, &rr)
	if rr.PV != 2 || rr.UV != 1 {
		t.Fatalf("Incorrect PV/UV implementation, got %v/%v", rr.PV, rr.UV)
	}
}

func TestAtomicUpdate(t *testing.T) {
	ctx := context.Background()
	s := prepare(ctx, t)

	// create a number of concurrent updater
	// check data is still consistent
	concurrent := 1000
	wg := sync.WaitGroup{}
	wg.Add(concurrent)
	for i := 0; i < concurrent; i++ {
		go func() {
			defer wg.Done()
			err := s.countVisit(ctx, "alias", 1, 1)
			if err != nil {
				t.Errorf("countVisit failure, err: %v\n", err)
				return
			}
		}()
	}
	wg.Wait()

	r := arecord{}
	check(ctx, t, s, prefixalias+kalias, &r)
	if r.PV != uint64(concurrent) || r.UV != uint64(concurrent) {
		t.Fatalf("Incorrect atomic readAndUpdate implementaiton: pv:%v, uv:%v\n", r.PV, r.UV)
	}
}
