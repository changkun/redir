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
	v any
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

	// Init rather than a walk with Remove: Remove clears the element's
	// next pointer, so walking and removing together stops after the
	// first element and leaves the rest reachable through Get.
	l.elems.Init()
	l.size = 0
}

func (l *LRU) Len() uint {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.size
}

// key identifies a cached redirect.
//
// A link is (host, alias), not alias: one process serves several sites
// and the same alias means different things on each. Caching by alias
// alone would serve whichever site's target was looked up first, and the
// wrong redirect would not appear in any log.
func key(host, alias string) string {
	return host + "\x00" + alias
}

// Get returns the cached redirect for an alias on a host and promotes it
// to the front. The promotion writes to the list, so this takes the write
// lock.
func (l *LRU) Get(host, alias string) (*models.Redir, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	k := key(host, alias)
	for e := l.elems.Front(); e != nil; e = e.Next() {
		if e.Value.(*item).k == k {
			l.elems.MoveToFront(e)
			return e.Value.(*item).v.(*models.Redir), true
		}
	}
	return nil, false
}

func (l *LRU) Put(host, alias string, v *models.Redir) {
	l.mu.Lock()
	defer l.mu.Unlock()

	k := key(host, alias)

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
