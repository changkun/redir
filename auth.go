// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"changkun.de/x/login"
	"changkun.de/x/redir/internal/config"
	"changkun.de/x/redir/internal/utils"
)

var errUnauthorized = errors.New("request unauthorized")

// blocklist holds the ip that should be blocked for further requests.
//
// This map may keep grow without releasing memory because of
// continuously attempts. we also do not persist this type of block info
// to the disk, which means if we reboot the service then all the blocker
// are gone and they can attack the server again.
// We clear the map very month.
var blocklist sync.Map // map[string]*blockinfo{}

func init() {
	t := time.NewTicker(time.Hour * 24 * 30)
	go func() {
		for range t.C {
			blocklist.Range(func(k, v interface{}) bool {
				blocklist.Delete(k)
				return true
			})
		}
	}()
}

type blockinfo struct {
	failCount int64
	lastFail  atomic.Value // time.Time
	blockTime atomic.Value // time.Duration
}

const maxFailureAttempts = 3

func (s *server) handleAuth(w http.ResponseWriter, r *http.Request) (user string, err error) {
	switch config.Conf.Auth.Enable {
	case config.None:
		return
	case config.SSO:
		user, err := login.HandleAuth(w, r)
		if err != nil {
			uu, _ := url.Parse(config.Conf.Auth.SSO)
			q := uu.Query()
			q.Set("redirect", "https://"+r.Host+r.URL.String())
			uu.RawQuery = q.Encode()
			http.Redirect(w, r, uu.String(), http.StatusFound)
		}
		return user, err
	case config.Basic:
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="redir"`)

	u, p, ok := r.BasicAuth()
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		err = fmt.Errorf("%w: failed to parsing basic auth", errUnauthorized)
		return
	}

	// check if the IP failure attempts are too much
	// if so, direct abort the request without checking credentials
	ip := utils.ReadIP(r)
	if i, ok := blocklist.Load(ip); ok {
		info := i.(*blockinfo)
		count := atomic.LoadInt64(&info.failCount)
		if count > maxFailureAttempts {
			// if the ip is under block, then directly abort
			last := info.lastFail.Load().(time.Time)
			bloc := info.blockTime.Load().(time.Duration)

			if time.Now().UTC().Sub(last.Add(bloc)) < 0 {
				log.Printf("block ip %v, too much failure attempts. Block time: %v, release until: %v\n",
					ip, bloc, last.Add(bloc))
				err = fmt.Errorf("%w: too much failure attempts", errUnauthorized)
				return
			}

			// clear the failcount, but increase the next block time
			atomic.StoreInt64(&info.failCount, 0)
			info.blockTime.Store(bloc * 2)
		}
	}

	defer func() {
		if !errors.Is(err, errUnauthorized) {
			return
		}

		if i, ok := blocklist.Load(ip); !ok {
			info := &blockinfo{
				failCount: 1,
			}
			info.lastFail.Store(time.Now().UTC())
			info.blockTime.Store(time.Second * 10)

			blocklist.Store(ip, info)
		} else {
			info := i.(*blockinfo)
			atomic.AddInt64(&info.failCount, 1)
			info.lastFail.Store(time.Now().UTC())
		}
	}()

	found := false
	for _, account := range config.Conf.Auth.Basic {
		if u == account.Username && p == account.Password {
			found = true
			break
		}
	}
	if !found {
		w.WriteHeader(http.StatusUnauthorized)
		return "", fmt.Errorf("%w: username or password is invalid", errUnauthorized)
	}
	return u, nil
}
