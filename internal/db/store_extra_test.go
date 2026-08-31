// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"github.com/jackc/pgx/v5"
)

// TestNewStoreScheme covers the rollback switch. The scheme in the store
// URI is the only thing that decides which backend runs, so a typo must
// be an error rather than a silent fallback to one of them.
func TestNewStoreScheme(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct{ name, uri string }{
		{"no scheme", "127.0.0.1:5432/redir"},
		{"empty", ""},
		{"unsupported", "mysql://127.0.0.1:3306/redir"},
		{"http", "http://127.0.0.1/redir"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := db.NewStore(ctx, tt.uri)
			if err == nil {
				s.Close()
				t.Fatalf("NewStore(%q) succeeded, want an error", tt.uri)
			}
		})
	}
}

// TestRedact keeps database credentials out of logs and error messages.
// The store URI used to be logged whole, which was harmless for the
// MongoDB URI and leaks the password for the PostgreSQL one.
func TestRedact(t *testing.T) {
	for _, tt := range []struct{ in, wantAbsent, wantPresent string }{
		{
			in:          "postgres://redir:s3cret@postgres:5432/redir?sslmode=disable",
			wantAbsent:  "s3cret",
			wantPresent: "postgres:5432/redir",
		},
		{
			in:          "mongodb://0.0.0.0:27018",
			wantAbsent:  "@",
			wantPresent: "0.0.0.0:27018",
		},
		{
			in:          "postgres://redir@postgres:5432/redir",
			wantAbsent:  "xxxxx",
			wantPresent: "redir@postgres",
		},
	} {
		got := db.Redact(tt.in)
		if strings.Contains(got, tt.wantAbsent) {
			t.Errorf("Redact(%q) = %q, still contains %q",
				tt.in, got, tt.wantAbsent)
		}
		if !strings.Contains(got, tt.wantPresent) {
			t.Errorf("Redact(%q) = %q, lost %q", tt.in, got, tt.wantPresent)
		}
	}
}

// TestMigrationsAreIdempotent checks that opening the store twice applies
// nothing the second time. The server opens the store on every start, so
// a migration that ran again would fail the boot.
func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	b := backends()[0]

	s1, err := db.NewStore(ctx, b.uri)
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	s1.Close()

	conn, err := pgx.Connect(ctx, b.uri)
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	defer conn.Close(ctx)

	var before int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM schema_migrations`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before == 0 {
		t.Fatal("no migration was recorded")
	}

	s2, err := db.NewStore(ctx, b.uri)
	if err != nil {
		t.Fatalf("reopening the store failed: %v", err)
	}
	s2.Close()

	var after int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM schema_migrations`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("schema_migrations went from %d to %d rows", before, after)
	}
}

func TestUpdateAliasErrors(t *testing.T) {
	run(t, func(t *testing.T, b backend, s db.Store) {
		ctx := context.Background()

		if err := s.UpdateAlias(ctx, &models.Redir{
			Host: khost, Alias: kalias, URL: "x",
		}); err == nil {
			t.Error("UpdateAlias with no ID succeeded, want an error")
		}
		if err := s.UpdateAlias(ctx, &models.Redir{
			ID: "not-an-id", Host: khost, Alias: kalias, URL: "x",
		}); err == nil {
			t.Error("UpdateAlias with a malformed ID succeeded, want an error")
		}
	})
}

// TestUpdateAliasMissing checks that updating a link that is not there is
// reported rather than silently doing nothing.
func TestUpdateAliasMissing(t *testing.T) {
	run(t, func(t *testing.T, b backend, s db.Store) {
		if !b.multiHost {
			t.Skip("identifier shape differs on the MongoDB backend")
		}
		ctx := context.Background()
		if err := s.UpdateAlias(ctx, &models.Redir{
			ID: "999999", Host: khost, Alias: kalias, URL: "x",
		}); err == nil {
			t.Fatal("UpdateAlias on a missing link succeeded, want an error")
		}
	})
}

// TestUpdateAliasRejectsCollision checks that renaming a link onto one
// that exists is refused rather than losing one of them.
func TestUpdateAliasRejectsCollision(t *testing.T) {
	run(t, func(t *testing.T, b backend, s db.Store) {
		if !b.multiHost {
			t.Skip("the MongoDB backend has no uniqueness constraint")
		}
		ctx := context.Background()

		if err := s.StoreAlias(ctx, &models.Redir{
			Host: khost, Alias: "second", URL: "https://example.com",
		}); err != nil {
			t.Fatal(err)
		}
		r, err := s.FetchAlias(ctx, khost, "second")
		if err != nil {
			t.Fatal(err)
		}

		err = s.UpdateAlias(ctx, &models.Redir{
			ID: r.ID, Host: khost, Alias: kalias, URL: "https://example.com",
		})
		if err != db.ErrAliasExists {
			t.Fatalf("UpdateAlias onto an existing alias err = %v, want ErrAliasExists", err)
		}
	})
}

// TestFetchAliasAllCounts covers the listing the admin dashboard reads,
// which is the only place PV and UV appear next to a link.
func TestFetchAliasAllCounts(t *testing.T) {
	run(t, func(t *testing.T, b backend, s db.Store) {
		ctx := context.Background()
		now := time.Now().UTC()

		for _, ip := range []string{"1.1.1.1", "1.1.1.1", "2.2.2.2"} {
			if _, err := s.RecordVisit(ctx, &models.Visit{
				Host: khost, Alias: kalias, IP: ip, Time: now,
			}); err != nil {
				t.Fatal(err)
			}
		}

		rs, total, err := s.FetchAliasAll(ctx, khost, false, 20, 1)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Fatalf("total = %d, want 1", total)
		}
		var found bool
		for _, r := range rs {
			if r.Alias != kalias {
				continue
			}
			found = true
			if r.URL == "" {
				t.Error("the admin listing hid the URL")
			}
			if r.PV != 3 || r.UV != 2 {
				t.Errorf("PV/UV = %d/%d, want 3/2", r.PV, r.UV)
			}
		}
		if !found {
			t.Fatalf("alias %v missing from %+v", kalias, rs)
		}
	})
}

