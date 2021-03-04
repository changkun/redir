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
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	db, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
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
func (db *database) CountVisit(ctx context.Context) (rs []record, err error) {
	// uv based on number of ip, this is not accurate since the number will be
	// smaller than the actual.
	// raw query:
	//
	// db.getCollection('visit').aggregate([
	// 	{"$lookup": {from: "links", localField: "alias", foreignField: "alias", as: "url"}},
	// 	{"$unwind": "$url"},
	// 	{"$group": {
	// 		_id: {alias: "$alias", ip:"$ip", url: "$url.url"},
	// 		count: {"$sum": 1}}
	// 	},
	// 	{"$group": {
	// 		_id:   "$_id.alias",
	// 		alias: {$first: "$_id.alias"},
	// 		url:   {$first: "$_id.url"},
	// 		pv:    {$sum: 1},
	// 		uv:    {$sum: "$count"}}
	// 	},
	// 	{ $sort : { pv : -1 } },
	// 	{ $sort : { uv : -1 } }
	// 	])

	col := db.cli.Database(dbname).Collection(colvisit)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{
			primitive.E{Key: "$lookup", Value: bson.M{
				"from":         collink,
				"localField":   "alias",
				"foreignField": "alias",
				"as":           "url",
			}},
		},
		bson.D{
			primitive.E{Key: "$unwind", Value: "$url"},
		},
		bson.D{
			primitive.E{Key: "$group", Value: bson.M{
				"_id": bson.M{
					"alias": "$alias",
					"ip":    "$ip",
					"url":   "$url.url",
				},
				"count": bson.M{"$sum": 1},
			}},
		},
		bson.D{
			primitive.E{Key: "$group", Value: bson.M{
				"_id":   "$_id.alias",
				"alias": bson.M{"$first": "$_id.alias"},
				"url":   bson.M{"$first": "$_id.url"},
				"uv":    bson.M{"$sum": 1},
				"pv":    bson.M{"$sum": "$count"},
			}},
		},
		bson.D{
			primitive.E{Key: "$sort", Value: bson.M{"pv": -1}},
		},
		bson.D{
			primitive.E{Key: "$sort", Value: bson.M{"uv": -1}},
		},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to count visit: %w", err)
	}
	defer cur.Close(ctx)

	var results []record
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch visit results: %w", err)
	}

	return results, nil
}
