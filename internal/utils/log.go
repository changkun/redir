package utils

import (
	"log"
	"net/http"

	"changkun.de/x/redir/internal/config"
)

// Logging wraps an http handler and returns a new handler that prints
// request log.
func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if config.Conf.Stats.HideIP {
					log.Println(r.Method, r.URL.Path, r.URL.RawQuery)
				} else {
					log.Println(ReadIP(r), r.Method, r.URL.Path, r.URL.RawQuery)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
