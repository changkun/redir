// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useCallback, useEffect, useMemo, useState } from 'react'
import { App, Input, Table, Tooltip, Typography } from 'antd'
import { tokens, mono } from '../theme'
import Spark from './Spark'
import Stats from './Stats'
import LinkForm from './LinkForm'
import { aliasPath, aliasURL, basePath } from '../lib/paths'
import { fetchIndex, remove } from '../lib/api'
import { targetLabel } from '../lib/format'

const fmt = (n) => (n ?? 0).toLocaleString()

// LinkTable is the console's list of links.
//
// It replaces EditableProTable. Editing happens in a form rather than in
// the row: a row is for reading, and eleven fields of inline inputs is
// what made the old table unreadable. The columns left are the ones an
// operator scans, and the rest are one click away.
const LinkTable = ({ isAdmin, statsMode, devMode, reloadKey, onLoaded }) => {
  const { message, modal } = App.useApp()
  const [rows, setRows] = useState([])
  const [series, setSeries] = useState({})
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')
  const [editing, setEditing] = useState(null)

  const pageSize = 20

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchIndex({ isAdmin, devMode, page, pageSize })
      setRows(data.data ?? [])
      setSeries(data.series ?? {})
      setTotal(data.total ?? 0)
      onLoaded?.()
    } catch (e) {
      message.error(String(e))
    } finally {
      setLoading(false)
    }
  }, [isAdmin, devMode, page, message, onLoaded])

  useEffect(() => {
    load()
  }, [load, reloadKey])

  // The filter is client side and deliberately so: it narrows the page in
  // front of you as you type, with no round trip. Finding a link that is
  // not on this page is what the pager is for.
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return rows
    return rows.filter(
      (r) =>
        r.alias.toLowerCase().includes(q) ||
        (r.url ?? '').toLowerCase().includes(q),
    )
  }, [rows, query])

  const columns = [
    {
      title: 'Alias',
      dataIndex: 'alias',
      render: (_, r) => (
        <Typography.Text
          copyable={{ text: aliasURL(r.alias), tooltips: ['Copy link', 'Copied'] }}
          style={{ fontFamily: mono, fontSize: 13 }}
        >
          {aliasPath(r.alias)}
        </Typography.Text>
      ),
    },
    {
      title: 'Target',
      dataIndex: 'url',
      render: (url) =>
        url ? (
          <Tooltip title={url} mouseEnterDelay={0.4}>
            <span
              style={{
                fontFamily: mono,
                fontSize: 12,
                color: tokens.textDim,
                display: 'block',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                maxWidth: 320,
              }}
            >
              {targetLabel(url)}
            </span>
          </Tooltip>
        ) : (
          <span style={{ color: tokens.textFaint }}>—</span>
        ),
    },
  ]

  if (isAdmin) {
    columns.push(
      {
        title: 'PV',
        dataIndex: 'pv',
        width: 92,
        align: 'right',
        sorter: (a, b) => a.pv - b.pv,
        render: (v) => <span className="num">{fmt(v)}</span>,
      },
      {
        title: 'UV',
        dataIndex: 'uv',
        width: 92,
        align: 'right',
        sorter: (a, b) => a.uv - b.uv,
        render: (v) => (
          <span className="num" style={{ color: tokens.textDim }}>
            {fmt(v)}
          </span>
        ),
      },
      {
        title: '14 days',
        key: 'spark',
        width: 100,
        render: (_, r) => <Spark data={series[r.alias] ?? []} />,
      },
      {
        title: '',
        key: 'flags',
        width: 76,
        render: (_, r) => (
          <span style={{ display: 'flex', gap: 6 }}>
            {r.private && (
              <Tooltip title="Not listed on the public index">
                <span style={{ color: tokens.textFaint, fontSize: 11 }}>private</span>
              </Tooltip>
            )}
            {!r.trust && (
              <Tooltip title="Shows a warning before redirecting off-site">
                <span style={{ color: tokens.warn, fontSize: 11 }}>warn</span>
              </Tooltip>
            )}
          </span>
        ),
      },
      {
        title: '',
        key: 'actions',
        width: 92,
        align: 'right',
        render: (_, r) => (
          <span style={{ display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
            <a onClick={() => setEditing(r)}>Edit</a>
            <a
              style={{ color: tokens.textDim }}
              onClick={() =>
                modal.confirm({
                  title: `Delete ${aliasPath(r.alias)}?`,
                  content:
                    'The link stops working. Its recorded visits are kept.',
                  okText: 'Delete',
                  okButtonProps: { danger: true },
                  onOk: async () => {
                    const err = await remove(r.alias)
                    if (err) {
                      message.error(err)
                      return
                    }
                    message.success(`${r.alias} deleted`)
                    load()
                  },
                })
              }
            >
              Delete
            </a>
          </span>
        ),
      },
    )
  }

  return (
    <>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: tokens.space(3),
          padding: `${tokens.space(4)}px 0 ${tokens.space(3)}px`,
        }}
      >
        <Input
          allowClear
          size="small"
          placeholder="Filter this page"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          style={{ maxWidth: 260, fontFamily: mono }}
        />
        <span style={{ color: tokens.textFaint, fontSize: 12 }}>
          {query
            ? `${visible.length} of ${rows.length} shown`
            : `${fmt(total)} links`}
        </span>
      </div>

      <Table
        size="small"
        rowKey="alias"
        loading={loading}
        columns={columns}
        dataSource={visible}
        pagination={{
          current: page,
          pageSize,
          total,
          onChange: setPage,
          showSizeChanger: false,
          size: 'small',
          hideOnSinglePage: true,
        }}
        expandable={
          isAdmin && statsMode
            ? {
                expandedRowRender: (r) => (
                  <Stats alias={r.alias} devMode={devMode} />
                ),
                expandRowByClick: true,
              }
            : undefined
        }
      />

      {editing && (
        <LinkForm
          record={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null)
            load()
          }}
        />
      )}
    </>
  )
}

export default LinkTable
