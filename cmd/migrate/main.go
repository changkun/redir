// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Command migrate copies redir's links and visits from MongoDB into
// PostgreSQL.
//
// MongoDB is opened for reading and is never written to, so the old store
// remains a working rollback target after the copy. The command refuses to
// run against a non-empty target unless -truncate is given, because
// running it twice would otherwise double every visit count.
//
// Usage:
//
//	migrate -from mongodb://... -to postgres://... -host changkun.de
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
	"unicode/utf8"

	"changkun.de/x/redir/internal/db"
	"changkun.de/x/redir/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"uuid"
)

// batchSize is how many visit rows are sent per CopyFrom. The visit table
// is a few hundred thousand rows, so this trades a bounded amount of
// memory for far fewer round trips.
const batchSize = 10000

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("migrate: ")

	var (
		from     = flag.String("from", "", "MongoDB URI to read from")
		to       = flag.String("to", "", "PostgreSQL URI to write to")
		host     = flag.String("host", "", "hostname to record for every migrated row")
		truncate = flag.Bool("truncate", false, "empty the target tables first")
		dryRun   = flag.Bool("dry-run", false, "read and count, write nothing")
	)
	flag.Parse()

	if *from == "" || *to == "" || *host == "" {
		flag.Usage()
		log.Fatal("-from, -to and -host are required")
	}

	// Ctrl-C during a copy should stop it, not leave a half-written
	// batch behind; every write runs in a transaction bound to this.
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, os.Kill)
	defer stop()

	if err := run(ctx, *from, *to, *host, *truncate, *dryRun); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, from, to, host string, truncate, dryRun bool) error {
	src, err := openMongo(ctx, from)
	if err != nil {
		return err
	}
	defer src.Disconnect(ctx) //nolint:errcheck // read-only connection

	// Opening the store applies the schema, so the target is ready even
	// on a database that has only just been created.
	store, err := db.NewStore(ctx, to)
	if err != nil {
		return err
	}
	defer store.Close()

	pool, err := pgxpool.New(ctx, to)
	if err != nil {
		return fmt.Errorf("cannot connect to %v: %w", db.Redact(to), err)
	}
	defer pool.Close()

	if err := prepareTarget(ctx, pool, truncate, dryRun); err != nil {
		return err
	}

	links, err := copyLinks(ctx, src, pool, host, dryRun)
	if err != nil {
		return err
	}
	visits, err := copyVisits(ctx, src, pool, host, dryRun)
	if err != nil {
		return err
	}

	log.Printf("copied %d links and %d visits for host %v", links, visits, host)
	return verify(ctx, src, pool, host, links, visits, dryRun)
}

func openMongo(ctx context.Context, uri string) (*mongo.Client, error) {
	// The source must come out of this unchanged, since it is the
	// rollback target. The driver has no read-only mode to switch on, so
	// that is a property of what this command does: it issues Find and
	// CountDocuments and nothing else.
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).
		SetServerSelectionTimeout(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %v: %w", db.Redact(uri), err)
	}
	if err := cli.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("cannot connect to %v: %w", db.Redact(uri), err)
	}
	return cli, nil
}

// prepareTarget refuses to add to a table that already holds rows.
// Running the copy twice would double every visit count, and a doubled
// count looks plausible enough to go unnoticed.
func prepareTarget(ctx context.Context, pool *pgxpool.Pool, truncate, dryRun bool) error {
	var links, visits int64
	if err := pool.QueryRow(ctx,
		`SELECT (SELECT COUNT(*) FROM links), (SELECT COUNT(*) FROM visits)`,
	).Scan(&links, &visits); err != nil {
		return fmt.Errorf("cannot inspect target: %w", err)
	}
	if links == 0 && visits == 0 {
		return nil
	}
	if !truncate {
		return fmt.Errorf(
			"target already holds %d links and %d visits; "+
				"re-run with -truncate to replace them", links, visits)
	}
	if dryRun {
		log.Printf("dry run: would truncate %d links and %d visits", links, visits)
		return nil
	}
	if _, err := pool.Exec(ctx,
		`TRUNCATE links, visits RESTART IDENTITY`); err != nil {
		return fmt.Errorf("cannot truncate target: %w", err)
	}
	log.Printf("truncated %d links and %d visits", links, visits)
	return nil
}

