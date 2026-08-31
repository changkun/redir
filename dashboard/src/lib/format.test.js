// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { describe, expect, it } from 'vitest'
import { targetLabel } from './format'

describe('targetLabel', () => {
  it('drops the scheme and keeps host and path', () => {
    expect(targetLabel('https://blog.changkun.de/')).toBe('blog.changkun.de')
    expect(targetLabel('https://github.com/golang-design/lockfree')).toBe(
      'github.com/golang-design/lockfree',
    )
  })

  it('drops the query, which is rarely what identifies a link', () => {
    expect(targetLabel('https://youtube.com/watch?v=abc&list=xyz')).toBe(
      'youtube.com/watch',
    )
  })

  // Production holds mailto: targets, one of them obfuscated as
  // "mailto:hi[at]golang.design". There is no host to strip, and
  // mangling it would misrepresent where the link goes.
  it('leaves an address alone', () => {
    expect(targetLabel('mailto:research@changkun.de')).toBe(
      'mailto:research@changkun.de',
    )
    expect(targetLabel('mailto:hi[at]golang.design')).toBe(
      'mailto:hi[at]golang.design',
    )
  })

  it('survives something that is not a URL', () => {
    expect(targetLabel('not a url')).toBe('not a url')
    expect(targetLabel('')).toBe('')
    expect(targetLabel(null)).toBe('')
  })
})
