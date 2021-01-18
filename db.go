// Copyright 2020 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	prefixalias = "redir:alias:"
	prefixip    = "redir:ip:"
)

var (
	errExistedAlias = errors.New("alias is existed")
)

type aliasKind int

const (
	kindShort aliasKind = iota
	kindRandom
)

// arecord indicates an alias record that stores an short alias
// in data store with statistics regarding its UVs and PVs.
type arecord struct {
	Alias     string    `json:"alias"`
	Kind      aliasKind `json:"kind"`
	URL       string    `json:"url"`
	UV        uint64    `json:"uv"`
	PV        uint64    `json:"pv"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// irecord indicates an ip record that stores a specific ip and
// its visit history
type irecord struct {
	IP        string               `json:"ip"`
	Aliases   map[string]time.Time `json:"aliases"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// store is a data store that records short aliases and its corresponding
// uv and pv. The data store format can be considered as follows:
//
// aliases {"redir:alias:x": {"url":"link", "uv_count": 1, "pv_count": 1}}}
// ips     {"redir:ip:ip1": [alias1, alias2, ...], "redir:ip:ip2": [], ...}
type store struct {
	db *redis.Client
}

func newStore(url string) (*store, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to create opt store: %w", err)
	}
	return &store{db: redis.NewClient(opt)}, nil
}

func (s *store) Close() (err error) {
	err = s.db.Close()
	if err != nil {
		err = fmt.Errorf("failed to close data store: %w", err)
	}
	return
}

// StoreAlias stores a given short alias with the given link if not exists
func (s *store) StoreAlias(ctx context.Context, a, l string, kind aliasKind) (err error) {
	b, err := json.Marshal(&arecord{
		URL: l, Kind: kind, Alias: a, PV: 0, UV: 0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		err = fmt.Errorf("failed to create new alias: %w", err)
		return
	}
	ok, err := s.db.SetNX(ctx, prefixalias+a, b, 0).Result()
	if err != nil {
		err = fmt.Errorf("failed to store the new record: %w", err)
		return
	}
	if !ok {
		err = errExistedAlias
		return
	}
	return
}

// UpdateAlias updates the link of a given alias
func (s *store) UpdateAlias(ctx context.Context, a, l string) (err error) {
	return s.readAndUpdate(ctx, prefixalias, a, func(old []byte) ([]byte, error) {
		record := arecord{}
		err = json.Unmarshal(old, &record)
		if err != nil {
			return nil, fmt.Errorf("corrupted data store: %w", err)
		}
		record.URL = l
		record.UpdatedAt = time.Now().UTC()
		updated, err := json.Marshal(record)
		if err != nil {
			return nil, fmt.Errorf("unable to encode data: %w", err)
		}
		return updated, nil
	})
}

// Delete deletes a given short alias if exists
func (s *store) DeleteAlias(ctx context.Context, a string) (err error) {
	return s.delete(ctx, prefixalias+a)
}

// FetchAlias reads a given alias
func (s *store) FetchAlias(ctx context.Context, a string) (string, error) {
	return s.Fetch(ctx, prefixalias+a)
}

// StoreIP stores a given new ip
func (s *store) StoreIP(ctx context.Context, ip, alias string) (err error) {
	now := time.Now().UTC()
	b, err := json.Marshal(&irecord{
		IP:        ip,
		Aliases:   map[string]time.Time{alias: now},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		err = fmt.Errorf("failed to create new ip: %w", err)
		return
	}
	ok, err := s.db.SetNX(ctx, prefixip+ip, b, 0).Result()
	if err != nil {
		err = fmt.Errorf("failed to store the new ip: %w", err)
		return
	}
	if !ok {
		err = errors.New("ip already exists in data store")
		return
	}

	s.countVisit(ctx, alias, 1, 1)
	return
}

// UpdateIP updates the visiting history of a given IP
func (s *store) UpdateIP(ctx context.Context, ip, alias string) (err error) {
	visited := false
	err = s.readAndUpdate(ctx, prefixip, ip, func(old []byte) ([]byte, error) {
		record := irecord{}
		err := json.Unmarshal(old, &record)
		if err != nil {
			return nil, fmt.Errorf("corrupted data store: %w", err)
		}

		now := time.Now().UTC()
		if _, ok := record.Aliases[alias]; ok {
			visited = true
		} else {
			record.Aliases[alias] = now
			record.UpdatedAt = now
		}

		updated, err := json.Marshal(record)
		if err != nil {
			return nil, fmt.Errorf("unable to encode data: %w", err)
		}
		return updated, nil
	})

	if visited {
		s.countVisit(ctx, alias, 1, 0)
	} else {
		s.countVisit(ctx, alias, 1, 1)
	}
	return
}

// FetchIP reads a given ip
func (s *store) FetchIP(ctx context.Context, ip string) (string, error) {
	return s.Fetch(ctx, prefixip+ip)
}

// delete deletes a given short alias if exists
func (s *store) deleteIP(ctx context.Context, ip string) (err error) {
	return s.delete(ctx, prefixip+ip)
}

func (s *store) delete(ctx context.Context, key string) (err error) {
	_, err = s.db.Del(ctx, key).Result()
	if err != nil {
		err = fmt.Errorf("failed to delete the alias: %w", err)
		return
	}
	return
}

func (s *store) countVisit(ctx context.Context, alias string, pv, uv int) error {
	return s.readAndUpdate(ctx, prefixalias, alias, func(old []byte) ([]byte, error) {
		record := arecord{}
		err := json.Unmarshal(old, &record)
		if err != nil {
			return nil, fmt.Errorf("corrupted data store: %w", err)
		}
		record.PV += uint64(pv)
		record.UV += uint64(uv)
		record.UpdatedAt = time.Now().UTC()
		updated, err := json.Marshal(record)
		if err != nil {
			return nil, fmt.Errorf("unable to encode data: %w", err)
		}
		return updated, nil
	})
}

// Fetch fetches a given short alias, returns a json marshared string if exists
func (s *store) Fetch(ctx context.Context, key string) (r string, err error) {
	r, err = s.db.Get(ctx, key).Result()
	if err != nil {
		err = fmt.Errorf("failed to read the alias: %w", err)
	}
	return
}

// Keys returns all keys with given prefix
func (s *store) Keys(ctx context.Context, prefix string) (r []string, err error) {
	r, err = s.db.Keys(ctx, prefix).Result()
	if err != nil {
		err = fmt.Errorf("failed to read the alias: %w", err)
	}
	return
}

// readAndUpdate updates a given key atomically, the updatef callback
// will provide the read data to its implementer.
func (s *store) readAndUpdate(ctx context.Context, prefix, key string, updatef func([]byte) ([]byte, error)) (err error) {
	k := prefix + key
	txupdate := func(tx *redis.Tx) error {
		data, err := tx.Get(ctx, k).Bytes()
		if err != nil {
			return fmt.Errorf("readAndUpdate failed in fetch phase: %w", err)
		}

		updated, err := updatef(data)
		if err != nil {
			return fmt.Errorf("readAndUpdate failed in update phase: %w", err)
		}

		// Commited only if watched key remain unchanged
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, k, updated, 0)
			return nil
		})
		return err
	}

	// Keep trying until it done or canceled.
	//
	// No guarantee in total order. This means if there are two or more
	// concurrent updates, the behavior is undefined.
	for {
		select {
		case <-ctx.Done():
			return errors.New("readAndUpdate is canceled")
		default:
			err := s.db.Watch(ctx, txupdate, k)
			if err == nil {
				return nil
			}
			if errors.Is(err, redis.TxFailedErr) {
				continue
			}
			return err
		}
	}
}
