import React, { useState } from 'react';
import { EditableProTable } from '@ant-design/pro-table';
import { ConfigProvider } from 'antd';
import enUS from 'antd/lib/locale/en_US';

const waitTime = (time = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve(true);
    }, time);
  });
};

const defaultData = [
  {
    id: 624748504,
    alias: 'blog',
    url: 'https://blog.changkun.de',
    private: 'false',
    valid_since: '2020-05-26T09:42:56Z',
  },
  {
    id: 624691229,
    alias: 'github',
    url: 'https://github.com/changkun',
    private: 'false',
    valid_since: '2020-05-26T09:42:56Z',
  },
];

export default () => {
  const [editableKeys, setEditableRowKeys] = useState([]);
  const [dataSource, setDataSource] = useState([]);
  const [newRecord, setNewRecord] = useState({
    id: (Math.random() * 1000000).toFixed(0),
  });

  const columns = [
    {
      title: 'Alias',
      dataIndex: 'alias',
      render: text => <a href={'/s/'+text}>/s/{text}</a>,
      width: '20%',
    },
    {
      title: 'URL',
      key: 'url',
      dataIndex: 'url',
      valueType: 'string',
    },
    {
      title: 'Private',
      key: 'private',
      dataIndex: 'private',
      valueType: 'text',
    },
    {
      title: 'Valid from',
      dataIndex: 'valid_from',
      valueType: 'dateTime',
    },
    {
      title: 'Operation',
      valueType: 'option',
      width: 200,
      render: (text, record, _, action) => [
        <a key='editable' onClick={() => {
            action.startEditable?.(record.id);
        }}>Edit</a>
      ],
    },
  ];

  return (
    <ConfigProvider locale={enUS}>
      <EditableProTable
        rowKey = 'id'
        recordCreatorProps = {{
          position: 'top',
          record: newRecord,
          creatorButtonText: 'Add A New Alias'
        }}
        columns={columns}
        search={{
          labelWidth: 'auto',
        }}
        pagination={{
          pageSize: 50,
        }}
        request={async () => ({
          data: defaultData,
          total: 3,
        })}
        value={dataSource}
        onChange={setDataSource}
        editable={{
          type: 'multiple',
          editableKeys,
          onSave: async () => {
            await waitTime(20);
            setNewRecord({
              id: (Math.random() * 1000000).toFixed(0),
            });
          },
          onChange: setEditableRowKeys,
        }}
      />
    </ConfigProvider>
  );
};