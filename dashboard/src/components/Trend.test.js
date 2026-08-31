// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { describe, expect, it } from 'vitest'
import { niceCeiling } from './Trend'

// The axis maximum is read against the line, so it has to be a number a
// person divides in their head. It also has to be stable: a ceiling that
// tracked the data exactly would move on every refresh and make the line
// appear to change shape when only the scale did.
describe('niceCeiling', () => {
  it('rounds up to 1, 2 or 5 times a power of ten', () => {
    expect(niceCeiling(1)).toBe(1)
    expect(niceCeiling(7)).toBe(10)
    expect(niceCeiling(12)).toBe(20)
    expect(niceCeiling(31)).toBe(50)
    expect(niceCeiling(64)).toBe(100)
    expect(niceCeiling(4300)).toBe(5000)
  })

  it('never returns less than the data', () => {
    for (const v of [1, 3, 9, 10, 11, 99, 100, 101, 4999, 31907]) {
      expect(niceCeiling(v)).toBeGreaterThanOrEqual(v)
    }
  })

  // An empty range divides by the ceiling, so it must not be zero.
  it('never returns zero', () => {
    expect(niceCeiling(0)).toBe(1)
    expect(niceCeiling(-5)).toBe(1)
  })
})
