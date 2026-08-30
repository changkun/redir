// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { App, Button } from 'antd';
import {
  ModalForm,
  ProForm,
  ProFormText,
  ProFormSelect,
  ProFormDateTimePicker,
} from '@ant-design/pro-components';
import { PlusOutlined } from '@ant-design/icons';
import { aliasPath, aliasURL, basePath } from '../lib/paths';
import { rfc3339 } from '../lib/time';

const RedirCreate = (props) => {
  const { message } = App.useApp()
  const ref = props.refreshRef

  return (
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
        onCancel: () => console.log('nothing to do. really ;-)'),
      }}
      onFinish={async (values) => {
        const resp = await fetch(basePath() + '/', {
          method: 'POST',
          headers: {
            'Accept': 'application/json',
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            op: 'create',
            data: {
              alias: values.alias,
              url: values.url,
              private: values.private === 'true',
              trust: values.trust === 'true',
              valid_from: rfc3339(values.valid_from),
            }
          })
        })
        if (!resp.ok) {
          const data = await resp.json()
          message.error(data.message)
          return false
        }
        message.success(`Short link ${aliasPath(values.alias)} is created and has been saved to your clipboard!`, 10)
        navigator.clipboard.writeText(aliasURL(values.alias))
        ref.current.reload() // refresh table.
        return true
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
              pattern: /^[\w-][\w\-. /]+$/,
              message: 'Please input a valid alias',
            },
          ]}
          width="md"
          name="alias"
          label="Alias"
          placeholder="Please input an alias"
          tooltip="A meaningful alias can help visitor recognize the content behind the link directly. Example: alias 'example' represents /s/example router."
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
      <ProFormSelect
        options={[{
            value: 'false',
            label: 'Untrusted',
          },
          {
            value: 'true',
            label: 'Trusted',
          },
        ]}
        width="xs"
        name="trust"
        label="Trustable"
        placeholder="Please select visibility"
        tooltip="Trusted alias will skip the privacy warning regarding external links to the visitor. Same origin URLs will always conduct the redirects and do not effected by this field (Default: Untrusted)."
      />
      <ProFormDateTimePicker
        name="valid_from"
        label="Valid from"
        placeholder="Please select accessible time"
        tooltip="The shortened link is avaliable since the time specified. Before the specified time, the link shows a countdown page."
      />
      </ProForm.Group>
    </ModalForm>
  )
}

export default RedirCreate