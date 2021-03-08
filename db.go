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

type locstat struct {
	Locations []string `bson:"locs" json:"locs"`
}

// CountLocation counts the recorded IPs from visit history.
// FIXME: IP can be changed overtime, it might be a good idea to just store
// the parse geo location (latitude, and longitude, and accuracy).
// Q: Any APIs can convert IP to geo location?
func (db *database) CountLocation(ctx context.Context, a string, k aliasKind, start, end time.Time) ([]string, error) {
	col := db.cli.Database(dbname).Collection(collink)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)

	// db.links.aggregate([ {$match: {kind: 0, alias: 'gp-1-intro'}}, {'$lookup': {from: 'visit', localField: 'alias', foreignField: 'alias', as: 'visit'}}, {'$group': {_id: '$alias', ip: {'$first': '$visit.ip'}}}, ])
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{primitive.E{
			Key: "$match", Value: bson.M{
				"kind": k, "alias": a,
			},
		}},
		bson.D{primitive.E{
			Key: "$lookup", Value: bson.M{
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
			},
		}},
		bson.D{primitive.E{
			Key: "$group",
			Value: bson.M{
				"_id":  "$alias",
				"locs": bson.M{"$first": "$visit.ip"},
			},
		}},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to count ua: %w", err)
	}
	defer cur.Close(ctx)

	var results []locstat
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch ua results: %w", err)
	}

	// Is it possible that we don't have any result or have mutiple entries?
	if len(results) != 1 {
		return []string{}, nil
	}

	return results[0].Locations, nil
}

type timehist struct {
	Year  int `bson:"year"  json:"year"`
	Month int `bson:"month" json:"month"`
	Day   int `bson:"day"   json:"day"`
	Hour  int `bson:"hour"  json:"hour"`
	Count int `bson:"count" json:"count"`
}

func (db *database) CountVisitHist(ctx context.Context, a string, k aliasKind, start, end time.Time) ([]timehist, error) {
	// db.links.aggregate([
	//     {$match: {kind: 0, alias: 'blog'}},
	//     {'$lookup': {from: 'visit', localField: 'alias', foreignField: 'alias', as: 'visit'}},
	//     {
	//         $group: {
	//             _id: "$alias", time: {'$first': '$visit.time'}
	//         },
	//     },
	//     {'$unwind': "$time"},
	//     {
	//         $project: {
	//             "year": {$year: "$time"},
	//             "month": {$month: "$time"},
	//             "day": {$dayOfMonth: "$time"},
	//             "hour": {$hour: "$time"},
	//         },
	//     },
	//     {
	//         "$group":{
	//             "_id": {
	//                 "year":"$year","month":"$month","day":"$day","hour":"$hour",
	//             },
	//             'year': {'$first': '$year'},
	//             'month': {'$first': '$month'},
	//             'day': {'$first': '$day'},
	//             'hour': {'$first': '$hour'},
	//             'count': {$sum: 1},
	//         },
	//     },
	//     {
	//         $sort: {
	//             year: -1,
	//             month: -1,
	//             day: -1,
	//             hour: -1,
	//         },
	//     },
	// ])

	col := db.cli.Database(dbname).Collection(collink)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{primitive.E{
			Key: "$match", Value: bson.M{
				"kind": k, "alias": a,
			},
		}},
		bson.D{primitive.E{
			Key: "$lookup", Value: bson.M{
				"from":         colvisit,
				"localField":   "alias",
				"foreignField": "alias",
				"as":           "visit",
			},
		}},
		bson.D{primitive.E{
			Key: "$group", Value: bson.M{
				"_id":  "$alias",
				"time": bson.M{"$first": "$visit.time"},
			},
		}},
		bson.D{primitive.E{
			Key: "$unwind", Value: bson.M{
				"path": "$time",
			},
		}},
		bson.D{primitive.E{
			Key: "$project", Value: bson.M{
				"year":  bson.M{"$year": "$time"},
				"month": bson.M{"$month": "$time"},
				"day":   bson.M{"$dayOfMonth": "$time"},
				"hour":  bson.M{"$hour": "$time"},
			},
		}},
		bson.D{primitive.E{
			Key: "$group",
			Value: bson.M{
				"_id": bson.M{
					"year":  "$year",
					"month": "$month",
					"day":   "$day",
					"hour":  "$hour",
				},
				"year":  bson.M{"$first": "$year"},
				"month": bson.M{"$first": "$month"},
				"day":   bson.M{"$first": "$day"},
				"hour":  bson.M{"$first": "$hour"},
				"count": bson.M{"$sum": 1},
			},
		}},
		bson.D{primitive.E{
			Key: "$sort", Value: bson.M{
				"year":  -1,
				"month": -1,
				"day":   -1,
				"hour":  -1,
			},
		}},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to count time hist: %w", err)
	}
	defer cur.Close(ctx)

	var results []timehist
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch time hist results: %w", err)
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
			primitive.E{Key: "$sort", Value: bson.M{"pv": -1, "uv": -1}},
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
