// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// The server groups the statistics and orders them, so these helpers only
// reshape a payload for a chart. The dashboard used to parse every user
// agent here instead, which meant the stats endpoint sent one row per
// distinct user agent string and the browser threw most of them away.

// toChartData maps a grouped statistic onto the fields the charts read.
// A missing or failed response is an empty chart, not a crash.
export const toChartData = (rows) =>
  Array.isArray(rows) ? rows.map(({ name, count }) => ({ name, value: count })) : []

// topN keeps the largest buckets and adds the rest together, so a long
// tail of single visits does not make a chart unreadable. The server
// orders by count, so this only has to cut.
export const topN = (rows, n = 8) => {
  const data = toChartData(rows)
  if (data.length <= n) {
    return data
  }
  const rest = data.slice(n).reduce((sum, d) => sum + d.value, 0)
  return rest > 0 ? [...data.slice(0, n), { name: 'Others', value: rest }] : data.slice(0, n)
}

// botsExcluded describes what the figures leave out. The stats count
// people, and an exclusion nobody can see is indistinguishable from
// missing data.
export const botsExcluded = (bots) => {
  const pv = bots?.pv ?? 0
  if (pv === 0) {
    return ''
  }
  const visits = pv === 1 ? '1 automated visit' : `${pv.toLocaleString()} automated visits`
  return `Excludes ${visits}. Bots are not counted in any figure on this page.`
}
