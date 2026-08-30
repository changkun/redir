// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	Conf.parse()

	// Test if all fields are filled.
	v := reflect.ValueOf(Conf)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.Struct {
			continue
		}
		if v.Field(i).Interface() != nil {
			continue
		}
		t.Fatalf("read empty from config, field: %v", v.Type().Field(i).Name)
	}
}

// TestMissingConfigIsFatal pins that a REDIR_CONF naming a file that
// cannot be read stops the server. The configuration is mounted into the
// container, so a wrong path is a realistic mistake, and falling back to
// the embedded sample would quietly point the deployment at a different
// database and re-enable the sample credentials.
func TestMissingConfigIsFatal(t *testing.T) {
	if os.Getenv("REDIR_TEST_SUBPROCESS") == "1" {
		// parse() already ran from init. Reaching here means it did not
		// stop, which the parent reports as a failure.
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestMissingConfigIsFatal$")
	cmd.Env = append(os.Environ(),
		"REDIR_TEST_SUBPROCESS=1",
		"REDIR_CONF="+filepath.Join(t.TempDir(), "absent.yml"),
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("an unreadable REDIR_CONF did not stop startup, output:\n%s", out)
	}
	if !strings.Contains(string(out), "cannot read REDIR_CONF") {
		t.Errorf("want a message naming REDIR_CONF, got:\n%s", out)
	}
}