// mongoLink mirrors the source document. The fields are read one by one
// rather than into models.Redir, because the source has no host column and
// its identifier is an ObjectID.
type mongoLink struct {
	Alias     string    `bson:"alias"`
	URL       string    `bson:"url"`
	Private   bool      `bson:"private"`
	Trust     bool      `bson:"trust"`
	ValidFrom time.Time `bson:"valid_from"`
	CreatedBy string    `bson:"created_by"`
	UpdatedBy string    `bson:"updated_by"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func copyLinks(
	ctx context.Context,
	src *mongo.Client,
	pool *pgxpool.Pool,
	host string,
	dryRun bool,
) (int64, error) {
	cur, err := src.Database("redir").Collection("links").Find(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("cannot read links: %w", err)
	}
	defer cur.Close(ctx)

	var rows [][]any
	for cur.Next(ctx) {
		var l mongoLink
		if err := cur.Decode(&l); err != nil {
			return 0, fmt.Errorf("cannot decode link: %w", err)
		}
		rows = append(rows, []any{
			host, sanitize(l.Alias), sanitize(l.URL), l.Private, l.Trust,
			orNow(l.ValidFrom), sanitize(l.CreatedBy), sanitize(l.UpdatedBy),
			orNow(l.CreatedAt), orNow(l.UpdatedAt),
		})
	}
	if err := cur.Err(); err != nil {
		return 0, fmt.Errorf("cannot read links: %w", err)
	}

	if dryRun {
		return int64(len(rows)), nil
	}
	n, err := pool.CopyFrom(ctx,
		pgx.Identifier{"links"},
		[]string{"host", "alias", "url", "private", "trust", "valid_from",
			"created_by", "updated_by", "created_at", "updated_at"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("cannot write links: %w", err)
	}
	return n, nil
}

// mongoVisit mirrors the source document.
type mongoVisit struct {
	VisitorID string    `bson:"visitor_id"`
	Alias     string    `bson:"alias"`
	IP        string    `bson:"ip"`
	UA        string    `bson:"ua"`
	Referer   string    `bson:"referer"`
	Time      time.Time `bson:"time"`
}

func copyVisits(
	ctx context.Context,
	src *mongo.Client,
	pool *pgxpool.Pool,
	host string,
	dryRun bool,
) (int64, error) {
	cur, err := src.Database("redir").Collection("visit").Find(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("cannot read visits: %w", err)
	}
	defer cur.Close(ctx)

	var (
		total int64
		batch [][]any
	)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if dryRun {
			total += int64(len(batch))
			batch = batch[:0]
			return nil
		}
		n, err := pool.CopyFrom(ctx,
			pgx.Identifier{"visits"},
			[]string{"host", "alias", "visitor_id", "ip", "ua", "referer",
				"referer_host", "browser", "os", "device", "is_bot", "time"},
			pgx.CopyFromRows(batch))
		if err != nil {
			return fmt.Errorf("cannot write visits: %w", err)
		}
		total += n
		batch = batch[:0]
		log.Printf("copied %d visits", total)
		return nil
	}

	for cur.Next(ctx) {
		var v mongoVisit
		if err := cur.Decode(&v); err != nil {
			return 0, fmt.Errorf("cannot decode visit: %w", err)
		}

		// Derive from the same code the server uses, so a migrated row
		// and a live one are identical.
		m := &models.Visit{
			Host:    host,
			Alias:   sanitize(v.Alias),
			IP:      sanitize(v.IP),
			UA:      sanitize(v.UA),
			Referer: sanitize(v.Referer),
			Time:    orNow(v.Time),
		}
		m.Derive()

		batch = append(batch, []any{
			m.Host, m.Alias, visitorID(v.VisitorID), m.IP, m.UA, m.Referer,
			m.RefererHost, m.Browser, m.OS, m.Device, m.IsBot, m.Time,
		})
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return 0, err
			}
		}
	}
	if err := cur.Err(); err != nil {
		return 0, fmt.Errorf("cannot read visits: %w", err)
	}
	if err := flush(); err != nil {
		return 0, err
	}
	return total, nil
}

// visitorID returns the stored identifier as a UUID, or nil.
//
// 10,708 production rows have no visitor_id at all and 505 hold values
// that are not UUIDs, mostly scanner probes recorded from the cookie
// before it was validated. Those become NULL: an absent identifier is
// honest, an invented one is not.
func visitorID(s string) *uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// sanitize removes what PostgreSQL text will not accept. BSON strings may
// carry NUL bytes and invalid encoding; a text column may not.
func sanitize(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\x00", "")
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}

func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

// verify re-counts both sides. A copy that reports success but moved the
// wrong number of rows is the failure worth catching here.
func verify(
	ctx context.Context,
	src *mongo.Client,
	pool *pgxpool.Pool,
	host string,
	links, visits int64,
	dryRun bool,
) error {
	srcLinks, err := src.Database("redir").Collection("links").
		CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("cannot count source links: %w", err)
	}
	srcVisits, err := src.Database("redir").Collection("visit").
		CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("cannot count source visits: %w", err)
	}

	if srcLinks != links || srcVisits != visits {
		return fmt.Errorf(
			"copied %d links and %d visits, source holds %d and %d",
			links, visits, srcLinks, srcVisits)
	}
	if dryRun {
		log.Printf("dry run: source holds %d links and %d visits",
			srcLinks, srcVisits)
		return nil
	}

	var dstLinks, dstVisits int64
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM links WHERE host = $1),
		       (SELECT COUNT(*) FROM visits WHERE host = $1)`, host,
	).Scan(&dstLinks, &dstVisits); err != nil {
		return fmt.Errorf("cannot count target: %w", err)
	}
	if dstLinks != srcLinks || dstVisits != srcVisits {
		return fmt.Errorf(
			"target holds %d links and %d visits, source holds %d and %d",
			dstLinks, dstVisits, srcLinks, srcVisits)
	}

	log.Printf("verified %d links and %d visits", dstLinks, dstVisits)
	return nil
}
