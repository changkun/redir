// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import React, { useState, useEffect } from 'react'
import { Row, Col, PageHeader, Divider, DatePicker } from 'antd';
import { Line, Pie, Bar } from '@ant-design/charts'
import UAParser from 'ua-parser-js'
import moment from 'moment';

const uaparser = new UAParser()

const Stats = (props) => {
  const today = new Date()
  today.setDate(today.getDate() + 1)
  const start = new Date()
  start.setDate(today.getDate() - 30)
  let begin = start.toISOString().slice(0, 10)
  let end = today.toISOString().slice(0, 10)

  const [t0, setT0] = useState([])
  useEffect(() => updateT0(begin), [])
  const updateT0 = (t0) => {
    setT0(t0)
  }
  const [t1, setT1] = useState([])
  useEffect(() => updateT1(end), [])
  const updateT1 = (t1) => {
    setT1(t1)
  }

  let endpoint = '/s/?'
  if (props.devMode) {
    endpoint = 'http://localhost:9123/s/?'
  }

  console.log(begin, end)

  const [pvuvData, setPVUVData] = useState([])
  useEffect(() => asyncFetchTime(t0, t1), [])
  const asyncFetchTime = (t0, t1) => {
    fetch(endpoint+ new URLSearchParams({
      mode: 'stats',
      a: props.alias,
      stat: 'time',
      t0: t0,
      t1: t1,
    }))
    .then((response) => response.json())
    .then((json) => {
      if (json === null) json = []
      setPVUVData(json)
    })
    .catch((error) => {
      console.log('fetch data failed', error)
    })
  }
  const [refData, setRefData] = useState([])
  useEffect(() => asyncFetchRef(t0, t1), [])
  const asyncFetchRef = (t0, t1) => {
    fetch(endpoint+ new URLSearchParams({
      mode: 'stats',
      a: props.alias,
      stat: 'referer',
      t0: t0,
      t1: t1,
    }))
    .then((response) => response.json())
    .then((json) => {
      if (json === null) json = []
      setRefData(json)
    })
    .catch((error) => {
      console.log('fetch data failed', error)
    })
  }
  const [uaData, setUAData] = useState([])
  useEffect(() => {asyncFetchUA(t0, t1)}, [])
  const asyncFetchUA = (t0, t1) => {
    fetch(endpoint+ new URLSearchParams({
      mode: 'stats',
      a: props.alias,
      stat: 'ua',
      t0: t0,
      t1: t1,
    }))
    .then((response) => response.json())
    .then((json) => {
      if (json === null) json = []
      setUAData(json)
    })
    .catch((error) => {
      console.log('fetch data failed', error)
    })
  }
  const validUAData = []
  for (let i = 0; i < uaData.length; i++) {
    const entry = uaData[i]
    if (entry.ua.includes('bot') || entry.ua.includes('unknown')) {
      continue
    }
    const r = uaparser.setUA(entry.ua).getResult()
    uaData[i].browser = r.browser.name
    uaData[i].device = r.os.name
    validUAData.push(uaData[i])
  }

  let browsers = {}
  let devices = {}
  for (let i = 0; i < validUAData.length; i++) {
    if (validUAData[i].browser === undefined) {
      validUAData[i].browser = 'Others'
    }
    if (browsers[validUAData[i].browser] === undefined) {
      browsers[validUAData[i].browser] = validUAData[i].count
    } else {
      browsers[validUAData[i].browser] += validUAData[i].count
    }

    if (validUAData[i].device === undefined) {
      validUAData[i].device = 'Others'
    }
    if (devices[validUAData[i].device] === undefined) {
      devices[validUAData[i].device] = validUAData[i].count
    } else {
      devices[validUAData[i].device] += validUAData[i].count
    }
  }
  const browserArray = []
  for (const [key, value] of Object.entries(browsers)) {
    browserArray.push({value: value, name: key})
  }
  const deviceArray = []
  for (const [key, value] of Object.entries(devices)) {
    deviceArray.push({value: value, name: key})
  }

  browserArray.sort((a, b) => {
    if (a.value < b.value) return 1
    if (a.value > b.value) return -1
    return 0
  })
  deviceArray.sort((a, b) => {
    if (a.value < b.value) return 1
    if (a.value > b.value) return -1
    return 0
  })
  const dateRangeOnChange = (_, dateString) => {
    const d0 = dateString[0]
    const d1 = dateString[1]
    setT0(d0)
    setT1(d1)
    asyncFetchTime(d0, d1)
    asyncFetchRef(d0, d1)
    asyncFetchUA(d0, d1)
  }
  return (
    <div>
      <PageHeader
        className="site-page-header"
        onBack={false}
        title="Visitors"
      />
      <DatePicker.RangePicker style={{float: 'right', bottom: '5px'}} defaultValue={[moment(begin), moment(end)]} onChange={dateRangeOnChange}/>
      <Divider />
      <StatLine alias={props.alias} data={pvuvData} t0={t0} t1={t1}/>
      <Row>
        <Col span={12}>
          <PageHeader
            className="site-page-header"
            title="Referrers"
          />
          <StatPieRef data={refData} t0={t0} t1={t1}/>
        </Col>
        <Col span={12}>
          <PageHeader
            className="site-page-header"
            title="Browsers"
          />
          <StatBarUA data={browserArray} t0={t0} t1={t1}/>
        </Col>
      </Row>
      <Divider />
      <Row>
      <Col span={24} style={{height: '200px'}}>
        <PageHeader
          className="site-page-header"
          title="Devices"
        />
        <StatBarUA data={deviceArray} t0={t0} t1={t1}/>
      </Col>
      </Row>
      <Divider />
    </div>
  )
}
const formatDate = (date) => {
  return new Date(date).toISOString().slice(0, 10)
}
const dateRange = (startDate, endDate, steps = 1) => {
  const dates = {}
  let currentDate = new Date(startDate)

  while (currentDate <= new Date(endDate)) {
    dates[formatDate(new Date(currentDate))] = 0
    currentDate.setUTCDate(currentDate.getUTCDate() + steps)
  }
  return dates
}

