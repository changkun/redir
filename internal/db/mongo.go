// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	dbname   = "redir"
	collink  = "links"
	colvisit = "visit"
)

type mongoStore struct {
	cli *mongo.Client
}

var _ Store = (*mongoStore)(nil)

// newMongoStore connects to MongoDB.
//
// This backend is the rollback path. It predates the host column, so it
// serves one host and ignores the host arguments; running two hosts on it
// would mix their aliases together.
func newMongoStore(ctx context.Context, uri string) (*mongoStore, error) {
	// Bound server selection. Without this, connecting to an address
	// where nothing listens blocks for the driver's 30 second default,
	// which turns a misconfigured store into a 30 second startup stall
	// and makes the test suite appear to hang.
	opts := options.Client().ApplyURI(uri).
		SetServerSelectionTimeout(5 * time.Second)

	// initialize database connection
	db, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}
	err = db.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}

	log.Printf("connected to %v", Redact(uri))
	return &mongoStore{db}, nil
}

func (db *mongoStore) Close() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err = db.cli.Disconnect(ctx)
	if err != nil {
		err = fmt.Errorf("failed to close database: %w", err)
	}
	return
}
