// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useState } from 'react'
import { EditableProTable } from '@ant-design/pro-components'
import { App, Typography } from 'antd'
import './RedirTable.css'
import Stats from './Stats'
import { aliasPath, aliasURL, basePath } from '../lib/paths'
import { rfc3339 } from '../lib/time'

const post = async (body) =>
  fetch(basePath() + '/', {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  })

const RedirTable = (props) => {
  const { message } = App.useApp()
  const refreshRef = props.refreshRef
  const [editableKeys, setEditableRowKeys] = useState([])
  const [dataSource, setDataSource] = useState([])

  let columns = [
    {
      title: 'Alias',
      dataIndex: 'alias',
      // The cell shows the path but copies the absolute link, which is
      // what a visitor needs. Rendering our own Typography.Text keeps the
      // copy text derived from the record instead of reaching into the
      // props of an element the table already built.
      render: (_, record) => (
        <Typography.Text copyable={{ text: aliasURL(record.alias) }}>
          {aliasPath(record.alias)}
        </Typography.Text>
      ),
      width: '15%',
      tip: 'A meaningful string can help visitor recognize the content behind the link directly. Example: alias "an/example" represents /s/an/example router.',
    },
    {
      title: 'URL',
      key: 'url',
      dataIndex: 'url',
      valueType: 'string',
      width: '30%',
      ellipsis: true,
      tip: 'The actual URL to be redirect via the shortened alias.',
    },
  ]
  if (props.isAdmin) {
    columns.unshift({
      title: 'PV/UV',
      dataIndex: 'visits',
      hideInSearch: true,
      editable: false,
      tip: 'Page visit (PV) count and user visit (UV) count.',
    })
    columns.push(...[
      {
        title: 'Visibility',
        key: 'private',
        dataIndex: 'private',
        valueType: 'select',
        valueEnum: {
          true: { text: 'Private' },
          false: { text: 'Public' },
        },
        tip: 'Public alias will be listed on the public index page (/s).',
      },
      {
        title: 'Trustable',
        key: 'trust',
        dataIndex: 'trust',
        valueType: 'select',
        valueEnum: {
          true: { text: 'Trusted' },
          false: { text: 'Untrusted' },
        },
        tip: 'Trusted alias will skip the privacy warning page regarding external links to the visitor. Same origin URLs will always conduct the redirects and do not effected by this field.',
      },
      {
        title: 'Valid from',
        dataIndex: 'valid_from',
        valueType: 'dateTime',
        hideInSearch: true,
        tip: 'The shortened link is avaliable since the time specified. Before the specified time, the link shows a countdown page.',
      },
      {
        title: 'Created By',
        dataIndex: 'created_by',
        hideInSearch: true,
        editable: false,
        tip: 'The person who created this alias.',
      },
      {
        title: 'Updated By',
        dataIndex: 'updated_by',
        hideInSearch: true,
        editable: false,
        tip: 'The person who updated this alias lately.',
      },
      {
        title: 'Operation',
        valueType: 'option',
        render: (text, record, _, action) => [
          <a key='editable' onClick={() => {
              action.startEditable?.(record.alias);
          }}>Edit</a>
        ],
      },
    ])
  }

  const pageSize = 18

  const expandedRowRender = (params) => {
    return <Stats alias={params.alias} devMode={props.devMode}/>
  }
  return (
    <EditableProTable
      actionRef={refreshRef}
      rowKey='alias'
      recordCreatorProps={false}
      columns={columns}
      pagination={{pageSize: pageSize}}
      expandable={props.isAdmin && props.statsMode ? { expandedRowRender } : false}
      request={async (params) => {
        const mode = props.isAdmin ? 'index-pro' : 'index'

        const host = props.devMode ? 'http://localhost:9123' : window.location.origin
        const path = props.devMode ? '/s' : basePath()

        const url = `${host}${path}/?mode=${mode}&pn=${params.current}&ps=${params.pageSize}`
        const resp = await fetch(url, {
          method: 'GET',
        })
        const redirs = await resp.json()
        for (let i = 0; i < redirs.data.length; i++) {
          if (!props.isAdmin) {
            redirs.data[i].url = window.location.host + `${path}/` + redirs.data[i].alias
          } else {
            redirs.data[i].private = redirs.data[i].private ? 'true' : 'false'
            redirs.data[i].trust = redirs.data[i].trust ? 'true' : 'false'
            if (redirs.data[i].valid_from === '0001-01-01T00:00:00Z') {
              redirs.data[i].valid_from = null
            }
            redirs.data[i].visits = `${redirs.data[i].pv}/${redirs.data[i].uv}`
          }
        }
        return redirs
      }}
      value={dataSource}
      onChange={setDataSource}
      editable={{
        type: 'multiple',
        deletePopconfirmMessage: 'Are your sure?',
        editableKeys,
        actionRender: (row, config, defaultDom) => [defaultDom.save, defaultDom.cancel, defaultDom.delete],
        onSave: async (alias, row) => {
          const resp = await post({
            op: 'update',
            alias: alias,
            data: {
              alias: row.alias,
              url: row.url,
              private: row.private === 'true',
              trust: row.trust === 'true',
              valid_from: rfc3339(row.valid_from),
            },
          })
          if (!resp.ok) {
            const data = await resp.json()
            message.error(data.message)
            return false
          }
          message.success('Update success!', 10)
          refreshRef.current.reload()
        },
        onChange: setEditableRowKeys,
        onDelete: async (alias) => {
          const resp = await post({op: 'delete', alias: alias})
          if (!resp.ok) {
            const data = await resp.json()
            message.error(data.message)
            return false
          }
          message.success('Delete success!', 10)
          refreshRef.current.reload()
        },
      }}
    />
  )
}

export default RedirTable
