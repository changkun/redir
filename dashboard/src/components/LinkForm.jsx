// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useState } from 'react'
import { DatePicker, Form, Input, Modal } from 'antd'
import dayjs from 'dayjs'
import Seg from './Seg'
import { rfc3339 } from '../lib/time'
import { save } from '../lib/api'

// LinkForm creates and edits a link.
//
// Editing happens here rather than inline in the table. A row is for
// reading: putting eleven inputs into it is what made the old table
// unreadable, and it left no room to say what a field means.
const LinkForm = ({ record, onClose, onSaved }) => {
  const [form] = Form.useForm()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
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
    setBusy(true)
    const err = await save(creating ? 'create' : 'update', record?.alias, {
      alias: v.alias,
      url: v.url,
      private: v.private === 'private',
      trust: v.trust === 'trusted',
      valid_from: rfc3339(v.valid_from),
    })
    setBusy(false)
    if (err) {
      // The refusal is shown where the fields are, so it can be fixed
      // without the dialog closing.
      setError(err)
      return
    }
    onSaved(v.alias)
  }

  return (
    <Modal
      open
      centered
      title={creating ? 'New link' : `Edit ${record.alias}`}
      onCancel={onClose}
      destroyOnHidden
      width={520}
      footer={
        <>
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="btn primary"
            disabled={busy}
            onClick={submit}
          >
            {creating ? 'Create' : 'Save'}
          </button>
        </>
      }
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={initial}
        requiredMark={false}
        style={{ marginTop: 12 }}
      >
        <Form.Item
          name="alias"
          label="Alias"
          extra="The path visitors use. Slashes are allowed, so news/2026 works."
          rules={[{ required: true, message: 'An alias is required' }]}
        >
          <Input placeholder="blog" />
        </Form.Item>

        <Form.Item
          name="url"
          label="Target"
          extra="Where the alias sends people."
          rules={[{ required: true, message: 'A target is required' }]}
        >
          <Input placeholder="https://example.com" />
        </Form.Item>

        <Form.Item
          name="private"
          label="Listing"
          extra="A private link works, it is simply not shown on the public index."
        >
          <Seg
            label="Listing"
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
          <Seg
            label="External redirects"
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
      {error && (
        <p className="status error" role="alert" style={{ margin: 0 }}>
          {error}
        </p>
      )}
    </Modal>
  )
}

export default LinkForm
