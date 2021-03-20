// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package utils

import (
	"net"
	"net/http"
	"strings"
)

// ReadIP implements a best effort approach to return the real client IP,
// it parses X-Real-IP and X-Forwarded-For in order to work properly with
// reverse-proxies such us: nginx or haproxy. Use X-Forwarded-For before
// X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
//
// This implementation is derived from gin-gonic/gin.
func ReadIP(r *http.Request) string {
	clientIP := r.Header.Get("X-Forwarded-For")
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])
	if clientIP == "" {
		clientIP = strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	}
	if clientIP != "" {
		return clientIP
	}
	if addr := r.Header.Get("X-Appengine-Remote-Addr"); addr != "" {
		return addr
	}
	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return "unknown" // use unknown to guarantee non empty string
	}
	return ip
}
