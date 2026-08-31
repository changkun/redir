// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package cache

import (
	"math/rand/v2"
	"reflect"
	"sync"
	"testing"
	"time"

	"changkun.de/x/redir/internal/models"
)

// host is the site every entry in these tests belongs to, except where
// a test deliberately uses a second one.
const host = "changkun.de"

func TestLRU(t *testing.T) {
	l := NewLRU(false)
	l.cap = 2 // limit the capacity for testing

	if _, ok := l.Get(host, "a"); ok {
		t.Fatalf("Get value from empty LRU")
	}
	if l.Len() != 0 {
		t.Fatalf("wrong size, want 0, got %v", l.Len())
	}

	r := &models.Redir{
		Alias:     "a",
		URL:       "1",
		Private:   false,
		ValidFrom: time.Now(),
	}
	l.Put(host, "a", r)
	_, ok := l.Get(host, "a")
	if !ok {
		t.Fatalf("Get value from LRU found nothing")
	}
	if l.Len() != 1 {
		t.Fatalf("wrong size, want 1, got %v", l.Len())
	}

	l.Put(host, "b", &models.Redir{
		Alias:     "b",
		URL:       "2",
		Private:   false,
		ValidFrom: time.Now(),
	})
	v, ok := l.Get(host, "a")
	if !ok { // a -> b
		t.Fatalf("Get value after Put from LRU found nothing")
	}
	if !reflect.DeepEqual(r, v) {
		t.Fatalf("Get value from LRU want %v got %v", r, v)
	}
	if l.Len() != 2 {
		t.Fatalf("wrong size, want 2, got %v", l.Len())
	}

	r = &models.Redir{
		Alias:     "c",
		URL:       "3",
		Private:   false,
		ValidFrom: time.Now(),
	}
	l.Put(host, "c", r)
	_, ok = l.Get(host, "b")
	if ok {
		t.Fatalf("Get value success meaning LRU incorrect")
	}
	v, ok = l.Get(host, "c")
	if !ok {
		t.Fatalf("Get value fail meaning LRU incorrect")
	}
	if !reflect.DeepEqual(v, r) {
		t.Fatalf("Get value from LRU want 3 got %v", v)
	}
	if l.Len() != 2 {
		t.Fatalf("wrong size, want 2, got %v", l.Len())
	}

	l.Flush()
	if l.Len() != 0 {
		t.Fatalf("wrong size, want 0, got %v", l.Len())
	}

	tt := time.Now().UTC()
	l.Put(host, "a", &models.Redir{
		Alias:     "a",
		URL:       "1",
		Private:   false,
		ValidFrom: tt,
	})
	l.Put(host, "b", &models.Redir{
		Alias:     "b",
		URL:       "2",
		Private:   false,
		ValidFrom: tt,
	})
	l.Put(host, "c", &models.Redir{
		Alias:     "c",
		URL:       "3",
		Private:   false,
		ValidFrom: tt,
	})
	rr := &models.Redir{
		Alias:     "a",
		URL:       "2",
		Private:   false,
		ValidFrom: time.Now().UTC(),
	}
	l.Put(host, "a", rr)
	v, ok = l.Get(host, "a")
	if !ok {
		t.Fatalf("Get value from LRU found nothing")
	}
	if !reflect.DeepEqual(rr, v) {
		t.Fatalf("Get value from LRU want %+v got %+v", rr, v)
	}
	if l.Len() != 2 {
		t.Fatalf("wrong size, want 2, got %v", l.Len())
	}
}

func rands() string {
	var alphabet = "qazwsxedcrfvtgbyhnujmikolpQAZWSXEDCRFVTGBYHNUJMIKOLP"
	ret := make([]byte, 5)
	for i := range 5 {
		ret[i] = alphabet[rand.IntN(len(alphabet))]
	}
	return string(ret)
}

