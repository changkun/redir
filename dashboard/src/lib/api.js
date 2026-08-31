// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { basePath } from './paths'

// origin is where the API lives. In development the dashboard is served
// by Vite and the server is elsewhere, so the two are not the same.
const origin = (devMode) =>
  devMode ? 'http://localhost:9123/s' : basePath()

const post = async (body) => {
  const resp = await fetch(basePath() + '/', {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (resp.ok) {
    return null
  }
  // The server answers a refusal with a message worth showing; a
  // transport failure has none, so say which one happened.
  try {
    const data = await resp.json()
    return data.message || `request failed (${resp.status})`
  } catch {
    return `request failed (${resp.status})`
  }
}

const get = async (devMode, params) => {
  const resp = await fetch(`${origin(devMode)}/?${new URLSearchParams(params)}`)
  if (!resp.ok) {
    throw new Error(`request failed (${resp.status})`)
  }
  return resp.json()
}

// fetchIndex returns a page of links. The admin listing carries the
// counts and a series per row; the public one carries neither.
export const fetchIndex = async ({ isAdmin, devMode, page, pageSize }) => {
  const data = await get(devMode, {
    mode: isAdmin ? 'index-pro' : 'index',
    pn: page,
    ps: pageSize,
  })
  const rows = (data.data ?? []).map((r) => ({
    ...r,
    // The zero time means "valid since always". Carrying it into a date
    // field would render the year 1 and offer to edit it.
    valid_from: r.valid_from === '0001-01-01T00:00:00Z' ? null : r.valid_from,
  }))
  return { ...data, data: rows }
}

export const fetchOverview = (devMode, t0, t1) =>
  get(devMode, { mode: 'overview', t0, t1 })

export const fetchStat = (devMode, alias, stat, t0, t1) =>
  get(devMode, { mode: 'stats', a: alias, stat, t0, t1 })

export const save = (op, alias, data) => post({ op, alias, data })

export const remove = (alias) => post({ op: 'delete', alias })
