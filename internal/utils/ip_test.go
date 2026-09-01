// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package utils_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/utils"
)

// req builds a request with the given headers and remote address.
func req(remote string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/s/x", nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

// TestReadIPPrecedence pins which source wins.
//
// The value this returns becomes the visits table's ip column, and UV is
// a distinct count over it, so the order decides whether a reverse proxy
// counts as one visitor or the people behind it count as many.
func TestReadIPPrecedence(t *testing.T) {
	restore(t, false)

	for _, tt := range []struct {
		name    string
		remote  string
		headers map[string]string
		want    string
	}{
		{
			name:    "x-forwarded-for wins",
			remote:  "10.0.0.1:5555",
			headers: map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-Ip": "9.9.9.9"},
			want:    "1.2.3.4",
		},
		{
			// A proxy chain appends, so the client is first and the
			// proxies that carried it follow.
			name:    "first entry of a chain",
			remote:  "10.0.0.1:5555",
			headers: map[string]string{"X-Forwarded-For": "1.2.3.4, 10.0.0.9, 10.0.0.8"},
			want:    "1.2.3.4",
		},
		{
			name:    "whitespace is trimmed",
			remote:  "10.0.0.1:5555",
			headers: map[string]string{"X-Forwarded-For": "  1.2.3.4  , 10.0.0.9"},
			want:    "1.2.3.4",
		},
		{
			// nginx sets X-Real-Ip to the proxy's own address, which is
			// why it is consulted only after X-Forwarded-For.
			name:    "x-real-ip when there is no forwarded-for",
			remote:  "10.0.0.1:5555",
			headers: map[string]string{"X-Real-Ip": "9.9.9.9"},
			want:    "9.9.9.9",
		},
		{
			name:    "app engine header",
			remote:  "10.0.0.1:5555",
			headers: map[string]string{"X-Appengine-Remote-Addr": "8.8.8.8"},
			want:    "8.8.8.8",
		},
		{
			name:   "the connection itself, without its port",
			remote: "203.0.113.7:41234",
			want:   "203.0.113.7",
		},
		{
			name:   "ipv6 keeps its address and loses its port",
			remote: "[2001:db8::1]:41234",
			want:   "2001:db8::1",
		},
		{
			// A non-empty string is guaranteed, because the column is
			// NOT NULL and an empty value would group with every other
			// unreadable request.
			name:   "unreadable remote address",
			remote: "not-an-address",
			want:   "unknown",
		},
		{
			name:    "an empty header falls through rather than winning",
			remote:  "203.0.113.7:41234",
			headers: map[string]string{"X-Forwarded-For": "", "X-Real-Ip": ""},
			want:    "203.0.113.7",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.ReadIP(req(tt.remote, tt.headers)); got != tt.want {
				t.Errorf("ReadIP = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestReadIPHidden covers gdpr.hide_ip.
//
// The stored value becomes a keyed digest, which is why the column is
// text and not inet: a digest is not an address, and neither is
// "unknown".
func TestReadIPHidden(t *testing.T) {
	restore(t, true)

	got := utils.ReadIP(req("10.0.0.1:5555", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	}))

	if got == "1.2.3.4" {
		t.Fatal("the address was stored as it is")
	}
	if len(got) != 64 {
		t.Errorf("digest is %d characters, want 64", len(got))
	}
	if want := keyed(t, "1.2.3.4"); got != want {
		t.Fatalf("ReadIP = %q, want %q", got, want)
	}
}

// TestReadIPHiddenIsKeyed is the property that makes hide_ip mean
// something.
//
// An unkeyed digest of an address is not pseudonymisation: there are
// about four billion IPv4 addresses, so anyone can compute every digest
// and read the original back out of a table. A digest that matches the
// plain one would be exactly that.
func TestReadIPHiddenIsKeyed(t *testing.T) {
	restore(t, true)

	got := utils.ReadIP(req("10.0.0.1:1", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	}))

	plain := sha256.Sum256([]byte("1.2.3.4"))
	if got == hex.EncodeToString(plain[:]) {
		t.Fatal("the digest is unkeyed, so every address can be enumerated " +
			"and this hides nothing")
	}
}

// keyed reproduces the digest independently of the implementation.
func keyed(t *testing.T, ip string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(os.Getenv("REDIR_IP_HASH_KEY")))
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}

// TestReadIPHiddenIsStable is what makes UV work under hide_ip. The count
// is distinct over this column, so one visitor has to produce one value.
func TestReadIPHiddenIsStable(t *testing.T) {
	restore(t, true)

	a := utils.ReadIP(req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "1.2.3.4"}))
	b := utils.ReadIP(req("10.0.0.2:2", map[string]string{"X-Forwarded-For": "1.2.3.4"}))
	c := utils.ReadIP(req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "1.2.3.5"}))

	if a != b {
		t.Error("the same address produced two values, so one visitor would count as two")
	}
	if a == c {
		t.Error("two addresses produced one value, so two visitors would count as one")
	}
}

// TestReadIPHidesTheFallbackToo checks that "unknown" is hashed like
// anything else. It is not an address, which is the reason the column
// cannot be inet.
func TestReadIPHiddenCoversUnknown(t *testing.T) {
	restore(t, true)

	got := utils.ReadIP(req("not-an-address", nil))
	if want := keyed(t, "unknown"); got != want {
		t.Fatalf("ReadIP = %q, want the digest of \"unknown\"", got)
	}
}

// TestReadIPTrustsItsProxy documents an assumption rather than a
// behaviour worth having.
//
// X-Forwarded-For is set by the client, and redir takes it over the
// address it is actually talking to. Behind a reverse proxy that
// overwrites the header, that is right and necessary. Exposed directly,
// it means anyone can choose what gets recorded as their address, and so
// can inflate or split the unique-visitor count at will.
//
// redir is deployed behind traefik, which sets the header, so this holds
// there. The test exists so the assumption is written down where someone
// changing the deployment will see it.
func TestReadIPTrustsItsProxy(t *testing.T) {
	restore(t, false)

	got := utils.ReadIP(req("203.0.113.7:41234", map[string]string{
		"X-Forwarded-For": "0.0.0.0",
	}))
	if got != "0.0.0.0" {
		t.Fatalf("ReadIP = %q; the header is taken over the connection, "+
			"which only holds behind a proxy that sets it", got)
	}
}

// TestMain gives the package a hashing key. A deployment supplies one
// through the environment, and refuses to start with hide_ip set and no
// key rather than hashing with nothing.
func TestMain(m *testing.M) {
	os.Setenv("REDIR_IP_HASH_KEY", "test-key-not-a-real-secret")
	os.Exit(m.Run())
}

// restore sets hide_ip for one test and puts it back afterwards, since
// the configuration is a package-level value.
func restore(t *testing.T, hide bool) {
	t.Helper()
	saved := config.Conf.GDPR.HideIP
	config.Conf.GDPR.HideIP = hide
	t.Cleanup(func() { config.Conf.GDPR.HideIP = saved })
}
