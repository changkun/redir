import React, { useState, useEffect } from 'react'
import { Row, Col, PageHeader, Divider } from 'antd';
import { Line, Pie, Bar } from '@ant-design/charts'
import UAParser from 'ua-parser-js'

const uaparser = new UAParser()

const Stats = (props) => {
  const today = new Date()
  const start = new Date()
  start.setDate(today.getDate() - 30)
  const t0 = start.toISOString().slice(0, 10)
  const t1 = today.toISOString().slice(0, 10)

  const [pvuvData, setPVUVData] = useState([])
  useEffect(() => {asyncFetchTime()}, [])
  const asyncFetchTime = () => {
    fetch('http://localhost:9123/s/?'+ new URLSearchParams({
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
  useEffect(() => {asyncFetchRef()}, [])
  const asyncFetchRef = () => {
    fetch('http://localhost:9123/s/?'+ new URLSearchParams({
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
  useEffect(() => {asyncFetchUA()}, [])
  const asyncFetchUA = () => {
    fetch('http://localhost:9123/s/?'+ new URLSearchParams({
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

  return (
    <div>
      <PageHeader
        className="site-page-header"
        onBack={false}
        title="Visitors"
        subTitle="Last 30 Days"
      />
      <StatLine alias={props.alias} data={pvuvData}/>
      <Divider />
      <Row>
        <Col span={12}>
          <PageHeader
            className="site-page-header"
            title="Referrers"
            subTitle="Last 30 Days"
          />
          <StatPieRef data={refData}/>
        </Col>
        <Col span={12}>
          <PageHeader
            className="site-page-header"
            title="Browsers"
            subTitle="Last 30 Days"
          />
          <StatBarUA data={browserArray}/>
        </Col>
      </Row>
      <Divider />
      <Row>
      <Col span={24} style={{height: '200px'}}>
        <PageHeader
          className="site-page-header"
          title="Devices"
          subTitle="Last 30 Days"
        />
        <StatBarUA data={deviceArray}/>
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
  const today = new Date()
  const start = new Date()
  start.setDate(today.getDate() - 30)

  const begin = formatDate(start)
  const end = formatDate(today)
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