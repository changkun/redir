// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package models

import (
	"time"
)

// Redir is the core redir model, it records a kind of alias
// and its correlated link.
type Redir struct {
	ID        string    `json:"-"          yaml:"-"`
	Host      string    `json:"-"          yaml:"host"`
	Alias     string    `json:"alias"      yaml:"alias"`
	URL       string    `json:"url"        yaml:"url"`
	Private   bool      `json:"private"    yaml:"private"`
	Trust     bool      `json:"trust"      yaml:"trust"`
	ValidFrom time.Time `json:"valid_from" yaml:"valid_from"`
	CreatedBy string    `json:"created_by" yaml:"created_by"`
	UpdatedBy string    `json:"updated_by" yaml:"updated_by"`
	CreatedAt time.Time `json:"-"          yaml:"created_at"`
	UpdatedAt time.Time `json:"-" yaml:"updated_at"`
}

// RedirIndex is an extension to Redir, which offers more statistic
// information such as PV/UV.
type RedirIndex struct {
	ID        string    `json:"-"          yaml:"-"`
	Host      string    `json:"-"          yaml:"host"`
	Alias     string    `json:"alias"      yaml:"alias"`
	URL       string    `json:"url"        yaml:"url"`
	Private   bool      `json:"private"    yaml:"private"`
	Trust     bool      `json:"trust"      yaml:"trust"`
	ValidFrom time.Time `json:"valid_from" yaml:"valid_from"`
	CreatedBy string    `json:"created_by" yaml:"created_by"`
	UpdatedBy string    `json:"updated_by" yaml:"updated_by"`
	CreatedAt time.Time `json:"-"          yaml:"created_at"`
	UpdatedAt time.Time `json:"-"          yaml:"updated_at"`
	UV        int64     `json:"uv"         yaml:"uv"`
	PV        int64     `json:"pv"         yaml:"pv"`
}

// Visit indicates an record of visit pattern.
//
// The fields below Time are derived from UA and Referer when the visit is
// recorded, so that a query can group by them instead of shipping every
// distinct user agent string to the browser to be parsed there.
type Visit struct {
	VisitorID string    `json:"visitor_id"`
	Host      string    `json:"host"`
	Alias     string    `json:"alias"`
	IP        string    `json:"ip"`
	UA        string    `json:"ua"`
	Referer   string    `json:"referer"`
	Time      time.Time `json:"time"`

	RefererHost string `json:"referer_host"`
	Browser     string `json:"browser"`
	OS          string `json:"os"`
	Device      string `json:"device"`
	IsBot       bool   `json:"is_bot"`
}

// VisitRecord represents the visit record of an alias.
// The record does not contain time range so that the user of this struct
// can customize it.
type VisitRecord struct {
	Alias string `json:"alias"`
	UV    int64  `json:"uv"`
	PV    int64  `json:"pv"`
}

// Referrer statistic
type RefStat struct {
	Referer string `json:"referer"`
	Count   int64  `json:"count"`
}

// UAStat statistics
type UAStat struct {
	UA    string `json:"ua"`
	Count int64  `json:"count"`
}

// TimeHist statistics
type TimeHist struct {
	Time time.Time `json:"time"`
	PV   int       `json:"pv"`
	UV   int       `json:"uv"`
}
