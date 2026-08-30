// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package config

import (
	"bytes"
	_ "embed"
	"log"
	"os"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

type authType string

var (
	None   authType = "none"
	Basic  authType = "basic"
	Latere authType = "latere"
)

type config struct {
	Title       string `yaml:"title"`
	Host        string `yaml:"host"`
	Addr        string `yaml:"addr"`
	Development bool   `yaml:"development"`
	Store       string `yaml:"store"`
	CORS        bool   `yaml:"cors"`
	S           struct {
		Prefix string `yaml:"prefix"`
	} `yaml:"s"`
	X struct {
		Enable     bool   `yaml:"enable"`
		Prefix     string `yaml:"prefix"`
		VCS        string `yaml:"vcs"`
		ImportPath string `yaml:"import_path"`
		RepoPath   string `yaml:"repo_path"`
		GoDocHost  string `yaml:"godoc_host"`
	} `yaml:"x"`
	Auth struct {
		Enable authType `yaml:"enable"`
		Basic  []struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"basic"`
	} `yaml:"auth"`
	Stats struct {
		Enable bool `yaml:"enable"`
	} `yaml:"stats"`
	GDPR struct {
		HideIP bool `yaml:"hide_ip"`
		Owner  struct {
			Name   string `yaml:"name"`
			Domain string `yaml:"domain"`
		} `yaml:"owner"`
		Contact struct {
			Enable bool   `yaml:"enable"`
			Email  string `yaml:"email"`
		} `yaml:"contact"`
		Impressum struct {
			Enable  bool   `yaml:"enable"`
			Content string `yaml:"content"`
		} `yaml:"impressum"`
		Privacy struct {
			Enable  bool   `yaml:"enable"`
			Content string `yaml:"content"`
		} `yaml:"privacy"`
	} `yaml:"gdpr"`
}

//go:embed config.yml
var defaultConf []byte

func (c *config) parse() {
	// An unset REDIR_CONF means "run on the built-in defaults". A set one
	// names a file the operator expects to be used, so failing to read it
	// is an error: falling back would quietly serve the sample config,
	// which points at another database and re-enables the sample
	// credentials.
	d := defaultConf
	if f := os.Getenv("REDIR_CONF"); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("cannot read REDIR_CONF %v: %v\n", f, err)
		}
		d = b
	}
	if d == nil {
		log.Fatalln("no configuration to read")
	}
	err := yaml.Unmarshal(d, c)
	if err != nil {
		log.Fatalf("cannot parse configuration: %v\n", err)
	}

	// An unrecognised auth mode used to fall through to basic auth, which
	// turns a typo into a silently different login. Report it here;
	// handleAuth then refuses administration. Serving public redirects
	// does not depend on this setting, so it is not worth refusing to
	// start over.
	switch c.Auth.Enable {
	case None, Basic, Latere:
	default:
		log.Printf("unknown auth.enable %q, administration is disabled; want one of: %v, %v, %v",
			c.Auth.Enable, None, Basic, Latere)
	}

	var buf bytes.Buffer
	if err := md.Convert([]byte(Conf.GDPR.Impressum.Content), &buf); err != nil {
		log.Fatalf("cannot parse impressum markdown content: %v\n", err)
	}
	Conf.GDPR.Impressum.Content = buf.String()
	buf.Reset()

	if err := md.Convert([]byte(Conf.GDPR.Privacy.Content), &buf); err != nil {
		log.Fatalf("cannot parse privacy markdown content: %v\n", err)
	}
	Conf.GDPR.Privacy.Content = buf.String()
}

var Conf config

func init() {
	Conf.parse()
}

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithXHTML(),
		html.WithUnsafe(),
	),
)
