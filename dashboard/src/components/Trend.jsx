// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useEffect, useState } from 'react'

const fmt = (n) => (n ?? 0).toLocaleString()

// dayLabel turns the YYYY-MM-DD a series is keyed by into "Sep 5". The
// string names a calendar day, so it is read as one and never shifted
// through a time zone.
export const dayLabel = (key, year = false) => {
  const [y, m, d] = String(key).split('-').map(Number)
  if (!y || !m || !d) return String(key)
  return new Date(y, m - 1, d).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    ...(year ? { year: 'numeric' } : {}),
  })
}

// Trend draws visits and visitors over a range of days.
//
// It is hand-drawn SVG. A charting engine was pulling most of a megabyte
// to render two polylines over at most a few dozen points, and it drew
// them in its own visual language rather than this one.
//
// Both series share one scale, because visitors are a subset of visits
// and the point of showing them together is the gap between them. Every
// figure can be read without the pointer: the arrow keys move the reading
// along the days, and the same numbers are there as a table.
const Trend = ({
  title,
  note,
  tools,
  pv = [],
  uv = [],
  labels = [],
  height = 200,
  loading = false,
}) => {
  // The element is kept in state so that the measuring below starts when
  // the plot appears, which is after the data has arrived.
  const [plot, setPlot] = useState(null)
  const [width, setWidth] = useState(800)
  const [hover, setHover] = useState(-1)
  const [table, setTable] = useState(false)

  // The chart is drawn in pixels of its own column, so a line stays two
  // pixels wide and a label eleven, whatever the width.
  useEffect(() => {
    if (!plot) return undefined
    const measure = () => plot.clientWidth > 0 && setWidth(plot.clientWidth)
    measure()
    const watch = new ResizeObserver(measure)
    watch.observe(plot)
    return () => watch.disconnect()
  }, [plot])

  const n = labels.length
  const total = pv.reduce((a, b) => a + b, 0)
  const empty = n === 0 || total === 0

  const head = (
    <div className="head">
      <h3>{title}</h3>
      {note && <span className="hint">{note}</span>}
      <div className="right">
        {tools}
        {!table && !empty && (
          <div className="legend">
            <span>
              <i style={{ background: 'var(--pv)' }} />
              Visits
            </span>
            <span>
              <i style={{ background: 'var(--uv)' }} />
              Visitors
            </span>
          </div>
        )}
        {!empty && (
          <button
            type="button"
            className="link"
            aria-pressed={table}
            onClick={() => setTable(!table)}
          >
            {table ? 'Show as chart' : 'Show as table'}
          </button>
        )}
      </div>
    </div>
  )

  if (empty) {
    return (
      <>
        {head}
        <div className="empty">
          {loading ? 'Loading…' : 'No visits in this period.'}
        </div>
      </>
    )
  }

  if (table) {
    return (
      <>
        {head}
        <div className="scroll">
          <table>
            <thead>
              <tr>
                <th>Day</th>
                <th className="num">Visits</th>
                <th className="num">Visitors</th>
              </tr>
            </thead>
            <tbody>
              {labels
                .map((key, i) => (
                  <tr key={key}>
                    <td>{dayLabel(key, true)}</td>
                    <td className="num">{fmt(pv[i])}</td>
                    <td className="num">{fmt(uv[i])}</td>
                  </tr>
                ))
                .reverse()}
            </tbody>
          </table>
        </div>
      </>
    )
  }

  const W = width - 16 // the plot's own padding
  const H = height
  const { top, step } = axis(Math.max(...pv, ...uv))
  const m = { l: 8 + fmt(top).length * 6.4, r: 12, t: 10, b: 22 }
  const pw = W - m.l - m.r
  const ph = H - m.t - m.b
  const x = (i) => m.l + (n > 1 ? (i / (n - 1)) * pw : pw / 2)
  const y = (v) => m.t + ph - (v / top) * ph

  const line = (series) =>
    series
      .map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`)
      .join('')

  // Date labels, evenly spaced and counted back from the last day so the
  // most recent one is always named.
  const every = Math.max(1, Math.ceil(n / Math.max(2, Math.floor(pw / 80))))
  const ticks = []
  for (let i = n - 1; i >= 0; i -= every) ticks.push(i)

  const nearest = (e) => {
    if (n === 1) return 0
    const r = e.currentTarget.getBoundingClientRect()
    const px = (e.clientX - r.left) * (W / r.width)
    return Math.max(0, Math.min(n - 1, Math.round(((px - m.l) / pw) * (n - 1))))
  }

  const at = hover >= 0 && hover < n ? hover : -1

  return (
    <>
      {head}
      <div className="plot" ref={setPlot}>
        <svg
          viewBox={`0 0 ${W} ${H}`}
          height={H}
          role="img"
          tabIndex={0}
          aria-label={`${title}, ${n} days. Use the arrow keys to read each day, or show the table.`}
          onPointerMove={(e) => setHover(nearest(e))}
          onPointerLeave={(e) => {
            if (document.activeElement !== e.currentTarget) setHover(-1)
          }}
          onFocus={() => setHover((h) => (h < 0 ? n - 1 : h))}
          onBlur={() => setHover(-1)}
          onKeyDown={(e) => {
            if (e.key === 'ArrowLeft')
              setHover((h) => Math.max(0, (h < 0 ? n : h) - 1))
            else if (e.key === 'ArrowRight')
              setHover((h) => Math.min(n - 1, h + 1))
            else if (e.key === 'Escape') setHover(-1)
            else return
            e.preventDefault()
          }}
        >
          {Array.from(
            { length: Math.round(top / step) + 1 },
            (_, i) => i * step,
          ).map((v) => (
            <g key={v}>
              <line
                x1={m.l}
                x2={W - m.r}
                y1={y(v)}
                y2={y(v)}
                stroke={v ? 'var(--grid)' : 'var(--axis)'}
              />
              <text x={m.l - 7} y={y(v) + 3.5} textAnchor="end">
                {fmt(v)}
              </text>
            </g>
          ))}
          {ticks.map((i) => (
            <text
              key={i}
              x={x(i)}
              y={H - 5}
              textAnchor={
                n === 1
                  ? 'middle'
                  : x(i) > W - m.r - 24
                    ? 'end'
                    : x(i) < m.l + 24
                      ? 'start'
                      : 'middle'
              }
            >
              {dayLabel(labels[i])}
            </text>
          ))}

          {n === 1 ? (
            // One day is two points, not two lines.
            <>
              <circle
                cx={x(0)}
                cy={y(pv[0])}
                r="4"
                fill="var(--pv)"
                stroke="var(--surface)"
                strokeWidth="2"
              />
              <circle
                cx={x(0)}
                cy={y(uv[0])}
                r="4"
                fill="var(--uv)"
                stroke="var(--surface)"
                strokeWidth="2"
              />
            </>
          ) : (
            <>
              <path
                d={`${line(pv)}L${x(n - 1).toFixed(1)},${y(0)}L${x(0).toFixed(1)},${y(0)}Z`}
                fill="var(--pv)"
                opacity="0.1"
              />
              <path
                d={line(pv)}
                fill="none"
                stroke="var(--pv)"
                strokeWidth="2"
                strokeLinejoin="round"
                strokeLinecap="round"
              />
              <path
                d={line(uv)}
                fill="none"
                stroke="var(--uv)"
                strokeWidth="2"
                strokeLinejoin="round"
                strokeLinecap="round"
              />
            </>
          )}

          {at >= 0 && (
            <g>
              <line
                x1={x(at)}
                x2={x(at)}
                y1={m.t}
                y2={m.t + ph}
                stroke="var(--axis)"
              />
              <circle
                cx={x(at)}
                cy={y(pv[at])}
                r="4"
                fill="var(--pv)"
                stroke="var(--surface)"
                strokeWidth="2"
              />
              <circle
                cx={x(at)}
                cy={y(uv[at])}
                r="4"
                fill="var(--uv)"
                stroke="var(--surface)"
                strokeWidth="2"
              />
            </g>
          )}
        </svg>

        {at >= 0 && (
          // The reading sits on whichever side of the hairline has room.
          <div
            className="tip"
            style={
              x(at) > W * 0.6 ? { right: W - x(at) + 20 } : { left: x(at) + 20 }
            }
          >
            <div className="day">{dayLabel(labels[at], true)}</div>
            <div className="row">
              <i style={{ background: 'var(--pv)' }} />
              <b>{fmt(pv[at])}</b>
              <span>visits</span>
            </div>
            <div className="row">
              <i style={{ background: 'var(--uv)' }} />
              <b>{fmt(uv[at])}</b>
              <span>visitors</span>
            </div>
          </div>
        )}
      </div>
    </>
  )
}

// axis picks the top of the scale and the step between gridlines: a round
// step, 1, 2, 2.5 or 5 times a power of ten where that is a whole number,
// that puts about four lines under the data. Counts are whole, so a line
// never sits on a fraction, and the top is never below the data nor zero.
export const axis = (max) => {
  if (!(max > 4)) return { top: 4, step: 1 }
  const raw = max / 4
  const pow = 10 ** Math.floor(Math.log10(raw))
  const step =
    [1, 2, 2.5, 5, 10]
      .map((f) => f * pow)
      .find((v) => v >= raw && Number.isInteger(v)) ?? 10 * pow
  return { top: Math.ceil(max / step) * step, step }
}

export default Trend
