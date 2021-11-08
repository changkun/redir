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

// Redir is the core redir model, it records a kind of alias
// and its correlated link.
type Redir struct {
	ID        string    `json:"-"          yaml:"-"          bson:"_id"`
	Alias     string    `json:"alias"      yaml:"alias"      bson:"alias"`
	Kind      AliasKind `json:"kind"       yaml:"-"          bson:"kind"`
	URL       string    `json:"url"        yaml:"url"        bson:"url"`
	Private   bool      `json:"private"    yaml:"private"    bson:"private"`
	Trust     bool      `json:"trust"      yaml:"trust"      bson:"trust"`
	ValidFrom time.Time `json:"valid_from" yaml:"valid_from" bson:"valid_from"`
	CreatedBy string    `json:"created_by" yaml:"created_by" bson:"created_by"`
	UpdatedBy string    `json:"updated_by" yaml:"updated_by" bson:"updated_by"`
	CreatedAt time.Time `json:"-"          yaml:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"-" yaml:"updated_at" bson:"updated_at"`
}

// RedirIndex is an extension to Redir, which offers more statistic
// information such as PV/UV.
type RedirIndex struct {
	ID        string    `json:"-"          yaml:"-"          bson:"_id"`
	Alias     string    `json:"alias"      yaml:"alias"      bson:"alias"`
	Kind      AliasKind `json:"kind"       yaml:"-"          bson:"kind"`
	URL       string    `json:"url"        yaml:"url"        bson:"url"`
	Private   bool      `json:"private"    yaml:"private"    bson:"private"`
	Trust     bool      `json:"trust"      yaml:"trust"      bson:"trust"`
	ValidFrom time.Time `json:"valid_from" yaml:"valid_from" bson:"valid_from"`
	CreatedBy string    `json:"created_by" yaml:"created_by" bson:"created_by"`
	UpdatedBy string    `json:"updated_by" yaml:"updated_by" bson:"updated_by"`
	CreatedAt time.Time `json:"-"          yaml:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"-"          yaml:"updated_at" bson:"updated_at"`
	UV        int64     `json:"uv"         yaml:"uv"         bson:"uv"`
	PV        int64     `json:"pv"         yaml:"pv"         bson:"pv"`
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
