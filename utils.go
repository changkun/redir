// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"math/rand"
	"sync"
	"time"
)

const alphanum = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var r = newRand()

// randstr generates a random string
func randstr(n int) string {
	var str string
	length := len(alphanum)
	for i := 0; i < n; i++ {
		a := alphanum[r.Intn(len(alphanum))%length]
		str += string(a)
	}
	return str
}

type poolSource struct {
	p *sync.Pool
}

func (s *poolSource) Int63() int64 {
	v := s.p.Get()
	defer s.p.Put(v)
	return v.(rand.Source).Int63()
}

func (s *poolSource) Seed(seed int64) {
	v := s.p.Get()
	defer s.p.Put(v)
	v.(rand.Source).Seed(seed)
}

func (s *poolSource) Uint64() uint64 {
	v := s.p.Get()
	defer s.p.Put(v)
	return v.(rand.Source64).Uint64()
}

func newPoolSource() *poolSource {
	s := &poolSource{}
	p := &sync.Pool{New: func() interface{} {
		return rand.NewSource(time.Now().Unix())
	}}
	s.p = p
	return s
}

func newRand() *rand.Rand {
	return rand.New(newPoolSource())
}
