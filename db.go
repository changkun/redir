// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	errExistedAlias = errors.New("alias is existed")
)

type aliasKind int

const (
	kindShort aliasKind = iota
	kindRandom
)

// redirect records a kind of alias and its correlated link.
type redirect struct {
	Alias   string    `json:"alias"   bson:"alias"`
	Kind    aliasKind `json:"kind"    bson:"kind"`
	URL     string    `json:"url"     bson:"url"`
	Private bool      `json:"private" bson:"private"`
}

// visit indicates an record of visit pattern.
type visit struct {
	Alias   string    `json:"alias"   bson:"alias"`
	IP      string    `json:"ip"      bson:"ip"`
	UA      string    `json:"ua"      bson:"ua"`
	Referer string    `json:"referer" bson:"referer"`
	Time    time.Time `json:"time"    bson:"time"`
}

const (
	dbname   = "redir"
	collink  = "links"
	colvisit = "visit"
)

type database struct {
	cli *mongo.Client
}

func newDB(uri string) (*database, error) {
	// initialize database connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	db, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}

	return &database{db}, nil
}

func (db *database) Close() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err = db.cli.Disconnect(ctx)
	if err != nil {
		err = fmt.Errorf("failed to close database: %w", err)
	}
	return
}

// StoreAlias stores a given short alias with the given link if not exists
func (db *database) StoreAlias(ctx context.Context, r *redirect) (err error) {
	col := db.cli.Database(dbname).Collection(collink)

	opts := options.Update().SetUpsert(true)
	filter := bson.D{{"alias", r.Alias}, {"kind", r.Kind}}

	_, err = col.UpdateOne(ctx, filter, bson.D{{"$set", r}}, opts)
	if err != nil {
		err = fmt.Errorf("failed to insert given redirect: %w", err)
		return
	}
	return
}

// UpdateAlias updates the link of a given alias
func (db *database) UpdateAlias(ctx context.Context, a, l string) (*redirect, error) {
	col := db.cli.Database(dbname).Collection(collink)

	filter := bson.D{{"alias", a}}
	update := bson.D{{"$set", bson.D{{"url", l}}}}

	var r redirect
	err := col.FindOneAndUpdate(ctx, filter, update).Decode(&r)
	if err != nil {
		err = fmt.Errorf("failed to update alias %s: %v", a, err)
		return nil, err
	}
	r.URL = l
	return &r, nil
}

// Delete deletes a given short alias if exists
func (db *database) DeleteAlias(ctx context.Context, a string) (err error) {
	col := db.cli.Database(dbname).Collection(collink)

	_, err = col.DeleteMany(ctx, bson.D{{"alias", a}})
	if err != nil {
		err = fmt.Errorf("delete alias %s failed: %w", a, err)
		return
	}
	return
}

// FetchAlias reads a given alias and returns the associated link
func (db *database) FetchAlias(ctx context.Context, a string) (*redirect, error) {
	col := db.cli.Database(dbname).Collection(collink)

	var r redirect
	err := col.FindOne(ctx, bson.D{{"alias", a}}).Decode(&r)
	if err != nil {
		return nil, fmt.Errorf("cannot find alias %s: %v", a, err)
	}
	return &r, nil
}

func (db *database) Aliases(ctx context.Context, kind aliasKind) ([]*redirect, error) {
	col := db.cli.Database(dbname).Collection(collink)

	r := []*redirect{}

	cur, err := col.Find(ctx, bson.D{{"kind", kind}})
	if err != nil {
		return nil, fmt.Errorf("failed to find aliases: %w", err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var result redirect
		err := cur.Decode(&result)
		if err != nil {
			return nil, fmt.Errorf("failed to decode result: %w", err)
		}
		r = append(r, &result)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate all records: %w", err)
	}
	return r, nil
}

func (db *database) RecordVisit(ctx context.Context, v *visit) (err error) {
	col := db.cli.Database(dbname).Collection(colvisit)

	_, err = col.InsertOne(ctx, v)
	if err != nil {
		err = fmt.Errorf("failed to insert record: %w", err)
		return
	}
	return
}

// CountVisit stores a given new visit record
func (db *database) CountVisit(ctx context.Context, alias string) (pv, uv int64, err error) {
	col := db.cli.Database(dbname).Collection(colvisit)

	pv, err = col.CountDocuments(ctx, bson.M{"alias": alias})
	if err != nil {
		return
	}

	// uv based on number of ip, this is not accurate since the number will be
	// smaller than the actual.

	result, err := col.Distinct(ctx, "ip", bson.D{
		{Key: "alias", Value: bson.D{{Key: "$eq", Value: alias}}},
	})
	if err != nil {
		return
	}
	uv = int64(len(result))
	return
}