func BenchmarkLRU(b *testing.B) {
	l := NewLRU(false)

	r := &models.Redir{
		Alias:     "a",
		URL:       "1",
		Private:   false,
		ValidFrom: time.Now(),
	}
	l.Put(host, "a", r)
	b.Run("Get", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				l.Get(host, "a")
			}
		})
	})
	b.Run("Put-Same", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			// each goroutine put its own k/v
			k := rands()
			v := &models.Redir{
				Alias:     k,
				URL:       rands(),
				Private:   false,
				ValidFrom: time.Now(),
			}
			for pb.Next() {
				l.Put(host, k, v)
			}
		})
	})

	// This is a very naive bench test, especially it
	// mostly measures the rands().
	b.Run("Put-Different", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				k := rands()
				v := &models.Redir{
					Alias:     k,
					URL:       rands(),
					Private:   false,
					ValidFrom: time.Now(),
				}
				// each put has a different k/v
				l.Put(host, k, v)
			}
		})
	})
}

// TestLRUFlush guards against Flush walking the list while removing from
// it. container/list clears the removed element's next pointer, so such a
// walk stops after one element and leaves the rest reachable through Get
// even though Len reports an empty cache.
func TestLRUFlush(t *testing.T) {
	l := NewLRU(false)
	keys := []string{"a", "b", "c"}
	for _, k := range keys {
		l.Put(host, k, &models.Redir{Alias: k, ValidFrom: time.Now()})
	}

	l.Flush()

	for _, k := range keys {
		if _, ok := l.Get(host, k); ok {
			t.Errorf("Get(%q) hits after Flush, want a miss", k)
		}
	}
	if got := l.elems.Len(); got != 0 {
		t.Errorf("list holds %v elements after Flush, want 0", got)
	}
	if got := l.Len(); got != 0 {
		t.Errorf("Len() = %v after Flush, want 0", got)
	}
}

// TestLRUGetConcurrent detects Get mutating the list under a read lock.
// It only reports under -race, which CI runs.
func TestLRUGetConcurrent(t *testing.T) {
	l := NewLRU(false)
	for _, k := range []string{"a", "b"} {
		l.Put(host, k, &models.Redir{Alias: k, ValidFrom: time.Now()})
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				l.Get(host, "a")
				l.Get(host, "b")
			}
		}()
	}
	wg.Wait()
}

// TestHostsDoNotShareEntries is the reason the key is (host, alias).
//
// One process serves several sites, and the same alias means different
// things on each. Keyed by alias alone, whichever site was looked up
// first wins and the other is served someone else's target: a wrong
// redirect that appears nowhere in the logs, because from the server's
// side it is a cache hit like any other.
func TestHostsDoNotShareEntries(t *testing.T) {
	l := NewLRU(true)

	l.Put("changkun.de", "s", &models.Redir{
		Host: "changkun.de", Alias: "s", URL: "https://changkun.de/target",
	})

	if _, ok := l.Get("golang.design", "s"); ok {
		t.Fatal("golang.design/s/s hit the cache entry for changkun.de")
	}

	l.Put("golang.design", "s", &models.Redir{
		Host: "golang.design", Alias: "s", URL: "https://golang.design/target",
	})

	for _, tt := range []struct{ host, want string }{
		{"changkun.de", "https://changkun.de/target"},
		{"golang.design", "https://golang.design/target"},
	} {
		v, ok := l.Get(tt.host, "s")
		if !ok {
			t.Fatalf("%v: alias missing from the cache", tt.host)
		}
		if v.URL != tt.want {
			t.Fatalf("%v: URL = %q, want %q", tt.host, v.URL, tt.want)
		}
	}

	if l.Len() != 2 {
		t.Fatalf("cache holds %d entries, want 2", l.Len())
	}
}

// TestKeyIsUnambiguous checks that two different (host, alias) pairs
// cannot produce the same key by concatenation.
func TestKeyIsUnambiguous(t *testing.T) {
	if key("a", "b/c") == key("a/b", "c") {
		t.Fatal("key collides when the separator appears in a component")
	}
	if key("changkun.de", "") == key("changkun.de", "x") {
		t.Fatal("the index page shares a key with an alias")
	}
}
