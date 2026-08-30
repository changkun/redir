// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// basePath returns the path the dashboard is mounted on, without its
// trailing slash, e.g. "/s" for a dashboard served at "/s/".
export const basePath = () => {
  const p = window.location.pathname
  return p.endsWith('/') ? p.slice(0, -1) : p
}

// aliasPath returns the server-relative short link for an alias.
export const aliasPath = (alias) => `${basePath()}/${alias}`

// aliasURL returns the absolute short link for an alias, which is what a
// visitor needs when the link is copied out of the dashboard.
export const aliasURL = (alias) =>
  `${window.location.origin}${aliasPath(alias)}`
