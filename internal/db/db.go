// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"changkun.de/x/redir/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrExistedAlias = errors.New("alias is existed")
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
func NewStore(uri string) (*Store, error) {
	// initialize database connection
	db, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}
	err = db.Ping(context.Background(), nil)
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

// StoreAlias stores a given short alias with the given link if not exists
func (db *Store) StoreAlias(ctx context.Context, r *models.Redirect) (err error) {
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
func (db *Store) UpdateAlias(ctx context.Context, r *models.Redirect) error {
	col := db.cli.Database(dbname).Collection(collink)

	var ret models.Redirect
	err := col.FindOneAndUpdate(ctx,
		bson.M{"alias": r.Alias},
		bson.M{"$set": bson.M{
			"url":        r.URL,
			"private":    r.Private,
			"valid_from": r.ValidFrom,
		}},
	).Decode(&ret)
	if err != nil {
		err = fmt.Errorf("failed to update alias %s: %v", r.Alias, err)
		return err
	}
	return nil
}

// Delete deletes a given short alias if exists
func (db *Store) DeleteAlias(ctx context.Context, a string) (err error) {
	col := db.cli.Database(dbname).Collection(collink)

	_, err = col.DeleteMany(ctx, bson.M{"alias": a})
	if err != nil {
		err = fmt.Errorf("delete alias %s failed: %w", a, err)
		return
	}
	return
}

// FetchAlias reads a given alias and returns the associated link
func (db *Store) FetchAlias(ctx context.Context, a string) (*models.Redirect, error) {
	col := db.cli.Database(dbname).Collection(collink)

	var r models.Redirect
	err := col.FindOne(ctx, bson.M{"alias": a}).Decode(&r)
	if err != nil {
		return nil, fmt.Errorf("cannot find alias %s: %v", a, err)
	}
	return &r, nil
}

// CountReferer fetches and counts all referers of a given alias
func (db *Store) CountReferer(
	ctx context.Context,
	a string,
	k models.AliasKind,
	start, end time.Time,
) ([]models.RefStat, error) {

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

	var results []models.RefStat
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch referer results: %w", err)
	}

	return results, nil
}

func (db *Store) CountUA(
	ctx context.Context,
	a string,
	k models.AliasKind,
	start, end time.Time,
) ([]models.UAStat, error) {

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

	var results []models.UAStat
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch ua results: %w", err)
	}

	return results, nil
}

func (db *Store) CountVisitHist(
	ctx context.Context,
	a string,
	k models.AliasKind,
	start, end time.Time,
) ([]models.TimeHist, error) {

	// Raw query
	// db.links.aggregate([
	// 	{$match: {kind: 0, alias: 'changkun'}},
	// 	{'$lookup': {
	// 		from: 'visit', localField: 'alias',
	// 		foreignField: 'alias', as: 'visit'},
	// 	},
	// 	{
	// 		$group: {
	// 			_id: "$alias",
	// 			time: {'$first': '$visit.time'},
	// 			ip: {'$first': '$visit.ip'},
	// 		},
	// 	},
	// 	{'$unwind': {path: "$time", includeArrayIndex: "idx1"}},
	// 	{'$unwind': {path: "$ip",   includeArrayIndex: "idx2"}},
	// 	{
	// 		$project: {
	// 			_id: 1,
	// 			time: 1,
	// 			ip:   1,
	// 			valid: {$eq: ["$idx1", "$idx2"]},
	// 		},
	// 	},
	// 	{$match: {valid: true}},
	// 	{
	// 		$project: {
	// 			_id: {
	// 				"year": {$year: "$time"},
	// 				"month": {$month: "$time"},
	// 				"day": {$dayOfMonth: "$time"},
	// 				"hour": {$hour: "$time"},
	// 			},
	// 			"time": "$time",
	// 			"ip": "$ip",
	// 		},
	// 	},
	// 	{
	// 		$group: {
	// 			_id: "$_id",
	// 			time: {$first: '$time'},
	// 			visits: {$push: '$ip'},
	// 			users: {$addToSet: '$ip'},
	// 		},
	// 	},
	// 	{
	// 		$project: {
	// 			time: 1,
	// 			pv: {$size: '$visits'},
	// 			uv: {$size: '$users'},
	// 		}
	// 	}
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
			Key: "$group", Value: bson.M{
				"_id":  "$alias",
				"time": bson.M{"$first": "$visit.time"},
				"ip":   bson.M{"$first": "$visit.ip"},
			},
		}},
		bson.D{primitive.E{
			Key: "$unwind", Value: bson.M{
				"path":              "$time",
				"includeArrayIndex": "idx1",
			},
		}},
		bson.D{primitive.E{
			Key: "$unwind", Value: bson.M{
				"path":              "$ip",
				"includeArrayIndex": "idx2",
			},
		}},
		bson.D{primitive.E{
			Key: "$project", Value: bson.M{
				"_id":   1,
				"time":  1,
				"ip":    1,
				"valid": bson.M{"$eq": []string{"$idx1", "$idx2"}},
			},
		}},
		bson.D{primitive.E{
			Key: "$match", Value: bson.M{"valid": true},
		}},
		bson.D{primitive.E{
			Key: "$project", Value: bson.M{
				"_id": bson.M{
					"year":  bson.M{"$year": "$time"},
					"month": bson.M{"$month": "$time"},
					"day":   bson.M{"$dayOfMonth": "$time"},
					"hour":  bson.M{"$hour": "$time"},
				},
				"time": "$time",
				"ip":   "$ip",
			},
		}},
		bson.D{primitive.E{
			Key: "$group",
			Value: bson.M{
				"_id":    "$_id",
				"time":   bson.M{"$first": "$time"},
				"visits": bson.M{"$push": "$ip"},
				"users":  bson.M{"$addToSet": "$ip"},
			},
		}},
		bson.D{primitive.E{
			Key: "$project",
			Value: bson.M{
				"time": 1,
				"pv":   bson.M{"$size": "$visits"},
				"uv":   bson.M{"$size": "$users"},
			},
		}},
		// bson.D{primitive.E{
		// 	Key: "$sort", Value: bson.M{
		// 		"pv": -1, "uv": -1,
		// 	},
		// }},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to count time hist: %w", err)
	}
	defer cur.Close(ctx)

	var results []models.TimeHist
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch time hist results: %w", err)
	}
	return results, nil
}

func (db *Store) RecordVisit(ctx context.Context, v *models.Visit) (err error) {
	col := db.cli.Database(dbname).Collection(colvisit)

	_, err = col.InsertOne(ctx, v)
	if err != nil {
		err = fmt.Errorf("failed to insert record: %w", err)
		return
	}
	return
}

// VisitRecord represents the visit record of an alias.

// CountVisit counts the PV/UV of aliases of a given kind
func (db *Store) CountVisit(ctx context.Context, kind models.AliasKind) (rs []models.VisitRecord, err error) {
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

	var results []models.VisitRecord
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to fetch visit results: %w", err)
	}

	return results, nil
}
