// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { describe, expect, it } from 'vitest'
import { botsExcluded, topN, toChartData } from './stats'

describe('toChartData', () => {
  it('maps the grouped payload onto chart fields', () => {
    expect(toChartData([{ name: 'Chrome', count: 3 }])).toEqual([
      { name: 'Chrome', value: 3 },
    ])
  })

  // The endpoint returns null for an alias with no traffic in range, and
  // fetch failures set the state to an empty array.
  it('survives a missing response', () => {
    expect(toChartData(null)).toEqual([])
    expect(toChartData(undefined)).toEqual([])
    expect(toChartData([])).toEqual([])
  })
})

describe('topN', () => {
  const rows = Array.from({ length: 12 }, (_, i) => ({
    name: `r${i}`,
    count: 12 - i,
  }))

  it('keeps every row when there are few enough', () => {
    expect(topN(rows.slice(0, 3), 8)).toHaveLength(3)
  })

  it('collapses the tail into one bucket without losing visits', () => {
    const got = topN(rows, 8)
    expect(got).toHaveLength(9)
    expect(got[8].name).toBe('Others')
    const before = rows.reduce((s, r) => s + r.count, 0)
    const after = got.reduce((s, d) => s + d.value, 0)
    expect(after).toBe(before)
  })
})

describe('botsExcluded', () => {
  it('says nothing when nothing was excluded', () => {
    expect(botsExcluded({ pv: 0, uv: 0 })).toBe('')
    expect(botsExcluded(null)).toBe('')
    expect(botsExcluded(undefined)).toBe('')
  })

  it('reports what was left out', () => {
    expect(botsExcluded({ pv: 1, uv: 1 })).toContain('1 automated visit')
    expect(botsExcluded({ pv: 23191, uv: 400 })).toContain('23,191 automated visits')
  })
})

// The parser this dashboard used to run in the browser is gone. If it
// comes back, the server-side grouping has been bypassed and every
// distinct user agent is crossing the wire again.
describe('the client no longer parses user agents', () => {
  it('does not depend on a user agent parser', async () => {
    const pkg = await import('../../package.json')
    const deps = Object.keys(pkg.default.dependencies ?? {})
    expect(deps).not.toContain('ua-parser-js')
    expect(deps.filter((d) => /ua|agent|parser/i.test(d))).toEqual([])
  })

  it('is not imported by the stats component', async () => {
    const fs = await import('node:fs/promises')
    const src = await fs.readFile(
      new URL('../components/Stats.jsx', import.meta.url),
      'utf8',
    )
    expect(src).not.toMatch(/ua-parser|UAParser/)
    // The synthetic string the old queries invented is gone from both
    // sides in the same change.
    expect(src).not.toMatch(/'unknown'/)
  })
})
