// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { describe, expect, it } from 'vitest'
import { nextSort, sortRows, targetLabel } from './format'

describe('targetLabel', () => {
  it('drops the scheme and keeps host and path', () => {
    expect(targetLabel('https://blog.changkun.de/')).toBe('blog.changkun.de')
    expect(targetLabel('https://github.com/golang-design/lockfree')).toBe(
      'github.com/golang-design/lockfree',
    )
  })

  it('drops the query, which is rarely what identifies a link', () => {
    expect(targetLabel('https://youtube.com/watch?v=abc&list=xyz')).toBe(
      'youtube.com/watch',
    )
  })

  // Production holds mailto: targets, one of them obfuscated as
  // "mailto:hi[at]golang.design". There is no host to strip, and
  // mangling it would misrepresent where the link goes.
  it('leaves an address alone', () => {
    expect(targetLabel('mailto:research@changkun.de')).toBe(
      'mailto:research@changkun.de',
    )
    expect(targetLabel('mailto:hi[at]golang.design')).toBe(
      'mailto:hi[at]golang.design',
    )
  })

  it('survives something that is not a URL', () => {
    expect(targetLabel('not a url')).toBe('not a url')
    expect(targetLabel('')).toBe('')
    expect(targetLabel(null)).toBe('')
  })
})

// A column heading is clicked to order the page by it, again to turn the
// order round, and a third time to get back what the server sent.
describe('nextSort', () => {
  it('goes largest first, smallest first, then off', () => {
    const first = nextSort(null, 'pv')
    expect(first).toEqual({ key: 'pv', dir: 'desc' })
    const second = nextSort(first, 'pv')
    expect(second).toEqual({ key: 'pv', dir: 'asc' })
    expect(nextSort(second, 'pv')).toBeNull()
  })

  it('starts over on another column', () => {
    expect(nextSort({ key: 'pv', dir: 'asc' }, 'uv')).toEqual({
      key: 'uv',
      dir: 'desc',
    })
  })
})

describe('sortRows', () => {
  const rows = [
    { alias: 'a', pv: 3 },
    { alias: 'b', pv: 10 },
    { alias: 'c' }, // a link nobody has followed carries no count
  ]

  it('orders by the count, counting a missing one as zero', () => {
    expect(
      sortRows(rows, { key: 'pv', dir: 'desc' }).map((r) => r.alias),
    ).toEqual(['b', 'a', 'c'])
    expect(
      sortRows(rows, { key: 'pv', dir: 'asc' }).map((r) => r.alias),
    ).toEqual(['c', 'a', 'b'])
  })

  it('leaves the rows alone, and returns them as given with nothing to sort by', () => {
    sortRows(rows, { key: 'pv', dir: 'desc' })
    expect(rows.map((r) => r.alias)).toEqual(['a', 'b', 'c'])
    expect(sortRows(rows, null)).toBe(rows)
  })
})
