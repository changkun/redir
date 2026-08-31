// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package config_test

import (
	"testing"

	"changkun.de/x/redir/internal/config"
)

// TestNormalizeHost pins the key the store uses. Two spellings of the same
// site must not become two tenants: a link created through one would be
// invisible through the other.
func TestNormalizeHost(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"https://changkun.de", "changkun.de"},
		{"https://changkun.de/", "changkun.de"},
		{"changkun.de", "changkun.de"},
		{"changkun.de:443", "changkun.de"},
		{"CHANGKUN.DE", "changkun.de"},
		{"changkun.de.", "changkun.de"},
		{"redir:80", "redir"},
		{"localhost:9123", "localhost"},
		{"[::1]:9123", "::1"},
		{"https://golang.design/s/", "golang.design"},
		{"", ""},
	} {
		if got := config.NormalizeHost(tt.in); got != tt.want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
