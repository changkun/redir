// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { App, DatePicker, Form, Input, Modal, Segmented } from 'antd'
import dayjs from 'dayjs'
import { mono, tokens } from '../theme'
import { rfc3339 } from '../lib/time'
import { save } from '../lib/api'

// LinkForm creates and edits a link.
//
// Editing happens here rather than inline in the table. A row is for
// reading: putting eleven inputs into it is what made the old table
// unreadable, and it left no room to say what a field means.
const LinkForm = ({ record, onClose, onSaved }) => {
  const { message } = App.useApp()
  const [form] = Form.useForm()
  const creating = !record?.alias

  const initial = {
    alias: record?.alias ?? '',
    url: record?.url ?? '',
    private: record?.private ? 'private' : 'public',
    trust: record?.trust ? 'trusted' : 'warn',
    valid_from: record?.valid_from ? dayjs(record.valid_from) : null,
  }

  const submit = async () => {
    // A form that does not validate is an ordinary outcome, not a
    // failure: the fields already say what is wrong. Letting the
    // rejection escape only puts it in the console.
    let v
    try {
      v = await form.validateFields()
    } catch {
      return
    }
    const err = await save(creating ? 'create' : 'update', record?.alias, {
      alias: v.alias,
      url: v.url,
      private: v.private === 'private',
      trust: v.trust === 'trusted',
      valid_from: rfc3339(v.valid_from),
    })
    if (err) {
      message.error(err)
      return
    }
    message.success(creating ? `${v.alias} created` : `${v.alias} saved`)
    onSaved()
  }

  return (
    <Modal
      open
      title={creating ? 'New link' : `Edit ${record.alias}`}
      okText={creating ? 'Create' : 'Save'}
      onOk={submit}
      onCancel={onClose}
      destroyOnHidden
      width={520}
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={initial}
        requiredMark={false}
        style={{ marginTop: tokens.space(4) }}
      >
        <Form.Item
          name="alias"
          label="Alias"
          extra="The path visitors use. Slashes are allowed, so news/2026 works."
          rules={[{ required: true, message: 'An alias is required' }]}
        >
          <Input style={{ fontFamily: mono }} placeholder="blog" />
        </Form.Item>

        <Form.Item
          name="url"
          label="Target"
          extra="Where the alias sends people."
          rules={[{ required: true, message: 'A target is required' }]}
        >
          <Input style={{ fontFamily: mono }} placeholder="https://example.com" />
        </Form.Item>

        <Form.Item
          name="private"
          label="Listing"
          extra="A private link works, it is simply not shown on the public index."
        >
          <Segmented
            options={[
              { label: 'Public', value: 'public' },
              { label: 'Private', value: 'private' },
            ]}
          />
        </Form.Item>

        <Form.Item
          name="trust"
          label="External redirects"
          extra="An untrusted link shows a warning page before leaving the site."
        >
          <Segmented
            options={[
              { label: 'Redirect directly', value: 'trusted' },
              { label: 'Warn first', value: 'warn' },
            ]}
          />
        </Form.Item>

        <Form.Item
          name="valid_from"
          label="Valid from"
          extra="Leave empty for always. Before this time the link shows a countdown."
        >
          <DatePicker showTime style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Modal>
  )
}

export default LinkForm
