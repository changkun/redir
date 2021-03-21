// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package models

import (
	"time"
)

// AliasKind represents a kind of alias
type AliasKind int

// All kinds of aliases
const (
	KindShort AliasKind = iota
	KindRandom
)

// Redirect records a kind of alias and its correlated link.
type Redirect struct {
	ID        string    `json:"-"          bson:"_id"`
	Alias     string    `json:"alias"      bson:"alias"`
	Kind      AliasKind `json:"kind"       bson:"kind"`
	URL       string    `json:"url"        bson:"url"`
	Private   bool      `json:"private"    bson:"private"`
	ValidFrom time.Time `json:"valid_from,omitempty" bson:"valid_from"`
}

// Visit indicates an record of visit pattern.
type Visit struct {
	VisitorID string    `json:"visitor_id" bson:"visitor_id"`
	Alias     string    `json:"alias"      bson:"alias"`
	Kind      AliasKind `json:"kind"       bson:"kind"`
	IP        string    `json:"ip"         bson:"ip"`
	UA        string    `json:"ua"         bson:"ua"`
	Referer   string    `json:"referer"    bson:"referer"`
	Time      time.Time `json:"time"       bson:"time"`
}

// VisitRecord represents the visit record of an alias.
// The record does not contain time range so that the user of this struct
// can customize it.
type VisitRecord struct {
	Alias string `json:"alias" bson:"alias"`
	UV    int64  `json:"uv"    bson:"uv"`
	PV    int64  `json:"pv"    bson:"pv"`
}

// Referrer statistic
type RefStat struct {
	Referer string `json:"referer" bson:"referer"`
	Count   int64  `json:"count"   bson:"count"`
}

// UAStat statistics
type UAStat struct {
	UA    string `json:"ua"    bson:"ua"`
	Count int64  `json:"count" bson:"count"`
}

// TimeHist statistics
type TimeHist struct {
	Time time.Time `bson:"time" json:"time"`
	PV   int       `bson:"pv"   json:"pv"`
	UV   int       `bson:"uv"   json:"uv"`
}
