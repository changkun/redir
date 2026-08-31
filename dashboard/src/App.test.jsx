// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import App from './App'

// The console replaced every component at once, so the suite renders it.
// A missing import or a crash on first paint blanks the page, and neither
// the build nor a unit test on a helper would notice.

const mount = (attrs) => {
  const root = document.createElement('div')
  root.id = 'root'
  for (const [k, v] of Object.entries(attrs)) {
    root.setAttribute(k, String(v))
  }
  document.body.append(root)
}

const respond = (body) =>
  Promise.resolve({ ok: true, json: () => Promise.resolve(body) })

const page = {
  data: [
    {
      alias: 'blog',
      url: 'https://blog.changkun.de/',
      pv: 31907,
      uv: 26839,
      private: false,
      trust: true,
      valid_from: '0001-01-01T00:00:00Z',
    },
  ],
  page: 1,
  total: 1,
  series: { blog: [1, 2, 3, 0, 5, 8, 13, 3, 2, 1, 0, 4, 6, 9] },
}

const overview = {
  links: 184,
  visits: 348465,
  people: 151006,
  bots: 197459,
  series: [{ day: '2026-08-30', pv: 812, uv: 403 }],
}

beforeEach(() => {
  vi.stubGlobal('fetch', (url) => {
    const u = String(url)
    if (u.includes('mode=overview')) return respond(overview)
    return respond(page)
  })
})

afterEach(() => {
  cleanup()
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('the console', () => {
  it('renders the admin view with its totals and links', async () => {
    mount({ 'is-admin': true, 'stats-mode': true, 'show-impressum': true })
    render(<App />)

    expect(await screen.findByText('184')).toBeTruthy() // links
    expect(await screen.findByText('348,465')).toBeTruthy() // visits
    expect(await screen.findByText('/blog')).toBeTruthy()
    // The share is derived, not sent, so it is worth asserting.
    expect(await screen.findByText('56.7%')).toBeTruthy()
    expect(screen.getByText('New link')).toBeTruthy()
  })

  it('renders the public view without the console furniture', async () => {
    mount({ 'is-admin': false })
    render(<App />)

    expect(await screen.findByText('/blog')).toBeTruthy()
    // A visitor gets no totals, no create button and no account menu.
    expect(screen.queryByText('New link')).toBeNull()
    expect(screen.queryByText('Account')).toBeNull()
    expect(screen.queryByText('348,465')).toBeNull()
    expect(screen.getByText('Sign in')).toBeTruthy()
  })

  it('shows the legal links the server enabled, and no others', async () => {
    mount({ 'is-admin': false, 'show-impressum': true, 'show-privacy': false })
    render(<App />)

    expect(await screen.findByText('Impressum')).toBeTruthy()
    expect(screen.queryByText('Privacy')).toBeNull()
  })
})
