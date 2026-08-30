// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

const pad = (n) => (n < 10 ? '0' + n : String(n))

// day formats a Date as the YYYY-MM-DD the stats endpoint parses. It reads
// the local calendar fields throughout, so the string always names the day
// the visitor sees rather than the UTC day.
export const day = (d) =>
  `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

// defaultRange returns the [start, end] days the stats view opens with:
// the last 30 days, ending tomorrow so today's visits are included.
export const defaultRange = (now = new Date()) => {
  const start = new Date(now)
  start.setDate(start.getDate() - 30)
  const end = new Date(now)
  end.setDate(end.getDate() + 1)
  return [day(start), day(end)]
}

// dayBuckets returns one zeroed bucket per day in [start, end], keyed by
// the same day strings the series data is bucketed into.
export const dayBuckets = (start, end) => {
  const buckets = {}
  const last = new Date(end)
  for (let d = new Date(start); d <= last; d.setDate(d.getDate() + 1)) {
    buckets[day(d)] = 0
  }
  return buckets
}

// rfc3339 formats a date-like value for the server, which stores times as
// RFC 3339. It returns null for an empty value so the field clears.
export const rfc3339 = (value) => {
  if (value === '' || value === null || value === undefined) {
    return null
  }
  if (typeof value !== 'string') {
    return value.format() // a dayjs value from the date picker
  }

  const d = new Date(value)
  const offset = d.getTimezoneOffset()
  const zone =
    offset === 0
      ? 'Z'
      : (offset > 0 ? '-' : '+') +
        pad(Math.floor(Math.abs(offset) / 60)) +
        ':' +
        pad(Math.abs(offset) % 60)

  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}` +
    zone
  )
}
