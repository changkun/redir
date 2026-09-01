// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"changkun.de/x/redir/internal/config"
)

// pseudonymKey is the secret gdpr.hide_ip hashes with.
//
// A plain digest of an address is not pseudonymisation. There are about
// four billion IPv4 addresses, so every one of their digests can be
// computed in minutes and the original read straight back out of a
// table. Keying the hash is what makes the stored value meaningless
// without the key, which is the entire point of the setting.
//
// It comes from the environment because it has to outlive a restart:
// unique visitors are a distinct count over this column, so a key that
// changed would make every returning visitor look new.
var (
	pseudonymOnce sync.Once
	pseudonymKey  []byte
)

func hashKey() []byte {
	pseudonymOnce.Do(func() {
		pseudonymKey = []byte(os.Getenv("REDIR_IP_HASH_KEY"))
		if len(pseudonymKey) == 0 {
			// Failing loudly beats hashing with nothing. A deployment
			// that asked for addresses to be hidden and got a digest
			// anyone can reverse is worse off than one that knows it
			// stores addresses.
			log.Fatalln("gdpr.hide_ip is set but REDIR_IP_HASH_KEY is empty: " +
				"an unkeyed digest of an address can be reversed by " +
				"computing every address, so it would hide nothing")
		}
	})
	return pseudonymKey
}

// ReadIP implements a best effort approach to return the real client IP,
// it parses X-Real-IP and X-Forwarded-For in order to work properly with
// reverse-proxies such us: nginx or haproxy. Use X-Forwarded-For before
// X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
//
// The purpose of this function is to produce an identifier of a visitor.
// Whether it is a real address does not matter; with gdpr.hide_ip it is a
// keyed digest instead, which is why the column that stores it is text.
//
// X-Forwarded-For is set by the client, and it is preferred over the
// address the connection actually came from. That is correct behind a
// reverse proxy that overwrites the header, and it is what redir is
// deployed behind. Exposed directly, a visitor could choose what is
// recorded for them.
//
// This implementation is derived from gin-gonic/gin.
func ReadIP(r *http.Request) (ip string) {
	defer func() {
		if config.Conf.GDPR.HideIP {
			mac := hmac.New(sha256.New, hashKey())
			mac.Write([]byte(ip))
			ip = hex.EncodeToString(mac.Sum(nil))
		}
	}()

	ip = r.Header.Get("X-Forwarded-For")
	ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	if ip == "" {
		ip = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	}
	if ip != "" {
		return ip
	}
	ip = r.Header.Get("X-Appengine-Remote-Addr")
	if ip != "" {
		return ip
	}
	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return "unknown" // use unknown to guarantee non empty string
	}
	return ip
}
