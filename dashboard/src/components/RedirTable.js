// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import React, { useState } from 'react'
import { EditableProTable } from '@ant-design/pro-table'
import { ConfigProvider, message } from 'antd'
import enUS from 'antd/lib/locale/en_US'
import './RedirTable.css'
import Stats from './Stats'

const waitTime = (time = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => { resolve(true) }, time)
  })
}

const rfc3339 = (datestr) => {
  if (datestr === '' || datestr === null || datestr === undefined) {
    return null
  }

  const d = new Date(datestr)

  function pad(n) {
      return n < 10 ? "0" + n : n;
  }

  function timezoneOffset(offset) {
      var sign;
      if (offset === 0) {
          return "Z";
      }
      sign = (offset > 0) ? "-" : "+";
      offset = Math.abs(offset);
      return sign + pad(Math.floor(offset / 60)) + ":" + pad(offset % 60);
  }

  return d.getFullYear() + "-" +
      pad(d.getMonth() + 1) + "-" +
      pad(d.getDate()) + "T" +
      pad(d.getHours()) + ":" +
      pad(d.getMinutes()) + ":" +
      pad(d.getSeconds()) + 
      timezoneOffset(d.getTimezoneOffset());
}

const RedirTable = (props) => {
  const refreshRef = props.refreshRef
  const [editableKeys, setEditableRowKeys] = useState([])
  const [dataSource, setDataSource] = useState([])

  let columns = [
    {
      title: 'Short Link',
      dataIndex: 'alias',
      render: text => {
        const path = window.location.pathname.endsWith('/') ?
          window.location.pathname.slice(0, -1) :
          window.location.pathname

        text.props.copyable.text = window.location.origin + path +'/' + text.props.copyable.text
        return <span>/s/{text}</span>
      },
      width: '15%',
      copyable: true,
    },
    {
      title: 'URL',
      key: 'url',
      dataIndex: 'url',
      valueType: 'string',
      width: '30%',
      ellipsis: true,
    },
  ]
  if (props.isAdmin) {
    columns.unshift({
      title: 'PV/UV',
      dataIndex: 'visits',
      hideInSearch: true,
      editable: false,
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
      },
      {
        title: 'Valid from',
        dataIndex: 'valid_from',
        valueType: 'dateTime',
        hideInSearch: true,
      },
      {
        title: 'Operation',
        valueType: 'option',
        render: (text, record, _, action) => [
          /* eslint-disable-next-line jsx-a11y/anchor-is-valid */
          <a key='editable' onClick={() => {
              action.startEditable?.(record.alias);
          }}>Edit</a>
        ],
      },
    ])
  }

  let pageSize = 10
  const expandedRowRender = (params) => {
    return <Stats alias={params.alias}/>
  }
  return (
    <ConfigProvider locale={enUS}>
      <EditableProTable
        actionRef={refreshRef}
        rowKey='alias'
        recordCreatorProps={false}
        columns={columns}
        pagination={{pageSize: pageSize}}
        expandable={props.isAdmin && props.statsMode ? { expandedRowRender } : false}
        request={async (params) => {
          const mode = props.isAdmin ? 'index-pro' : 'index'
          const host = window.location.origin
          const path = window.location.pathname.endsWith('/') ?
            window.location.pathname.slice(0, -1) :
            window.location.pathname;

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
          onSave: async (alias, row) => {
            await waitTime(20)

            const path = window.location.pathname.endsWith('/') ?
              window.location.pathname.slice(0, -1) :
              window.location.pathname

            const data = {
              op: 'update',
              alias: alias,
              data: {
                alias: row.alias,
                url: row.url,
                private: row.private === 'true' ? true : false,
                valid_from: row.valid_from === null ? null : (
                  (typeof row.valid_from) === 'string' ? rfc3339(row.valid_from) : row.valid_from.format()
                )
              },
            }

            const resp = await fetch(path+'/', {
              method: 'POST',
              headers: {
                'Accept': 'application/json',
                'Content-Type': 'application/json',
              },
              body: JSON.stringify(data)
            })

            if (!resp.ok) {
              const data = await resp.json()
              message.error(data.message)
              return false
            }
            message.success(`Update success!`, 10)

            await waitTime(20)

            refreshRef.current.reload()
          },
          onChange: setEditableRowKeys,
          onDelete: async (alias) => {
            await waitTime(20)

            const path = window.location.pathname.endsWith('/') ?
              window.location.pathname.slice(0, -1) :
              window.location.pathname
            const data = {
              op: 'delete',
              alias: alias,
            }
            console.log(data)
            const resp = await fetch(path+'/', {
              method: 'POST',
              headers: {
                'Accept': 'application/json',
                'Content-Type': 'application/json',
              },
              body: JSON.stringify(data)
            }).catch(err => {
              console.error(err)
            })
            if (!resp.ok) {
              const data = await resp.json()
              message.error(data.message)
              return false
            }
            message.success(`Delete success!`, 10)

            await waitTime(20)
            refreshRef.current.reload()
          },
        }}
      />
    </ConfigProvider>
  )
}

export default RedirTable
