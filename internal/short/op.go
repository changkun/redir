// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package short

// Op is a short link operator
type Op string

const (
	// opCreate represents a create operation for short link
	OpCreate Op = "create"
	// opDelete represents a delete operation for short link
	OpDelete = "delete"
	// opUpdate represents a update operation for short link
	OpUpdate = "update"
	// opFetch represents a fetch operation for short link
	OpFetch = "fetch"
)

// Valid checks if the given Op is valid.
func (o Op) Valid() bool {
	switch o {
	case OpCreate, OpDelete, OpUpdate, OpFetch:
		return true
	default:
		return false
	}
}
