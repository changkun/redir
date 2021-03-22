// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import React from 'react';
import { ConfigProvider, Button, message } from 'antd';
import ProForm, {
  ModalForm,
  ProFormText,
  ProFormSelect,
  ProFormDateTimePicker,
} from '@ant-design/pro-form';
import enUS from 'antd/lib/locale/en_US'
import { PlusOutlined } from '@ant-design/icons';

const waitTime = (time = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve(true)
    }, time)
  })
}

const RedirCreate = () => {
  return (
    <ConfigProvider locale={enUS}>
    <ModalForm
      title="Create A New Short Link"
      submitter={{
        searchConfig: {
          submitText: 'Confirm',
          resetText: 'Cancel',
        },
      }}
      trigger={
        <Button type="primary" style={{ margin: '20px 0' }}>
        <PlusOutlined />
        Create
        </Button>
      }
      modalProps={{
        onCancel: () => console.log('run'),
      }}
      onFinish={async (values) => {
        await waitTime(200);
        console.log(values);
        message.success('Success');
        return true;
      }}
    >
      <ProForm.Group>
        <ProFormText
          rules={[
            {
              required: true,
              message: 'Please input an alias',
            },
            {
              pattern: /^[\w\-][\w\-. \/]+$/,
              message: 'Please input a valid alias',
            },
          ]}
          width="md"
          name="alias"
          label="Alias"
          placeholder="Please input an alias"
          tooltip="A meaningful alias can help visitor recognize the content behind the link directly."
        />
      </ProForm.Group>
      <ProForm.Group>
        <ProFormText
          rules={[
            {
              required: true,
              message: 'Please input an URL',
            },
            {
              pattern: /(https?:\/\/(?:www\.|(?!www))[a-zA-Z0-9][a-zA-Z0-9-]+[a-zA-Z0-9]\.[^\s]{2,}|www\.[a-zA-Z0-9][a-zA-Z0-9-]+[a-zA-Z0-9]\.[^\s]{2,}|https?:\/\/(?:www\.|(?!www))[a-zA-Z0-9]+\.[^\s]{2,}|www\.[a-zA-Z0-9]+\.[^\s]{2,})/,
              message: 'Please input a valid URL',
            },
          ]}
          width="440px"
          name="url"
          label="URL"
          placeholder="Please input the actual URL"
          tooltip="The actual URL to be redirect via the shortened alias."
        />
      </ProForm.Group>
      <ProForm.Group>
      <ProFormSelect
          options={[{
              value: 'false',
              label: 'Public',
            },
            {
              value: 'true',
              label: 'Private',
            },
          ]}
          width="xs"
          name="private"
          label="Visibility"
          placeholder="Please select visibility"
          tooltip="Public alias will be listed on the public index page (Default: Public)."
        />
        <ProFormDateTimePicker
          name="contractTime"
          label="Accessible from"
          placeholder="Please select accessible time"
          tooltip="The shortened link is avaliable since the time you specified. Before the specified time, the link shows a countdown page."
        />
      </ProForm.Group>
    </ModalForm>
    </ConfigProvider>
  )
}

export default RedirCreate