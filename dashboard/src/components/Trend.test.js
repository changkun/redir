// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { describe, expect, it } from 'vitest'
import { axis, dayLabel } from './Trend'

// The scale is read against the line, so its gridlines have to sit on
// numbers a person divides in their head, and they have to be whole,
// because what is counted is visits.
describe('axis', () => {
  it('steps by a round, whole number', () => {
    expect(axis(7)).toEqual({ top: 8, step: 2 })
    expect(axis(12)).toEqual({ top: 15, step: 5 })
    expect(axis(64)).toEqual({ top: 80, step: 20 })
    expect(axis(105)).toEqual({ top: 150, step: 50 })
    expect(axis(437)).toEqual({ top: 600, step: 200 })
    expect(axis(4300)).toEqual({ top: 6000, step: 2000 })
  })

  it('never puts the top below the data, and keeps the lines few', () => {
    for (const v of [1, 3, 5, 9, 10, 11, 99, 100, 101, 4999, 31907]) {
      const { top, step } = axis(v)
      expect(top).toBeGreaterThanOrEqual(v)
      expect(Number.isInteger(step)).toBe(true)
      expect(top / step).toBeLessThanOrEqual(5)
    }
  })

  // An empty range divides by the top, so it must not be zero.
  it('never returns zero', () => {
    expect(axis(0)).toEqual({ top: 4, step: 1 })
    expect(axis(-5)).toEqual({ top: 4, step: 1 })
    expect(axis(NaN)).toEqual({ top: 4, step: 1 })
  })
})

// A series is keyed by calendar days. The label must name that day in
// every time zone, so it is built from the parts and never parsed as an
// instant.
describe('dayLabel', () => {
  it('names the day the key names', () => {
    expect(dayLabel('2026-09-05')).toBe('Sep 5')
    expect(dayLabel('2026-01-01', true)).toBe('Jan 1, 2026')
    expect(dayLabel('2026-12-31')).toBe('Dec 31')
  })

  it('returns what it cannot read', () => {
    expect(dayLabel('soon')).toBe('soon')
  })
})
