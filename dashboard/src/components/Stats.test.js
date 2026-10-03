// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { describe, expect, it } from 'vitest'
import { toSeries } from './Stats'

// A link's detail asks for a range that ends tomorrow, so that today is
// inside it. The chart draws a day for each one that has begun.
describe('toSeries', () => {
  const hours = [
    { time: '2026-10-01T09:00:00', pv: 2, uv: 1 },
    { time: '2026-10-01T17:00:00', pv: 3, uv: 2 },
    { time: '2026-10-03T08:00:00', pv: 4, uv: 4 },
  ]

  it('adds the hours of a day together and keeps a quiet day as zero', () => {
    const s = toSeries(hours, '2026-10-01', '2026-10-03', '2026-10-03')
    expect(s.labels).toEqual(['2026-10-01', '2026-10-02', '2026-10-03'])
    expect(s.pv).toEqual([5, 0, 4])
    expect(s.uv).toEqual([3, 0, 4])
  })

  it('does not draw a day that has not begun', () => {
    const s = toSeries(hours, '2026-10-01', '2026-10-04', '2026-10-03')
    expect(s.labels.at(-1)).toBe('2026-10-03')
    expect(s.pv).toHaveLength(3)
  })
})
