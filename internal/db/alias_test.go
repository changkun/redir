// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db_test

import (
	"context"
	"encoding/json"
	"testing"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
)

const kalias = "alias"

func prepare(ctx context.Context, t *testing.T) *db.Store {
	s, err := db.NewStore(ctx, "mongodb://0.0.0.0:27018")
	if err != nil {
		t.Skip("cannot connect to data store")
	}

	err = s.StoreAlias(ctx, &models.Redir{
		Alias:   kalias,
		URL:     "link",
		Private: false,
		Trust:   false,
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

	r, err := s.FetchAlias(ctx, kalias)
	if err != nil {
		t.Fatalf("FetchAlias failed with err: %v", err)
	}

	err = s.UpdateAlias(ctx, &models.Redir{
		ID:    r.ID,
		Alias: kalias,
		URL:   want,
	})
	if err != nil {
		t.Fatalf("UpdateAlias failed with err: %v", err)
	}

	r, err = s.FetchAlias(ctx, kalias)
	if err != nil {
		t.Fatalf("UpdateAlias failed with err: %v", err)
	}
	if r.URL != want {
		t.Fatalf("Incorrect UpdateAlias implementaiton, want %v, got %v", want, r.URL)
	}
}

type indexOutput struct {
	Data  []models.RedirIndex `json:"data"`
	Page  int64               `json:"page"`
	Total int64               `json:"total"`
}

func TestFetchAliasAll(t *testing.T) {
	ctx := context.Background()
	s, err := db.NewStore(ctx, "mongodb://0.0.0.0:27018")
	if err != nil {
		t.Skip("cannot connect to data store")
	}
	rs, total, err := s.FetchAliasAll(ctx, true, 20, 1)
	if err != nil || len(rs) == 0 || total == 0 {
		t.Fatalf("fetch failed: %v, %v, %v", err, rs, total)
	}
	b, err := json.Marshal(indexOutput{
		Data:  rs,
		Page:  int64(1),
		Total: total,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
}

func BenchmarkFetchAliasAll(b *testing.B) {
	ctx := context.Background()
	s, err := db.NewStore(ctx, "mongodb://0.0.0.0:27018")
	if err != nil {
		b.Skip("cannot connect to data store")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rs, total, err := s.FetchAliasAll(ctx, false, 100, 1)
		if err != nil || len(rs) == 0 || total == 0 {
			b.Fatalf("fetch failed: %v, %v, %v", err, rs, total)
		}
	}
}
