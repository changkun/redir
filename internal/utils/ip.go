// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package utils

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"changkun.de/x/redir/internal/config"
)

// ReadIP implements a best effort approach to return the real client IP,
// it parses X-Real-IP and X-Forwarded-For in order to work properly with
// reverse-proxies such us: nginx or haproxy. Use X-Forwarded-For before
// X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
//
// The purpose of this function is to produce an identifier of visitor.
// It does not matter wheather it is an real IP or not. Depending on the
// configuration, the returned IP address might be an encrypted hash string.
//
// This implementation is derived from gin-gonic/gin.
func ReadIP(r *http.Request) (ip string) {
	defer func() {
		if config.Conf.GDPR.HideIP {
			hh := sha1.New()
			io.WriteString(hh, ip)
			ip = fmt.Sprintf("%x", hh.Sum(nil))
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
