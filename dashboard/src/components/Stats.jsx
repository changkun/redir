// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useCallback, useEffect, useState } from 'react'
import { DatePicker } from 'antd'
import dayjs from 'dayjs'
import { day, dayBuckets, defaultRange } from '../lib/time'
import { botsExcluded, topN } from '../lib/stats'
import { fetchStat } from '../lib/api'
import { aliasPath } from '../lib/paths'
import Trend from './Trend'

// List draws a grouped statistic as rows rather than as a chart.
//
// A bar chart of six categories spends 300 pixels and a rendering engine
// on what a list with a proportional rule says in a fifth of the space,
// and the list keeps the exact figure legible next to it.
const List = ({ title, data }) => {
  const max = Math.max(...data.map((d) => d.value), 1)
  const total = data.reduce((s, d) => s + d.value, 0)

  return (
    <div>
      <h3>{title}</h3>
      {data.length === 0 && <div className="hint">No visits.</div>}
      {data.map((d) => (
        <div
          className="line"
          key={d.name}
          title={
            total > 0
              ? `${((d.value / total) * 100).toFixed(1)}% of ${title.toLowerCase()}`
              : ''
          }
        >
          <span title={d.name}>{d.name}</span>
          <span className="bar">
            <i style={{ width: `${Math.max(2, (d.value / max) * 100)}%` }} />
          </span>
          <b>{d.value.toLocaleString()}</b>
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

  return (
    <div className="detail-in">
      <Trend
        title={`Daily visits of ${aliasPath(alias)}`}
        note={botsExcluded(bots) || 'Bots are not counted in any figure here.'}
        tools={
          <DatePicker.RangePicker
            size="small"
            allowClear={false}
            defaultValue={[dayjs(begin), dayjs(end)]}
            onChange={(_, s) => {
              setT0(s[0])
              setT1(s[1])
              load(s[0], s[1])
            }}
          />
        }
        {...toSeries(pvuv, t0, t1)}
      />

      <div className="lists">
        <List title="Referrers" data={topN(refs, 6)} />
        <List title="Browsers" data={topN(browsers, 6)} />
        <List title="Systems" data={topN(oses, 6)} />
        <List title="Devices" data={topN(devices, 4)} />
      </div>
    </div>
  )
}

// toSeries turns the hourly counts the endpoint returns into the daily
// arrays the chart draws, with a value for every day in the range so a
// quiet day is a dip rather than a missing point.
//
// The range the view opens with ends tomorrow, so that today is inside
// it. A day that has not begun is not drawn: it could only ever read
// zero, and the line would fall to the axis at its right end every day.
export const toSeries = (data, t0, t1, today = day(new Date())) => {
  const pv = dayBuckets(t0, t1)
  const uv = dayBuckets(t0, t1)
  for (const p of data) {
    const d = day(new Date(p.time))
    if (pv[d] !== undefined) pv[d] += p.pv
    if (uv[d] !== undefined) uv[d] += p.uv
  }
  const labels = Object.keys(pv).filter((d) => d <= today)
  return {
    labels,
    pv: labels.map((d) => pv[d]),
    uv: labels.map((d) => uv[d]),
  }
}

export default Stats
