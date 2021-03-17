// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"testing"
)

const kalias = "alias"

func prepare(ctx context.Context, t *testing.T) *database {
	s, err := newDB("mongodb://0.0.0.0:27017")
	if err != nil {
		t.Fatalf("cannot connect to data store")
	}

	err = s.StoreAlias(ctx, &redirect{
		Alias:   kalias,
		Kind:    kindShort,
		URL:     "link",
		Private: false,
	})
	if err != nil {
		t.Fatalf("cannot store alias to data store: %v\n", err)
	}
	t.Cleanup(func() {
		err := s.DeleteAlias(ctx, kalias)
		if err != nil {
			t.Fatalf("DeleteAlias failure: %v", err)
		}
		s.Close()
	})
	return s
}

func TestUpdateAlias(t *testing.T) {
	want := "link2"

	ctx := context.Background()
	s := prepare(ctx, t)

	err := s.UpdateAlias(ctx, &redirect{
		Alias: kalias,
		URL:   want,
	})
	if err != nil {
		t.Fatalf("UpdateAlias failed with err: %v", err)
	}

	r, err := s.FetchAlias(ctx, kalias)
	if err != nil {
		t.Fatalf("UpdateAlias failed with err: %v", err)
	}
	if r.URL != want {
		t.Fatalf("Incorrect UpdateAlias implementaiton, want %v, got %v", want, r.URL)
	}
}
