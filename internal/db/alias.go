// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"errors"
	"fmt"

	"changkun.de/x/redir/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StoreAlias stores a given short alias with the given link if not exists
func (db *Store) StoreAlias(ctx context.Context, r *models.Redirect) (err error) {
	col := db.cli.Database(dbname).Collection(collink)

	opts := options.Update().SetUpsert(true)
	filter := bson.M{"alias": r.Alias, "kind": r.Kind}

	_, err = col.UpdateOne(ctx, filter, bson.M{"$set": bson.M{
		// do not use r directly, because it can clear object id.
		"alias":      r.Alias,
		"kind":       r.Kind,
		"url":        r.URL,
		"private":    r.Private,
		"valid_from": r.ValidFrom,
	}}, opts)
	if err != nil {
		err = fmt.Errorf("failed to insert given redirect: %w", err)
		return
	}
	return
}

// UpdateAlias updates the link of a given alias
func (db *Store) UpdateAlias(ctx context.Context, r *models.Redirect) error {
	if r.ID == "" {
		return errors.New("missing document ID")
	}
	id, err := primitive.ObjectIDFromHex(r.ID)
	if err != nil {
		return err
	}

	col := db.cli.Database(dbname).Collection(collink)

	var ret models.Redirect
	err = col.FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"alias":      r.Alias,
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
func (db *Store) FetchAlias(ctx context.Context, a string) (*models.Redirect, error) {
	col := db.cli.Database(dbname).Collection(collink)

	var r models.Redirect
	err := col.FindOne(ctx, bson.M{"alias": a}).Decode(&r)
	if err != nil {
		return nil, fmt.Errorf("cannot find alias %s: %v", a, err)
	}
	return &r, nil
}

// FetchAliasAll reads all aliases by given page size and page number.
func (db *Store) FetchAliasAll(ctx context.Context, public bool, pageSize, pageNum int64) ([]models.Redirect, int64, error) {
	col := db.cli.Database(dbname).Collection(collink)

	filter := bson.M{}
	if public {
		filter = bson.M{"private": false}
	}

	opts := []*options.FindOptions{
		options.Find().SetLimit(pageSize),
		options.Find().SetSkip((pageNum - 1) * pageSize),
	}
	if public {
		opts = append(opts, options.Find().SetProjection(bson.M{"url": 0}))
	}

	cur, err := col.Find(ctx, filter, opts...)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	n, err := col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	var rs []models.Redirect
	if err := cur.All(ctx, &rs); err != nil {
		return nil, 0, err
	}

	return rs, n, nil
}
