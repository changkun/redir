// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useState } from 'react'
import { Modal } from 'antd'

// Confirm asks before something that cannot be taken back. It says what
// will happen and to what, and the button that does it is the only red
// thing on the page.
const Confirm = ({ title, text, what, detail, action, onConfirm, onClose }) => {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const run = async () => {
    setBusy(true)
    const err = await onConfirm()
    setBusy(false)
    if (err) setError(err)
  }

  return (
    <Modal
      open
      centered
      title={title}
      onCancel={onClose}
      width={460}
      closable={false}
      footer={
        <>
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="btn danger"
            disabled={busy}
            onClick={run}
          >
            {action}
          </button>
        </>
      }
    >
      <div className="confirm">
        <p>{text}</p>
        <div className="what">
          <span title={what}>{what}</span>
          {detail && <span title={detail}>{detail}</span>}
        </div>
        {error && (
          <p className="status error" role="alert" style={{ marginTop: 10 }}>
            {error}
          </p>
        )}
      </div>
    </Modal>
  )
}

export default Confirm
