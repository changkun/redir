// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { describe, expect, it } from 'vitest'
import { day, dayBuckets, defaultRange, rfc3339 } from './time'

describe('defaultRange', () => {
  // The start day was derived from the *end* day's day-of-month, so the
  // window slipped by a whole month whenever the two fell in different
  // months. On 31 January the range began on 2 December.
  it('spans the 30 days before now across a month boundary', () => {
    expect(defaultRange(new Date(2026, 0, 31, 12))).toEqual([
      '2026-01-01',
      '2026-02-01',
    ])
  })

  it('ends tomorrow so today is included', () => {
    expect(defaultRange(new Date(2026, 5, 15, 12))).toEqual([
      '2026-05-16',
      '2026-06-16',
    ])
  })
})

describe('day', () => {
  it('pads month and day', () => {
    expect(day(new Date(2026, 0, 5, 12))).toBe('2026-01-05')
  })
})

describe('dayBuckets', () => {
  it('covers both ends of the range', () => {
    expect(dayBuckets('2026-03-01', '2026-03-04')).toEqual({
      '2026-03-01': 0,
      '2026-03-02': 0,
      '2026-03-03': 0,
      '2026-03-04': 0,
    })
  })
})

describe('rfc3339', () => {
  it('returns null for an empty value', () => {
    expect(rfc3339('')).toBeNull()
    expect(rfc3339(null)).toBeNull()
    expect(rfc3339(undefined)).toBeNull()
  })

  it('delegates to a date picker value', () => {
    expect(rfc3339({ format: () => '2026-03-01T10:00:00+01:00' })).toBe(
      '2026-03-01T10:00:00+01:00',
    )
  })

  it('formats a string date with a zone offset', () => {
    expect(rfc3339('2026-03-01T10:00:00')).toMatch(
      /^2026-03-01T10:00:00(Z|[+-]\d{2}:\d{2})$/,
    )
  })
})
