import React, { useState } from 'react'
import { EditableProTable } from '@ant-design/pro-table'
import { ConfigProvider } from 'antd'
import enUS from 'antd/lib/locale/en_US'
import './RedirTable.css'
import Stats from './Stats'

const waitTime = (time = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => { resolve(true) }, time)
  })
}

const RedirTable = (props) => {
  const [editableKeys, setEditableRowKeys] = useState([])
  const [dataSource, setDataSource] = useState([])
  const [newRecord, setNewRecord] = useState({
    // id: (Math.random() * 1000000).toFixed(0),
  })

  let columns = [
    {
      title: 'Alias',
      dataIndex: 'alias',
      render: text => <a href={'/s/'+text}>{text}</a>,
      width: '20%',
    },
    {
      title: 'URL',
      key: 'url',
      dataIndex: 'url',
      render: text => {
        if (props.isAdmin) {
          return text
        }
        return <a
          href={window.location.protocol+'//'+text.props.copyable.text}
        >{text.props.copyable.text}</a>
      },
      valueType: 'string',
      width: '30%',
      copyable: true,
      ellipsis: true,
    },
  ]
  if (props.isAdmin) {
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
        rowKey='alias'
        recordCreatorProps={false}
        columns={columns}
        search={props.isAdmin ? {labelWidth: 'auto'} : false}
        pagination={{pageSize: pageSize}}
        expandable={{ expandedRowRender }}
        request={async (params) => {
          let headers = new Headers()
          headers.set('Authorization', 'Basic ' + btoa('changkun:redir'))

          const mode = props.isAdmin ? 'admin' : 'index'
          // const host = window.location.protocol + '//' + window.location.host
          const host = 'http://localhost:9123'
          const url = `${host}/s/?mode=${mode}&pn=${params.current}&ps=${params.pageSize}`
          const resp = await fetch(url, {
            method: 'GET', headers: headers,
          })
          const redirs = await resp.json()
          for (let i = 0; i < redirs.data.length; i++) {
            redirs.data[i].url = window.location.host + '/s/' + redirs.data[i].alias
            redirs.data[i].private = redirs.data[i].private ? 'true' : 'false'
            if (redirs.data[i].valid_from === '0001-01-01T00:00:00Z') {
              redirs.data[i].valid_from = null
            }
          }
          return redirs
        }}
        value={dataSource}
        onChange={setDataSource}
        editable={{
          type: 'multiple',
          editableKeys,
          onSave: async (params) => {
            await waitTime(20);
            setNewRecord({
              alias: '',
              // id: (Math.random() * 1000000).toFixed(0),
            });
            console.log('save: ', params, newRecord)
          },
          onChange: setEditableRowKeys,
        }}
      />
    </ConfigProvider>
  )
}

export default RedirTable
