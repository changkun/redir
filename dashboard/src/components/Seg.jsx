// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Seg is a choice between a few named options, shown all at once.
//
// It takes value and onChange, which is what a form field is handed, so
// it sits inside a Form.Item like any other input.
const Seg = ({ value, onChange, options, label, id }) => (
  <div className="seg" role="group" aria-label={label} id={id}>
    {options.map((o) => (
      <button
        type="button"
        key={o.value}
        aria-pressed={value === o.value}
        onClick={() => onChange?.(o.value)}
      >
        {o.label}
      </button>
    ))}
  </div>
)

export default Seg
