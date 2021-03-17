// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"container/list"
	"sync"
	"time"
)

type item struct {
	k string
	v interface{}
}

// lru is a naive thread-safe lru cache
type lru struct {
	cap   uint
	size  uint
	elems *list.List // of redirect

	mu sync.RWMutex
}

func newLRU(doexpire bool) *lru {
	l := &lru{
		cap:   32, // could do it with memory quota
		size:  0,
		elems: list.New(),
		mu:    sync.RWMutex{},
	}
	if doexpire {
		go l.clear()
	}
	return l
}

// clear clears the lru after a while, this is just a dirty
// solution to prevent if the database is updated but lru is
// not synced.
func (l *lru) clear() {
	t := time.NewTicker(time.Minute * 5)
	for range t.C {
		l.flush()
	}
}

func (l *lru) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for e := l.elems.Front(); e != nil; e = e.Next() {
		l.elems.Remove(e)
	}
	l.size = 0
}

func (l *lru) Len() uint {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.size
}

func (l *lru) Get(k string) (*redirect, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for e := l.elems.Front(); e != nil; e = e.Next() {
		if e.Value.(*item).k == k {
			l.elems.MoveToFront(e)
			return e.Value.(*item).v.(*redirect), true
		}
	}
	return nil, false
}

func (l *lru) Put(k string, v *redirect) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// found from cache
	i := &item{k: k, v: v}
	for e := l.elems.Front(); e != nil; e = e.Next() {
		if e.Value.(*item).k == k {
			l.elems.Remove(e)
			l.elems.PushFront(i)
			return
		}
	}

	// check if cache is full
	l.elems.PushFront(i)
	if l.size+1 > l.cap {
		l.elems.Remove(l.elems.Back())
	} else {
		l.size++
	}
}
