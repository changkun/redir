// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package utils_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"changkun.de/x/redir/internal/utils"
)

// TestLoggingOmitsAddressesWhenHidden checks the access log honours
// gdpr.hide_ip.
//
// Hashing what goes into the database while writing the address into the
// log beside it would defeat the setting: the log is the easier of the
// two to read.
func TestLoggingOmitsAddressesWhenHidden(t *testing.T) {
	for _, tt := range []struct {
		name     string
		hide     bool
		wantAddr bool
	}{
		{"addresses are logged by default", false, true},
		{"and left out when they are hidden", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			restore(t, tt.hide)

			var buf bytes.Buffer
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(nil) })

			var served bool
			h := utils.Logging()(http.HandlerFunc(
				func(http.ResponseWriter, *http.Request) { served = true }))

			r := httptest.NewRequest(http.MethodGet, "/s/blog?mode=admin", nil)
			r.RemoteAddr = "203.0.113.7:41234"
			h.ServeHTTP(httptest.NewRecorder(), r)

			if !served {
				t.Fatal("the wrapped handler was not called")
			}
			out := buf.String()
			if got := strings.Contains(out, "203.0.113.7"); got != tt.wantAddr {
				t.Errorf("address in the log = %v, want %v: %q", got, tt.wantAddr, out)
			}
			// The request itself is logged either way, or the log stops
			// being useful for anything.
			for _, want := range []string{"GET", "/s/blog", "mode=admin"} {
				if !strings.Contains(out, want) {
					t.Errorf("log is missing %q: %q", want, out)
				}
			}
		})
	}
}
