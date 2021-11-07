// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package cache

import (
	"container/list"
	"sync"
	"time"

	"changkun.de/x/redir/internal/models"
)

type item struct {
	k string
	v interface{}
}

// LRU is a naive thread-safe LRU cache
type LRU struct {
	cap   uint
	size  uint
	elems *list.List // of redirect

	mu sync.RWMutex
}

func NewLRU(doexpire bool) *LRU {
	l := &LRU{
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
func (l *LRU) clear() {
	t := time.NewTicker(5 * time.Minute)
	for range t.C {
		l.Flush()
	}
}

func (l *LRU) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for e := l.elems.Front(); e != nil; e = e.Next() {
		l.elems.Remove(e)
	}
	l.size = 0
}

func (l *LRU) Len() uint {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.size
}

func (l *LRU) Get(k string) (*models.Redir, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for e := l.elems.Front(); e != nil; e = e.Next() {
		if e.Value.(*item).k == k {
			l.elems.MoveToFront(e)
			return e.Value.(*item).v.(*models.Redir), true
		}
	}
	return nil, false
}

func (l *LRU) Put(k string, v *models.Redir) {
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
