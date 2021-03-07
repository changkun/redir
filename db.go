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
	Kind    aliasKind `json:"kind"    bson:"kind"`
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
	filter := bson.M{"alias": r.Alias, "kind": r.Kind}

	_, err = col.UpdateOne(ctx, filter, bson.M{"$set": r}, opts)
	if err != nil {
		err = fmt.Errorf("failed to insert given redirect: %w", err)
		return
	}
	return
}

// UpdateAlias updates the link of a given alias
func (db *database) UpdateAlias(ctx context.Context, a, l string) (*redirect, error) {
	col := db.cli.Database(dbname).Collection(collink)

	var r redirect
	err := col.FindOneAndUpdate(ctx,
		bson.M{"alias": a},
		bson.M{"$set": bson.M{"url": l}},
	).Decode(&r)
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

	_, err = col.DeleteMany(ctx, bson.M{"alias": a})
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
	err := col.FindOne(ctx, bson.M{"alias": a}).Decode(&r)
	if err != nil {
		return nil, fmt.Errorf("cannot find alias %s: %v", a, err)
	}
	return &r, nil
}

type refstat struct {
	Referer string `json:"referer" bson:"referer"`
	Count   int64  `json:"count"   bson:"count"`
}

// CountReferer fetches and counts all referers of a given alias
func (db *database) CountReferer(ctx context.Context, a string, k aliasKind, start, end time.Time) ([]refstat, error) {
	col := db.cli.Database(dbname).Collection(collink)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{
			primitive.E{Key: "$match", Value: bson.M{
				"kind": k, "alias": a,
			}},
		},
		bson.D{
			primitive.E{Key: "$lookup", Value: bson.M{
				"from": colvisit,
				"as":   "visit",
				"pipeline": mongo.Pipeline{bson.D{
					primitive.E{Key: "$match", Value: bson.M{
						"$expr": bson.M{
							"$and": []bson.M{
								{"$eq": []string{a, "$alias"}},
								{"$gte": []interface{}{"$time", start}},
								{"$lt": []interface{}{"$time", end}},
							},
						},
					}},
				}},
			}},
		},
		bson.D{
			primitive.E{Key: "$unwind", Value: bson.M{
				"path":                       "$visit",
				"preserveNullAndEmptyArrays": true,
			}},
		},
		bson.D{
			primitive.E{Key: "$group", Value: bson.M{
				"_id": bson.M{
					"$cond": bson.M{
						"if": bson.M{
							"$eq": []string{"", "$visit.referer"},
						},
						"then": "unknown",
						"else": "$visit.referer",
					},
				},
				"referer": bson.M{"$first": bson.M{
					"$cond": bson.M{
						"if": bson.M{
							"$eq": []string{"", "$visit.referer"},
						},
						"then": "unknown",
						"else": "$visit.referer",
					},
				}},
				"count": bson.M{"$sum": 1},
			}},
		},
		bson.D{
			primitive.E{Key: "$sort", Value: bson.M{"count": -1}},
		},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to count referer: %w", err)
	}
	defer cur.Close(ctx)

	var results []refstat
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch referer results: %w", err)
	}

	return results, nil
}

type uastat struct {
	UA    string `json:"ua"    bson:"ua"`
	Count int64  `json:"count" bson:"count"`
}

func (db *database) CountUA(ctx context.Context, a string, k aliasKind, start, end time.Time) ([]uastat, error) {
	col := db.cli.Database(dbname).Collection(collink)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{
			primitive.E{Key: "$match", Value: bson.M{
				"kind": k, "alias": a,
			}},
		},
		bson.D{
			primitive.E{Key: "$lookup", Value: bson.M{
				"from": colvisit,
				"as":   "visit",
				"pipeline": mongo.Pipeline{bson.D{
					primitive.E{Key: "$match", Value: bson.M{
						"$expr": bson.M{
							"$and": []bson.M{
								{"$eq": []string{a, "$alias"}},
								{"$gte": []interface{}{"$time", start}},
								{"$lt": []interface{}{"$time", end}},
							},
						},
					}},
				}},
			}},
		},
		bson.D{
			primitive.E{Key: "$unwind", Value: bson.M{
				"path":                       "$visit",
				"preserveNullAndEmptyArrays": true,
			}},
		},
		bson.D{
			primitive.E{Key: "$group", Value: bson.M{
				"_id": bson.M{
					"$cond": bson.M{
						"if": bson.M{
							"$eq": []string{"", "$visit.ua"},
						},
						"then": "unknown",
						"else": "$visit.ua",
					},
				},
				"ua": bson.M{"$first": bson.M{
					"$cond": bson.M{
						"if": bson.M{
							"$eq": []string{"", "$visit.ua"},
						},
						"then": "unknown",
						"else": "$visit.ua",
					},
				}},
				"count": bson.M{"$sum": 1},
			}},
		},
		bson.D{
			primitive.E{Key: "$sort", Value: bson.M{"count": -1}},
		},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to count ua: %w", err)
	}
	defer cur.Close(ctx)

	var results []uastat
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch ua results: %w", err)
	}

	return results, nil
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

type record struct {
	Alias string `bson:"alias"`
	UV    int64  `bson:"uv"`
	PV    int64  `bson:"pv"`
}

// CountVisit counts the PV/UV of aliases of a given kind
func (db *database) CountVisit(ctx context.Context, kind aliasKind) (rs []record, err error) {
	// uv based on number of ip, this is not accurate since the number will be
	// smaller than the actual.
	// raw query:
	//
	// db.links.aggregate([
	// 	{$match: {kind: 0}},
	// 	{'$lookup': {from: 'visit', localField: 'alias', foreignField: 'alias', as: 'visit'}},
	// 	{'$unwind': {path: '$visit', preserveNullAndEmptyArrays: true}},
	// 	{$group: {_id: {alias: '$alias', ip: '$visit.ip'}, count: {$sum: 1}}},
	// 	{$group: {_id: '$_id.alias', uv: {$sum: 1}, pv: {$sum: '$count'}}},
	// 	{$sort : {pv: -1}},
	// 	{$sort : {uv: -1}},
	// ])
	col := db.cli.Database(dbname).Collection(collink)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{
			primitive.E{Key: "$match", Value: bson.M{
				"kind": kind,
			}},
		},
		bson.D{
			primitive.E{Key: "$lookup", Value: bson.M{
				"from":         colvisit,
				"localField":   "alias",
				"foreignField": "alias",
				"as":           "visit",
			}},
		},
		bson.D{
			primitive.E{Key: "$unwind", Value: bson.M{
				"path":                       "$visit",
				"preserveNullAndEmptyArrays": true,
			}},
		},
		bson.D{
			primitive.E{Key: "$group", Value: bson.M{
				"_id":   bson.M{"alias": "$alias", "ip": "$visit.ip"},
				"count": bson.M{"$sum": 1},
			}},
		},
		bson.D{
			primitive.E{Key: "$group", Value: bson.M{
				"_id":   "$_id.alias",
				"alias": bson.M{"$first": "$_id.alias"},
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
