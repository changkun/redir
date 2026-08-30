// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useCallback, useEffect, useState } from 'react'
import { Row, Col, Divider, DatePicker } from 'antd'
import { PageHeader } from '@ant-design/pro-components'
import { Line, Pie, Bar } from '@ant-design/charts'
import UAParser from 'ua-parser-js'
import dayjs from 'dayjs'
import { day, dayBuckets, defaultRange } from '../lib/time'

const uaparser = new UAParser()

// charts v2 renders through G2 5, whose dark palette is named classicDark.
const chartTheme = 'classicDark'

const Stats = (props) => {
  const [[begin, end]] = useState(() => defaultRange())
  const [t0, setT0] = useState(begin)
  const [t1, setT1] = useState(end)

  const [pvuvData, setPVUVData] = useState([])
  const [refData, setRefData] = useState([])
  const [uaData, setUAData] = useState([])

  const endpoint = props.devMode
    ? 'http://localhost:9123/s/?'
    : '/s/?'

  const fetchStat = useCallback(
    (stat, from, to, set) => {
      fetch(
        endpoint +
          new URLSearchParams({
            mode: 'stats',
            a: props.alias,
            stat: stat,
            t0: from,
            t1: to,
          }),
      )
        .then((response) => response.json())
        .then((json) => set(json === null ? [] : json))
        .catch((error) => console.log('fetch data failed', error))
    },
    [endpoint, props.alias],
  )

  const fetchAll = useCallback(
    (from, to) => {
      fetchStat('time', from, to, setPVUVData)
      fetchStat('referer', from, to, setRefData)
      fetchStat('ua', from, to, setUAData)
    },
    [fetchStat],
  )

  useEffect(() => fetchAll(begin, end), [fetchAll, begin, end])

  const dateRangeOnChange = (_, dateString) => {
    setT0(dateString[0])
    setT1(dateString[1])
    fetchAll(dateString[0], dateString[1])
  }

  // Group the per-user-agent counts into browser and device totals,
  // dropping bots and entries the parser cannot identify.
  const browsers = {}
  const devices = {}
  for (const entry of uaData) {
    if (entry.ua.includes('bot') || entry.ua.includes('unknown')) {
      continue
    }
    const r = uaparser.setUA(entry.ua).getResult()
    const browser = r.browser.name ?? 'Others'
    const device = r.os.name ?? 'Others'
    browsers[browser] = (browsers[browser] ?? 0) + entry.count
    devices[device] = (devices[device] ?? 0) + entry.count
  }
  const byCount = (o) =>
    Object.entries(o)
      .map(([name, value]) => ({ name, value }))
      .sort((a, b) => b.value - a.value)

  return (
    <div className="redir-stats">
      <PageHeader className="site-page-header" title="Visitors" />
      <DatePicker.RangePicker
        style={{ float: 'right', bottom: '5px' }}
        defaultValue={[dayjs(begin), dayjs(end)]}
        onChange={dateRangeOnChange}
      />
      <Divider />
      <StatLine data={pvuvData} t0={t0} t1={t1} />
      <Row>
        <Col span={12}>
          <PageHeader className="site-page-header" title="Referrers" />
          <StatPieRef data={refData} />
        </Col>
        <Col span={12}>
          <PageHeader className="site-page-header" title="Browsers" />
          <StatBarUA data={byCount(browsers)} />
        </Col>
      </Row>
      <Divider />
      <Row>
        <Col span={24}>
          <PageHeader className="site-page-header" title="Devices" />
          <StatBarUA data={byCount(devices)} />
        </Col>
      </Row>
      <Divider />
    </div>
  )
}

const StatLine = (props) => {
  const pv = dayBuckets(props.t0, props.t1)
  const uv = dayBuckets(props.t0, props.t1)
  for (const point of props.data) {
    const d = day(new Date(point.time))
    if (pv[d] !== undefined) {
      pv[d] += point.pv
    }
    if (uv[d] !== undefined) {
      uv[d] += point.uv
    }
  }
  const data = [
    ...Object.entries(pv).map(([time, value]) => ({ time, value, category: 'PV' })),
    ...Object.entries(uv).map(([time, value]) => ({ time, value, category: 'UV' })),
  ]

  return (
    <Line
      theme={chartTheme}
      data={data}
      xField="time"
      yField="value"
      colorField="category"
      shapeField="smooth"
      scale={{ color: { range: ['#5B8FF9', '#5AD8A6'] } }}
      axis={{ x: { title: false }, y: { title: false } }}
      legend={{ color: { position: 'top', layout: { justifyContent: 'center' } } }}
      autoFit
      height={300}
    />
  )
}

const StatPieRef = (props) => (
  <Pie
    theme={chartTheme}
    data={props.data.map((entry) => ({
      name: entry.referer === 'unknown' ? 'Direct' : entry.referer,
      value: entry.count,
    }))}
    angleField="value"
    colorField="name"
    radius={0.8}
    label={{ text: 'name', position: 'spider' }}
    legend={{
      color: { position: 'bottom', layout: { justifyContent: 'center' } },
    }}
    autoFit
    height={300}
  />
)

// Bar in charts v2 is an interval mark on a transposed coordinate, so the
// category goes on xField and the measure on yField. This is the reverse
// of the v1 API.
const StatBarUA = (props) => (
  <Bar
    theme={chartTheme}
    data={props.data}
    xField="name"
    yField="value"
    colorField="name"
    legend={false}
    axis={{ x: { title: false }, y: { title: false } }}
    autoFit
    height={300}
  />
)

export default Stats
