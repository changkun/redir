// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package utils

import "math/rand/v2"

const alphanum = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Randstr generates a random string of n characters drawn from alphanum.
//
// The top-level math/rand/v2 generator is seeded from the runtime and is
// safe for concurrent use, so no pooled source is needed here.
func Randstr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphanum[rand.IntN(len(alphanum))]
	}
	return string(b)
}
