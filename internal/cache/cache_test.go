// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package cache

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	"changkun.de/x/redir/internal/models"
)

func TestLRU(t *testing.T) {
	l := NewLRU(false)
	l.cap = 2 // limit the capacity for testing

	if _, ok := l.Get("a"); ok {
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
	l.Put("a", r)
	_, ok := l.Get("a")
	if !ok {
		t.Fatalf("Get value from LRU found nothing")
	}
	if l.Len() != 1 {
		t.Fatalf("wrong size, want 1, got %v", l.Len())
	}

	l.Put("b", &models.Redir{
		Alias:     "b",
		URL:       "2",
		Private:   false,
		ValidFrom: time.Now(),
	})
	v, ok := l.Get("a")
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
	l.Put("c", r)
	_, ok = l.Get("b")
	if ok {
		t.Fatalf("Get value success meaning LRU incorrect")
	}
	v, ok = l.Get("c")
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
	l.Put("a", &models.Redir{
		Alias:     "a",
		URL:       "1",
		Private:   false,
		ValidFrom: tt,
	})
	l.Put("b", &models.Redir{
		Alias:     "b",
		URL:       "2",
		Private:   false,
		ValidFrom: tt,
	})
	l.Put("c", &models.Redir{
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
	l.Put("a", rr)
	v, ok = l.Get("a")
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
	for i := 0; i < 5; i++ {
		ret[i] = alphabet[rand.Intn(len(alphabet))]
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
	l.Put("a", r)
	b.Run("Get", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				l.Get("a")
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
				l.Put(k, v)
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
				l.Put(k, v)
			}
		})
	})
}
