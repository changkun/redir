// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"uuid"

	"changkun.de/x/redir/internal/models"
)

// RecordVisit records a visit event. If the visit is a new user, it returns
// and ID to set a cookie to the user.
func (db *mongoStore) RecordVisit(ctx context.Context, v *models.Visit) (string, error) {
	col := db.cli.Database(dbname).Collection(colvisit)

	// if visitor ID does not present, then generate a new visitor ID.
	if v.VisitorID == "" {
		v.VisitorID = uuid.New().String()
	}

	_, err := col.InsertOne(ctx, v)
	if err != nil {
		return "", fmt.Errorf("failed to insert record: %w", err)
	}
	return v.VisitorID, nil
}
