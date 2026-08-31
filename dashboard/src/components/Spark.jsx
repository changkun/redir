// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { tokens } from '../theme'

// Spark draws a row's recent traffic as bars.
//
// It is hand-drawn SVG rather than a charting library: fourteen bars with
// no axes, no legend and no interaction do not justify a rendering engine,
// and one of these is drawn per row.
//
// The scale is per row on purpose. A sparkline answers "is this link busy
// lately, and when", not "is it busier than the row above"; the PV column
// beside it already answers that, and a shared scale would flatten every
// quiet row into a straight line.
const Spark = ({ data = [], width = 84, height = 18, title }) => {
  if (!data.length) {
    // A link with no traffic gets a baseline rather than nothing, so the
    // column keeps its rhythm down the page.
    return (
      <svg width={width} height={height} role="img" aria-label="no traffic">
        <line
          x1="0"
          y1={height - 1}
          x2={width}
          y2={height - 1}
          stroke={tokens.line}
          strokeWidth="1"
        />
      </svg>
    )
  }

  const max = Math.max(...data, 1)
  const gap = 1
  const barWidth = Math.max(1, (width - gap * (data.length - 1)) / data.length)
  const total = data.reduce((a, b) => a + b, 0)

  return (
    <svg
      width={width}
      height={height}
      role="img"
      aria-label={title ?? `${total} visits over the last ${data.length} days`}
      style={{ display: 'block' }}
    >
      {data.map((v, i) => {
        // A day with traffic never disappears: it keeps a minimum of one
        // pixel, so "a little" reads differently from "none".
        const h = v === 0 ? 1 : Math.max(1.5, (v / max) * (height - 2))
        return (
          <rect
            key={i}
            x={i * (barWidth + gap)}
            y={height - h}
            width={barWidth}
            height={h}
            rx={barWidth > 2 ? 1 : 0}
            fill={v === 0 ? tokens.line : tokens.accent}
            opacity={v === 0 ? 1 : 0.55 + 0.45 * (v / max)}
          />
        )
      })}
    </svg>
  )
}

export default Spark
