// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package short_test

import (
	"context"
	"testing"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"changkun.de/x/redir/internal/short"
)

// fakeStore holds one link, so an update can be checked against what it
// started as.
type fakeStore struct {
	db.Store
	stored  *models.Redir
	updated *models.Redir
}

func (s *fakeStore) FetchAlias(context.Context, string, string) (*models.Redir, error) {
	c := *s.stored
	return &c, nil
}

func (s *fakeStore) UpdateAlias(_ context.Context, r *models.Redir) error {
	c := *r
	s.updated = &c
	return nil
}

func (s *fakeStore) StoreAlias(_ context.Context, r *models.Redir) error {
	c := *r
	s.updated = &c
	return nil
}

// TestUpdateKeepsUnmentionedFields is a regression test for a bug the
// code carried a FIXME about.
//
// A boolean flag that was not passed is false, which is indistinguishable
// from one passed as false. An update read that as an instruction, so
// `redir -op update -a x -l <new url>` also made a private link public
// and a trusted link warn before redirecting. Nothing said so.
func TestUpdateKeepsUnmentionedFields(t *testing.T) {
	held := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	s := &fakeStore{stored: &models.Redir{
		ID: "1", Alias: "x", URL: "https://old.example",
		Private: true, Trust: true, ValidFrom: held,
	}}

	// Only the URL is given, as `-op update -a x -l ...` supplies.
	err := short.Edit(context.Background(), s, short.OpUpdate, "x",
		&models.Redir{Alias: "x", URL: "https://new.example"},
		short.Given{URL: true})
	if err != nil {
		t.Fatal(err)
	}

	if s.updated.URL != "https://new.example" {
		t.Errorf("URL = %q, want the new one", s.updated.URL)
	}
	if !s.updated.Private {
		t.Error("a private link was made public by an update that said nothing about it")
	}
	if !s.updated.Trust {
		t.Error("a trusted link was made to warn by an update that said nothing about it")
	}
	if !s.updated.ValidFrom.Equal(held) {
		t.Errorf("ValidFrom = %v, want the stored %v", s.updated.ValidFrom, held)
	}
}

// TestUpdateAppliesWhatIsGiven checks the other direction: a field that
// was named takes effect, including when its value is the zero one.
func TestUpdateAppliesWhatIsGiven(t *testing.T) {
	s := &fakeStore{stored: &models.Redir{
		ID: "1", Alias: "x", URL: "https://old.example",
		Private: true, Trust: true,
	}}

	err := short.Edit(context.Background(), s, short.OpUpdate, "x",
		&models.Redir{Alias: "x", Private: false, Trust: false},
		short.Given{Private: true, Trust: true})
	if err != nil {
		t.Fatal(err)
	}

	if s.updated.Private {
		t.Error("-p was given as false and did not take effect")
	}
	if s.updated.Trust {
		t.Error("-trust was given as false and did not take effect")
	}
	if s.updated.URL != "https://old.example" {
		t.Errorf("URL = %q, want the stored one", s.updated.URL)
	}
}
