// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	dbname   = "redir"
	collink  = "links"
	colvisit = "visit"
)

type Store struct {
	cli *mongo.Client
}

// NewStore parses the given URI and returns the database instantiation.
func NewStore(ctx context.Context, uri string) (*Store, error) {
	// initialize database connection
	db, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}
	err = db.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}

	return &Store{db}, nil
}

func (db *Store) Close() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err = db.cli.Disconnect(ctx)
	if err != nil {
		err = fmt.Errorf("failed to close database: %w", err)
	}
	return
}
