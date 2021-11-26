// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"time"

	"changkun.de/x/redir/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StatReferer fetches and counts all referers of a given alias
func (db *Store) StatReferer(
	ctx context.Context,
	a string,
	start, end time.Time,
) ([]models.RefStat, error) {

	col := db.cli.Database(dbname).Collection(collink)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{
			primitive.E{Key: "$match", Value: bson.M{"alias": a}},
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

func (db *Store) StatUA(
	ctx context.Context,
	a string,
	start, end time.Time,
) ([]models.UAStat, error) {

	col := db.cli.Database(dbname).Collection(collink)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{
			primitive.E{Key: "$match", Value: bson.M{"alias": a}},
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

// StatVisitHist is a enhanced version of StatVisit.
// It offers the ability to query PV/UV for a range of time.
//
// The current approach is to count IP address.
func (db *Store) StatVisitHist(
	ctx context.Context,
	a string,
	start, end time.Time,
) ([]models.TimeHist, error) {

	// Raw query
	// db.links.aggregate([
	// 	{$match: {alias: 'changkun'}},
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
			Key: "$match", Value: bson.M{"alias": a},
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

// StatVisit counts the PV/UV of given aliases.
//
// The current approach is to use visitor's IP address.
func (db *Store) StatVisit(ctx context.Context, as []string) (rs []models.VisitRecord, err error) {
	if len(as) == 0 {
		return nil, nil
	}

	col := db.cli.Database(dbname).Collection(collink)
	opts := options.Aggregate().SetMaxTime(10 * time.Second)
	matches := []bson.M{}
	for _, a := range as {
		matches = append(matches, bson.M{"alias": a})
	}

	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{
			primitive.E{Key: "$match", Value: bson.M{"$or": matches}},
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
