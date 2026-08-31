// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useCallback, useEffect, useState } from 'react'
import { DatePicker } from 'antd'
import dayjs from 'dayjs'
import { day, dayBuckets, defaultRange } from '../lib/time'
import { botsExcluded, topN } from '../lib/stats'
import { fetchStat } from '../lib/api'
import { tokens, mono } from '../theme'
import Trend from './Trend'

// Bars draws a grouped statistic as rows rather than as a chart.
//
// A bar chart of six categories spends 300 pixels and a rendering engine
// on what a list with a proportional rule says in a fifth of the space,
// and the list keeps the exact figure legible next to it.
const Bars = ({ title, data }) => {
  const max = Math.max(...data.map((d) => d.value), 1)
  const total = data.reduce((s, d) => s + d.value, 0)

  return (
    <div style={{ minWidth: 0, flex: 1 }}>
      <div
        style={{
          color: tokens.textDim,
          fontSize: 11,
          textTransform: 'uppercase',
          letterSpacing: '0.06em',
          marginBottom: 6,
        }}
      >
        {title}
      </div>
      {data.length === 0 && (
        <div style={{ color: tokens.textFaint, fontSize: 12 }}>no data</div>
      )}
      {data.map((d) => (
        <div
          key={d.name}
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 52px',
            alignItems: 'center',
            gap: 8,
            padding: '2px 0',
          }}
        >
          <div style={{ minWidth: 0, position: 'relative' }}>
            <div
              style={{
                position: 'absolute',
                inset: 0,
                width: `${(d.value / max) * 100}%`,
                background: tokens.accent,
                opacity: 0.14,
                borderRadius: 2,
              }}
            />
            <div
              style={{
                position: 'relative',
                fontFamily: mono,
                fontSize: 12,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                padding: '1px 4px',
              }}
              title={d.name}
            >
              {d.name}
            </div>
          </div>
          <div
            className="num"
            style={{ fontSize: 12, color: tokens.textDim }}
            title={total > 0 ? `${((d.value / total) * 100).toFixed(1)}%` : ''}
          >
            {d.value.toLocaleString()}
          </div>
        </div>
      ))}
    </div>
  )
}

// Stats is one link's detail: when it was used, and by whom.
const Stats = ({ alias, devMode }) => {
  const [[begin, end]] = useState(() => defaultRange())
  const [t0, setT0] = useState(begin)
  const [t1, setT1] = useState(end)

  const [pvuv, setPVUV] = useState([])
  const [refs, setRefs] = useState([])
  const [browsers, setBrowsers] = useState([])
  const [oses, setOSes] = useState([])
  const [devices, setDevices] = useState([])
  const [bots, setBots] = useState(null)

  const load = useCallback(
    (from, to) => {
      const one = (stat, set) =>
        fetchStat(devMode, alias, stat, from, to)
          .then((j) => set(j === null ? [] : j))
          .catch(() => set([]))

      one('time', setPVUV)
      one('referer', setRefs)
      one('browser', setBrowsers)
      one('os', setOSes)
      one('device', setDevices)
      one('bots', setBots)
    },
    [alias, devMode],
  )

  useEffect(() => load(begin, end), [load, begin, end])

  const excluded = botsExcluded(bots)

  return (
    <div className="redir-stats" style={{ padding: `${tokens.space(3)}px 0` }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: tokens.space(3),
          marginBottom: tokens.space(3),
        }}
      >
        <span style={{ color: tokens.textDim, fontSize: 12 }}>
          {excluded || 'Bots are not counted in any figure here.'}
        </span>
        <DatePicker.RangePicker
          size="small"
          style={{ marginLeft: 'auto' }}
          defaultValue={[dayjs(begin), dayjs(end)]}
          onChange={(_, s) => {
            setT0(s[0])
            setT1(s[1])
            load(s[0], s[1])
          }}
        />
      </div>

      <Trend {...toSeries(pvuv, t0, t1)} />

      <div
        style={{
          display: 'flex',
          gap: tokens.space(8),
          marginTop: tokens.space(4),
          flexWrap: 'wrap',
        }}
      >
        <Bars title="Referrers" data={topN(refs, 6)} />
        <Bars title="Browsers" data={topN(browsers, 6)} />
        <Bars title="Systems" data={topN(oses, 6)} />
        <Bars title="Devices" data={topN(devices, 4)} />
      </div>
    </div>
  )
}

// toSeries turns the hourly counts the endpoint returns into the daily
// arrays the chart draws, with a value for every day in the range so a
// quiet day is a dip rather than a missing point.
const toSeries = (data, t0, t1) => {
  const pv = dayBuckets(t0, t1)
  const uv = dayBuckets(t0, t1)
  for (const p of data) {
    const d = day(new Date(p.time))
    if (pv[d] !== undefined) pv[d] += p.pv
    if (uv[d] !== undefined) uv[d] += p.uv
  }
  return {
    labels: Object.keys(pv),
    pv: Object.values(pv),
    uv: Object.values(uv),
  }
}

export default Stats
