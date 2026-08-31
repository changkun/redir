// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useState } from 'react'
import { tokens, mono } from '../theme'

// Trend draws page and unique views over a range of days.
//
// It is hand-drawn SVG. A charting engine was pulling most of a megabyte
// to render two polylines over at most a few dozen points, and it drew
// them in its own visual language rather than this one.
//
// Both series share a scale, because UV is a subset of PV and the point
// of showing them together is the gap between them.
const Trend = ({ pv = [], uv = [], labels = [], height = 150 }) => {
  const [hover, setHover] = useState(null)

  const n = labels.length
  if (n === 0) {
    return <div style={{ color: tokens.textFaint, fontSize: 12 }}>no data</div>
  }

  const pad = { top: 10, right: 8, bottom: 18, left: 40 }
  const w = 100 // a viewBox in percent, so the chart scales with its column
  const h = height
  const plotW = w - pad.left / 6 - pad.right / 6
  const plotH = h - pad.top - pad.bottom

  const max = Math.max(...pv, ...uv, 1)
  // A round ceiling makes the axis label readable and the line stable as
  // data changes underneath it.
  const ceil = niceCeiling(max)

  const x = (i) => (n === 1 ? plotW / 2 : (i / (n - 1)) * plotW)
  const y = (v) => pad.top + plotH - (v / ceil) * plotH

  const path = (series) =>
    series.map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i)},${y(v)}`).join(' ')

  const area = (series) =>
    `${path(series)} L${x(n - 1)},${pad.top + plotH} L${x(0)},${pad.top + plotH} Z`

  return (
    <div style={{ position: 'relative' }}>
      <svg
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        style={{ width: '100%', height, display: 'block' }}
        onMouseLeave={() => setHover(null)}
        onMouseMove={(e) => {
          const r = e.currentTarget.getBoundingClientRect()
          const rel = ((e.clientX - r.left) / r.width) * plotW
          const i = Math.round((rel / plotW) * (n - 1))
          setHover(Math.min(n - 1, Math.max(0, i)))
        }}
        role="img"
        aria-label={`page and unique views over ${n} days`}
      >
        {/* Two gridlines, not a grid. Enough to read a value against. */}
        {[0, 0.5, 1].map((f) => (
          <line
            key={f}
            x1="0"
            x2={plotW}
            y1={pad.top + plotH * f}
            y2={pad.top + plotH * f}
            stroke={tokens.line}
            strokeWidth="0.5"
            vectorEffect="non-scaling-stroke"
          />
        ))}

        <path d={area(pv)} fill={tokens.accent} opacity="0.10" />
        <path
          d={path(pv)}
          fill="none"
          stroke={tokens.accent}
          strokeWidth="1.5"
          vectorEffect="non-scaling-stroke"
          strokeLinejoin="round"
        />
        <path
          d={path(uv)}
          fill="none"
          stroke={tokens.good}
          strokeWidth="1.5"
          vectorEffect="non-scaling-stroke"
          strokeLinejoin="round"
        />

        {hover !== null && (
          <>
            <line
              x1={x(hover)}
              x2={x(hover)}
              y1={pad.top}
              y2={pad.top + plotH}
              stroke={tokens.lineStrong}
              strokeWidth="1"
              vectorEffect="non-scaling-stroke"
            />
            <circle cx={x(hover)} cy={y(pv[hover])} r="2" fill={tokens.accent} />
            <circle cx={x(hover)} cy={y(uv[hover])} r="2" fill={tokens.good} />
          </>
        )}
      </svg>

      {/* Labels sit outside the SVG so they are not stretched by the
          non-uniform viewBox scaling. */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          fontSize: 10,
          color: tokens.textFaint,
          fontFamily: mono,
        }}
      >
        {ceil.toLocaleString()}
      </div>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          fontSize: 10,
          color: tokens.textFaint,
          fontFamily: mono,
          marginTop: -14,
        }}
      >
        <span>{labels[0]}</span>
        <span>{labels[n - 1]}</span>
      </div>

      <div
        style={{
          display: 'flex',
          gap: 14,
          fontSize: 11,
          color: tokens.textDim,
          marginTop: 6,
        }}
      >
        <Legend colour={tokens.accent} name="PV" />
        <Legend colour={tokens.good} name="UV" />
        {hover !== null && (
          <span style={{ marginLeft: 'auto', fontFamily: mono }}>
            {labels[hover]} · {pv[hover].toLocaleString()} PV ·{' '}
            {uv[hover].toLocaleString()} UV
          </span>
        )}
      </div>
    </div>
  )
}

const Legend = ({ colour, name }) => (
  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
    <span
      style={{
        width: 8,
        height: 2,
        background: colour,
        borderRadius: 1,
        display: 'inline-block',
      }}
    />
    {name}
  </span>
)

// niceCeiling rounds an axis maximum up to something a reader can divide
// in their head: 1, 2 or 5 times a power of ten.
export const niceCeiling = (max) => {
  if (max <= 1) return 1
  const pow = 10 ** Math.floor(Math.log10(max))
  for (const step of [1, 2, 5, 10]) {
    if (max <= step * pow) return step * pow
  }
  return 10 * pow
}

export default Trend
