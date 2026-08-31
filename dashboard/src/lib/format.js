// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// targetLabel strips a URL down to what identifies it at a glance.
//
// The column is for scanning down, so it shows host and path and drops
// the scheme, the query and the fragment. The full target is on hover and
// in the editor, and nothing here is the only place it can be read.
export const targetLabel = (url) => {
  if (!url) return ''
  try {
    const u = new URL(url)
    if (!u.host) {
      // mailto: and similar carry everything in the opaque part, so
      // there is nothing to strip.
      return url
    }
    const path = u.pathname === '/' ? '' : u.pathname
    return u.host + path
  } catch {
    return url
  }
}
