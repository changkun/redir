package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type visit struct {
	Alias   string    `json:"alias"   bson:"alias"`
	IP      string    `json:"ip"      bson:"ip"`
	UA      string    `json:"ua"      bson:"ua"`
	Referer string    `json:"referer" bson:"referer"`
	Time    time.Time `json:"time"    bson:"time"`
}
type redirect struct {
	Alias   string    `json:"alias"   bson:"alias"`
	Kind    aliasKind `json:"kind"    bson:"kind"`
	URL     string    `json:"url"     bson:"url"`
	Private bool      `json:"private" bson:"private"`
}

type aliasKind int

const (
	kindShort aliasKind = iota
	kindRandom
)

// from old redis
type irecord struct {
	IP        string               `json:"ip"`
	Aliases   map[string]time.Time `json:"aliases"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// from old redis
type arecord struct {
	Alias     string    `json:"alias"`
	Kind      aliasKind `json:"kind"`
	URL       string    `json:"url"`
	UV        uint64    `json:"uv"`
	PV        uint64    `json:"pv"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func main() {
	// initialize database connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	db, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://0.0.0.0:27017"))
	if err != nil {
		panic(fmt.Errorf("cannot connect to database: %w", err))
	}

	// restore redirect
	filepath.WalkDir("./dump/alias", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		var r arecord
		err = json.Unmarshal(b, &r)
		if err != nil {
			panic(err)
		}

		rr := &redirect{
			Alias:   r.Alias,
			Kind:    r.Kind,
			URL:     r.URL,
			Private: false,
		}

		col := db.Database("redir").Collection("links")
		_, err = col.InsertOne(ctx, rr)
		if err != nil {
			err = fmt.Errorf("failed to insert record: %w", err)
			panic(err)
		}
		return nil
	})

	// restore visits
	filepath.WalkDir("./dump/ips", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		var r irecord
		err = json.Unmarshal(b, &r)
		if err != nil {
			panic(err)
		}

		for a, t := range r.Aliases {
			v := visit{
				Alias:   a,
				IP:      r.IP,
				UA:      "",
				Referer: "",
				Time:    t,
			}
			col := db.Database("redir").Collection("visit")
			_, err = col.InsertOne(ctx, v)
			if err != nil {
				err = fmt.Errorf("failed to insert record: %w", err)
				panic(err)
			}
		}

		return nil
	})
}
