// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { tokens } from '../theme'
import Spark from './Spark'

const fmt = (n) => (n ?? 0).toLocaleString()

// pct renders a share without pretending to precision it does not have.
const pct = (part, whole) =>
  whole > 0 ? `${((part / whole) * 100).toFixed(1)}%` : '—'

const Figure = ({ value, label, hint, tone }) => (
  <div style={{ minWidth: 0 }}>
    <div
      className="num"
      style={{
        fontSize: 22,
        lineHeight: '28px',
        textAlign: 'left',
        color: tone ?? tokens.text,
        letterSpacing: '-0.01em',
      }}
    >
      {value}
    </div>
    <div style={{ color: tokens.textDim, fontSize: 12 }}>
      {label}
      {hint && (
        <span style={{ color: tokens.textFaint }}> · {hint}</span>
      )}
    </div>
  </div>
)

// Overview is the console's first answer: whether anything is happening.
//
// Four figures and a shape. The figures are the ones an operator acts on,
// and automated traffic is among them because it is the majority here and
// leaving it out would make the others look wrong.
const Overview = ({ data, days }) => {
  const o = data ?? {}
  const series = (o.series ?? []).map((d) => d.pv)

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'flex-end',
        gap: tokens.space(10),
        padding: `${tokens.space(4)}px 0 ${tokens.space(4)}px`,
        borderBottom: `1px solid ${tokens.line}`,
        flexWrap: 'wrap',
      }}
    >
      <Figure value={fmt(o.links)} label="links" />
      <Figure value={fmt(o.visits)} label="visits" hint={`${days} days`} />
      <Figure value={fmt(o.people)} label="people" />
      <Figure
        value={pct(o.bots, o.visits)}
        label="automated"
        hint={fmt(o.bots)}
        tone={tokens.textDim}
      />
      <div style={{ marginLeft: 'auto', textAlign: 'right' }}>
        <Spark
          data={series}
          width={220}
          height={34}
          title={`traffic over the last ${days} days`}
        />
        <div style={{ color: tokens.textFaint, fontSize: 11, marginTop: 2 }}>
          people per day
        </div>
      </div>
    </div>
  )
}

export default Overview
