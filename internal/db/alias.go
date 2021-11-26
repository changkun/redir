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

// StoreAlias stores a given short alias with the given link if not exists
func (db *Store) StoreAlias(ctx context.Context, r *models.Redir) (err error) {
	col := db.cli.Database(dbname).Collection(collink)

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"alias": r.Alias}

	now := time.Now().UTC()
	ret, err := col.UpdateOne(ctx, filter, bson.M{"$setOnInsert": bson.M{
		// do not use r directly, because it can clear object id.
		"alias":      r.Alias,
		"url":        r.URL,
		"private":    r.Private,
		"trust":      r.Trust,
		"valid_from": r.ValidFrom,
		"created_by": r.CreatedBy,
		"updated_by": r.UpdatedBy,
		"created_at": now,
		"updated_at": now,
	}}, opts)
	if err != nil {
		err = fmt.Errorf("failed to insert given redirect: %w", err)
		return
	}
	if ret.MatchedCount > 0 {
		err = errors.New("alias already existed")
	}
	return
}

// UpdateAlias updates the link of a given alias
func (db *Store) UpdateAlias(ctx context.Context, r *models.Redir) error {
	if r.ID == "" {
		return errors.New("missing document ID")
	}
	id, err := primitive.ObjectIDFromHex(r.ID)
	if err != nil {
		return err
	}

	col := db.cli.Database(dbname).Collection(collink)

	var ret models.Redir
	err = col.FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"alias":      r.Alias,
			"url":        r.URL,
			"private":    r.Private,
			"trust":      r.Trust,
			"valid_from": r.ValidFrom,
			"updated_by": r.UpdatedBy,
			"updated_at": time.Now(),
		}},
	).Decode(&ret)
	if err != nil {
		err = fmt.Errorf("failed to update alias %s: %v", r.Alias, err)
		return err
	}
	return nil
}

// DeleteAlias deletes a given short alias if exists.
func (db *Store) DeleteAlias(ctx context.Context, a string) (err error) {
	col := db.cli.Database(dbname).Collection(collink)

	_, err = col.DeleteMany(ctx, bson.M{"alias": a})
	if err != nil {
		err = fmt.Errorf("delete alias %s failed: %w", a, err)
		return
	}
	return
}

// FetchAlias reads a given alias and returns the associated link.
func (db *Store) FetchAlias(ctx context.Context, a string) (*models.Redir, error) {
	col := db.cli.Database(dbname).Collection(collink)

	var r models.Redir
	err := col.FindOne(ctx, bson.M{"alias": a}).Decode(&r)
	if err != nil {
		return nil, fmt.Errorf("cannot find alias %s: %v", a, err)
	}
	return &r, nil
}

// FetchAliasAll reads all aliases by given page size and page number.
func (db *Store) FetchAliasAll(
	ctx context.Context,
	public bool,
	pageSize, pageNum int64,
) ([]models.RedirIndex, int64, error) {
	col := db.cli.Database(dbname).Collection(collink)

	// public UI does not offer any statistic informations:
	// no PV/UV, no actual URLs.
	if public {
		filter := bson.M{"private": false}
		cur, err := col.Find(ctx, filter, []*options.FindOptions{
			options.Find().SetLimit(pageSize),
			options.Find().SetSkip((pageNum - 1) * pageSize),
			options.Find().SetProjection(bson.M{"url": 0})}...)
		if err != nil {
			return nil, 0, err
		}
		defer cur.Close(ctx)

		n, err := col.CountDocuments(ctx, filter)
		if err != nil {
			return nil, 0, err
		}
		var rs []models.RedirIndex
		if err := cur.All(ctx, &rs); err != nil {
			return nil, 0, err
		}
		return rs, n, nil
	}

	// Non-public mode queries PV/UV as additional information,
	// and paginates on this. Let's first find the aliases.
	n, err := col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	// db.links.aggregate([
	// 	{$skip:  20},
	// 	{$limit: 10},
	// 	{'$lookup': {from: 'visit', localField: 'alias', foreignField: 'alias', as: 'visit'}},
	// 	{'$unwind': {path: '$visit', preserveNullAndEmptyArrays: true}},
	// 	{
	// 		$group: {
	// 			_id: {alias: '$alias', ip: '$visit.ip'},
	// 			url: {$first: '$url'},
	// 			private: {$first: '$private'},
	// 			trust: {$first: '$trust'},
	// 			valid_from: {$first: '$valid_from'},
	// 			created_by: {$first: '$created_by'},
	// 			updated_by: {$first: '$updated_by'},
	// 			updated_at: {$first: '$updated_at'},
	// 			count: {$sum: 1},
	// 		},
	// 	},
	// 	{$group: {
	// 		_id: '$_id.alias',
	// 		alias: {$first: '$_id.alias'},
	// 		url: {$first: '$url'},
	// 		private: {$first: '$private'},
	// 		trust: {$first: '$trust'},
	// 		valid_from: {$first: '$valid_from'},
	// 		created_by: {$first: '$created_by'},
	// 		updated_by: {$first: '$updated_by'},
	// 		updated_at: {$first: '$updated_at'},
	// 		uv: {$sum: 1},
	// 		pv: {$sum: '$count'},
	// 	}},
	// 	{$sort : {updated_at: -1}},
	// 	{$sort : {pv: -1}},
	// 	{$sort : {uv: -1}},
	// ])
	cur, err := col.Aggregate(ctx, mongo.Pipeline{
		bson.D{
			primitive.E{Key: "$skip", Value: (pageNum - 1) * pageSize},
		},
		bson.D{
			primitive.E{Key: "$limit", Value: pageSize},
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
				"_id":        bson.M{"alias": "$alias", "ip": "$visit.ip"},
				"url":        bson.M{"$first": "$url"},
				"private":    bson.M{"$first": "$private"},
				"trust":      bson.M{"$first": "$trust"},
				"valid_from": bson.M{"$first": "$valid_from"},
				"created_by": bson.M{"$first": "$created_by"},
				"updated_by": bson.M{"$first": "$updated_by"},
				"updated_at": bson.M{"$first": "$updated_at"},
				"count":      bson.M{"$sum": 1},
			}},
		},
		bson.D{
			primitive.E{Key: "$group", Value: bson.M{
				"_id":        "$_id.alias",
				"alias":      bson.M{"$first": "$_id.alias"},
				"url":        bson.M{"$first": "$url"},
				"private":    bson.M{"$first": "$private"},
				"trust":      bson.M{"$first": "$trust"},
				"valid_from": bson.M{"$first": "$valid_from"},
				"created_by": bson.M{"$first": "$created_by"},
				"updated_by": bson.M{"$first": "$updated_by"},
				"updated_at": bson.M{"$first": "$updated_at"},
				"uv":         bson.M{"$sum": 1},
				"pv":         bson.M{"$sum": "$count"},
			}},
		},
		// After the aggregation, the result is not stable.
		// We still need sort the result a little bit to get more
		// stable result. We can either sort by the PV/UV or the date.

		// Sort by PV/UV.
		// bson.D{
		// 	primitive.E{Key: "$sort", Value: bson.M{"pv": -1, "uv": -1}},
		// },
		// Sort by updated date.
		bson.D{
			primitive.E{Key: "$sort", Value: bson.M{"updated_at": -1}},
		},
	}, &options.AggregateOptions{
		// Sort by natural order.
		Hint: bson.D{
			primitive.E{Key: "$natural", Value: -1},
		},
	})
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var rs []models.RedirIndex
	if err := cur.All(ctx, &rs); err != nil {
		return nil, 0, err
	}

	return rs, n, nil
}