// TestFetchAliasAllPaging checks that a second page returns different
// links rather than repeating the first.
func TestFetchAliasAllPaging(t *testing.T) {
	run(t, func(t *testing.T, b backend, s db.Store) {
		if !b.multiHost {
			t.Skip("the MongoDB backend pages before its lookup, see specs/002")
		}
		ctx := context.Background()

		for _, a := range []string{"p1", "p2", "p3"} {
			if err := s.StoreAlias(ctx, &models.Redir{
				Host: khost, Alias: a, URL: "https://example.com/" + a,
			}); err != nil {
				t.Fatal(err)
			}
		}

		seen := map[string]bool{}
		for page := int64(1); page <= 4; page++ {
			rs, total, err := s.FetchAliasAll(ctx, khost, false, 1, page)
			if err != nil {
				t.Fatal(err)
			}
			if total != 4 {
				t.Fatalf("total = %d, want 4", total)
			}
			for _, r := range rs {
				if seen[r.Alias] {
					t.Fatalf("alias %v appeared on two pages", r.Alias)
				}
				seen[r.Alias] = true
			}
		}
		if len(seen) != 4 {
			t.Fatalf("paged through %d links, want 4", len(seen))
		}
	})
}

// TestStatVisitHistBothBackends exercises the hourly histogram on both
// stores. The MongoDB pipeline behind it is elaborate and no test covered
// it before.
func TestStatVisitHistBothBackends(t *testing.T) {
	run(t, func(t *testing.T, b backend, s db.Store) {
		ctx := context.Background()
		base := time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)

		// Two visits in one hour from one address, one in the next hour
		// from another.
		visits := []struct {
			at time.Time
			ip string
		}{
			{base.Add(10 * time.Minute), "1.1.1.1"},
			{base.Add(20 * time.Minute), "1.1.1.1"},
			{base.Add(time.Hour + time.Minute), "2.2.2.2"},
		}
		for _, v := range visits {
			if _, err := s.RecordVisit(ctx, &models.Visit{
				Host: khost, Alias: kalias, IP: v.ip, Time: v.at,
			}); err != nil {
				t.Fatal(err)
			}
		}

		hist, err := s.StatVisitHist(ctx, khost, kalias,
			base.Add(-time.Hour), base.Add(4*time.Hour))
		if err != nil {
			t.Fatal(err)
		}

		var pv, uv int
		for _, h := range hist {
			pv += h.PV
			uv += h.UV
		}
		if pv != 3 {
			t.Errorf("total PV = %d, want 3 (%+v)", pv, hist)
		}
		// Two addresses, in separate hours, so the per-hour unique
		// counts add to two.
		if uv != 2 {
			t.Errorf("total UV = %d, want 2 (%+v)", uv, hist)
		}
	})
}

// TestStatsOnUnknownAliasAreEmpty checks that asking about an alias that
// does not exist returns nothing rather than failing, since the dashboard
// can be pointed at a deleted link.
func TestStatsOnUnknownAliasAreEmpty(t *testing.T) {
	run(t, func(t *testing.T, b backend, s db.Store) {
		ctx := context.Background()
		now := time.Now().UTC()
		from, to := now.Add(-time.Hour), now.Add(time.Hour)

		refs, err := s.StatReferer(ctx, khost, "no-such-alias", from, to)
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != 0 {
			t.Errorf("referer stats = %+v, want none", refs)
		}
		uas, err := s.StatUA(ctx, khost, "no-such-alias", from, to)
		if err != nil {
			t.Fatal(err)
		}
		if len(uas) != 0 {
			t.Errorf("ua stats = %+v, want none", uas)
		}
		rs, err := s.StatVisit(ctx, khost, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 0 {
			t.Errorf("visit stats for no aliases = %+v, want none", rs)
		}
	})
}

// TestNewStoreUnreachable checks that an unreachable database is reported
// rather than returning a store that fails on first use. The server calls
// log.Fatal on this, so it must arrive as an error.
func TestNewStoreUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, tt := range []struct{ name, uri string }{
		{"postgres", "postgres://redir:redir@127.0.0.1:1/redir?sslmode=disable"},
		{"mongo", "mongodb://127.0.0.1:1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := db.NewStore(ctx, tt.uri)
			if err == nil {
				s.Close()
				t.Fatal("NewStore succeeded against a closed port")
			}
			// The message reaches the logs, so it must not carry the
			// password.
			if strings.Contains(err.Error(), "redir:redir@") {
				t.Errorf("error message leaked the password: %v", err)
			}
		})
	}
}

// TestRedactInvalid checks that an unparseable URI yields a placeholder
// rather than the original string, which could be a malformed URI that
// still holds a password.
func TestRedactInvalid(t *testing.T) {
	const bad = "postgres://redir:s3cret@%zz:5432/redir"
	if got := db.Redact(bad); strings.Contains(got, "s3cret") {
		t.Fatalf("Redact(%q) = %q, still contains the password", bad, got)
	}
}