const StatLine = (props) => {
  const begin = props.t0
  const end = props.t1  
  const pv = dateRange(begin, end)
  const uv = dateRange(begin, end)
  for (let i = 0; i < props.data.length; i++) {
    const d = formatDate(new Date(props.data[i].time))
      if (pv[d] !== undefined) {
        pv[d] += props.data[i].pv
      }
      if (uv[d] !== undefined) {
        uv[d] += props.data[i].uv
      }
  }
  const allData = []
  for (const [k, v] of Object.entries(pv)) {
    allData.push({
      time: k,
      value: v,
      category: 'PV'
    })
  }
  for (const [k, v] of Object.entries(uv)) {
    allData.push({
      time: k,
      value: v,
      category: 'UV'
    })
  }

  const config = {
    theme: 'dark',
    appendPadding: 20,
    data: allData,
    showTitle: true,
    xField: 'time',
    yField: 'value',
    seriesField: 'category',
    smooth: true,
    color: ['#5B8FF9', '#5AD8A6'],
    point: {
      shape: function shape(_ref) {
        var category = _ref.category
        return category === 'PV' ? 'square' : 'circle'
      },
    },
  }
  return <Line {...config} style={{height: '300px'}}/>
}

const StatPieRef = (props) => {
  const config = {
    theme: 'dark',
    appendPadding: 20,
    data: props.data.map(entry => {
      if (entry.referer === 'unknown') {
        entry.referer = 'Direct'
      }
      return {
        value: entry.count,
        name: entry.referer,
      }
    }),
    angleField: 'value',
    colorField: 'name',
    radius: 0.8,
    legend: {
      position: 'bottom',
      layout: 'horizontal',
    },
    label: {
      type: 'outer',
      content: '{percentage}',
    },
    interactions: [{
      type: 'pie-legend-active'
    }, {
      type: 'element-active'
    }],
  }
  return <Pie {...config} />
}

const StatBarUA = (props) => {
  const config = {
    theme: 'dark',
    appendPadding: 20,
    data: props.data,
    xField: 'value',
    yField: 'name',
    seriesField: 'name',
    legend: { position: 'top-right' },
  }
  return <Bar {...config} />
}

export default Stats