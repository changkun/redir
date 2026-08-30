// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"io/fs"
	"path"
	"strings"
	"testing"
)

// TestDashboardEmbedContract pins the agreement between the dashboard
// bundler and the server: index.html keeps the Go template placeholders,
// its asset URLs live under /static so init can rewrite them to the
// ./.static route, and the embedded tree actually holds those assets.
// A bundler or config change that moves the output breaks this here
// rather than at runtime on an empty page.
func TestDashboardEmbedContract(t *testing.T) {
	placeholders := []string{
		"AdminView", "StatsMode", "DevMode",
		"ShowImpressum", "ShowPrivacy", "ShowContact", "LogoutURL",
	}
	for _, p := range placeholders {
		if want := "{{." + p + "}}"; !strings.Contains(dtmpl, want) {
			t.Errorf("dashboard index.html is missing the %s placeholder", want)
		}
	}

	// init rewrote /static to ./.static, so the rewritten form is what
	// proves the assets were emitted where the server expects them.
	if !strings.Contains(dtmpl, "./.static/") {
		t.Errorf("dashboard index.html references no ./.static asset:\n%s", dtmpl)
	}
	if strings.Contains(dtmpl, `"/static/`) {
		t.Errorf("dashboard index.html still holds an unrewritten /static reference")
	}

	assets, err := fs.Sub(sasse, "dashboard/build/static")
	if err != nil {
		t.Fatalf("cannot open the embedded static tree: %v", err)
	}
	exts := map[string]bool{}
	err = fs.WalkDir(assets, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			exts[path.Ext(p)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cannot walk the embedded static tree: %v", err)
	}
	for _, ext := range []string{".js", ".css"} {
		if !exts[ext] {
			t.Errorf("embedded static tree holds no %s asset, got %v", ext, exts)
		}
	}
}
