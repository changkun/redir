// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	_ "embed"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type config struct {
	Title string `yaml:"title"`
	Host  string `yaml:"host"`
	Addr  string `yaml:"addr"`
	Store string `yaml:"store"`
	S     struct {
		Prefix string `yaml:"prefix"`
	} `yaml:"s"`
	R struct {
		Length int    `yaml:"length"`
		Prefix string `yaml:"prefix"`
	} `yaml:"r"`
	X struct {
		Prefix     string `yaml:"prefix"`
		VCS        string `yaml:"vcs"`
		ImportPath string `yaml:"import_path"`
		RepoPath   string `yaml:"repo_path"`
		GoDocHost  string `yaml:"godoc_host"`
	} `yaml:"x"`
}

//go:embed config.yml
var defaultConf []byte

func (c *config) parse() {
	f := os.Getenv("REDIR_CONF")
	d, err := os.ReadFile(f)
	if err != nil {
		// Just try again with default setting.
		d = defaultConf
		if d == nil {
			log.Fatalf("cannot read configuration: %v\n", err)
		}
	}
	err = yaml.Unmarshal(d, c)
	if err != nil {
		log.Fatalf("cannot parse configuration: %v\n", err)
	}
}

var conf config

func init() {
	conf.parse()
}
