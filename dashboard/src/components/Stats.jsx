// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useCallback, useEffect, useState } from 'react'
import { Row, Col, Divider, DatePicker, Typography } from 'antd'
import { PageHeader } from '@ant-design/pro-components'
import { Line, Pie, Bar } from '@ant-design/charts'
import dayjs from 'dayjs'
import { day, dayBuckets, defaultRange } from '../lib/time'
import { botsExcluded, topN } from '../lib/stats'

// charts v2 renders through G2 5, whose dark palette is named classicDark.
const chartTheme = 'classicDark'

const Stats = (props) => {
  const [[begin, end]] = useState(() => defaultRange())
  const [t0, setT0] = useState(begin)
  const [t1, setT1] = useState(end)

  const [pvuvData, setPVUVData] = useState([])
  const [refData, setRefData] = useState([])
  const [browserData, setBrowserData] = useState([])
  const [osData, setOSData] = useState([])
  const [deviceData, setDeviceData] = useState([])
  const [bots, setBots] = useState(null)

  const endpoint = props.devMode ? 'http://localhost:9123/s/?' : '/s/?'

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

  // Every series is grouped and ordered by the server. The browser no
  // longer parses user agents: the counts arrive ready to draw.
  const fetchAll = useCallback(
    (from, to) => {
      fetchStat('time', from, to, setPVUVData)
      fetchStat('referer', from, to, setRefData)
      fetchStat('browser', from, to, setBrowserData)
      fetchStat('os', from, to, setOSData)
      fetchStat('device', from, to, setDeviceData)
      fetchStat('bots', from, to, setBots)
    },
    [fetchStat],
  )

  useEffect(() => fetchAll(begin, end), [fetchAll, begin, end])

  const dateRangeOnChange = (_, dateString) => {
    setT0(dateString[0])
    setT1(dateString[1])
    fetchAll(dateString[0], dateString[1])
  }

  const excluded = botsExcluded(bots)

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
      {excluded && (
        <Typography.Paragraph type="secondary" style={{ marginTop: 8 }}>
          {excluded}
        </Typography.Paragraph>
      )}
      <Row>
        <Col span={12}>
          <PageHeader className="site-page-header" title="Referrers" />
          <StatPie data={topN(refData)} />
        </Col>
        <Col span={12}>
          <PageHeader className="site-page-header" title="Browsers" />
          <StatBar data={topN(browserData)} />
        </Col>
      </Row>
      <Divider />
      <Row>
        <Col span={12}>
          <PageHeader className="site-page-header" title="Operating systems" />
          <StatBar data={topN(osData)} />
        </Col>
        <Col span={12}>
          <PageHeader className="site-page-header" title="Devices" />
          <StatBar data={topN(deviceData)} />
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

const StatPie = (props) => (
  <Pie
    theme={chartTheme}
    data={props.data}
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
const StatBar = (props) => (
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
