// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useEffect, useState } from 'react'
import { theme } from 'antd'

// The console's colors, for the components antd renders.
//
// Everything the console draws itself reads the custom properties in
// index.css, which follow the theme on the root on their own. antd
// computes its shades in script, so it needs the same values as plain
// strings, and needs telling which of the two is in use. A dark dialog
// sits a step above the page, as a raised surface does.
export const palette = {
  light: {
    page: '#ffffff',
    surface: '#ffffff',
    ink: '#0d0d0d',
    ink2: '#525252',
    muted: '#666666',
    border: 'rgba(0, 0, 0, 0.14)',
    accent: '#38538f',
    down: '#b3261e',
  },
  dark: {
    page: '#161616',
    surface: '#1d1d1d',
    ink: '#ececec',
    ink2: '#b4b4b4',
    muted: '#9a9a9a',
    border: 'rgba(255, 255, 255, 0.14)',
    accent: '#9bb0e4',
    down: '#e66767',
  },
}

export const sans =
  'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif'

// useMedia says whether a media query holds, and keeps saying so as the
// answer changes while the console is open.
export const useMedia = (query) => {
  const [holds, setHolds] = useState(() => window.matchMedia(query).matches)
  useEffect(() => {
    const media = window.matchMedia(query)
    const change = (e) => setHolds(e.matches)
    setHolds(media.matches)
    media.addEventListener('change', change)
    return () => media.removeEventListener('change', change)
  }, [query])
  return holds
}

// The theme on the root: 'light' or 'dark' once index.html has set it,
// from the reader's choice or the system's setting.
const rootTheme = () => document.documentElement.getAttribute('data-theme')

// useDark says whether the page is dark: the theme on the root, which the
// toggle and a change of the system's setting both move, or the system's
// setting where nothing has set one.
export const useDark = () => {
  const system = useMedia('(prefers-color-scheme: dark)')
  const [chosen, setChosen] = useState(rootTheme)
  useEffect(() => {
    const watch = new MutationObserver(() => setChosen(rootTheme()))
    watch.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    })
    return () => watch.disconnect()
  }, [])
  return chosen ? chosen === 'dark' : system
}

// toggleTheme turns the page to the other theme and keeps the choice under
// the key changkun.de's other pages keep it under, so it holds on all of
// them: they share an origin with the console.
export const toggleTheme = () => {
  const root = document.documentElement
  const dark = root.hasAttribute('data-theme')
    ? root.getAttribute('data-theme') === 'dark'
    : window.matchMedia('(prefers-color-scheme: dark)').matches
  const next = dark ? 'light' : 'dark'
  root.setAttribute('data-theme', next)
  try {
    localStorage.setItem('theme', next)
  } catch {
    // Private browsing may refuse storage; the page still turns.
  }
}

// useNarrow is a phone's width, the same one index.css reflows at.
export const useNarrow = () => useMedia('(max-width: 820px)')

// antdTheme dresses the dialogs, their fields and the date pickers, which
// are the parts antd still renders, in the console's design.
export const antdTheme = (dark) => {
  const c = dark ? palette.dark : palette.light
  return {
    algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorPrimary: c.accent,
      colorError: c.down,
      colorBgBase: c.surface,
      colorBgContainer: c.page,
      colorBgElevated: c.surface,
      colorBgMask: 'rgba(0, 0, 0, 0.45)',
      colorTextBase: c.ink,
      colorText: c.ink,
      colorTextSecondary: c.ink2,
      colorTextTertiary: c.ink2,
      colorTextDescription: c.ink2,
      colorTextPlaceholder: c.muted,
      colorBorder: c.border,
      colorBorderSecondary: c.border,
      borderRadius: 6,
      fontFamily: sans,
      fontSize: 13,
      controlHeight: 28,
    },
    components: {
      Modal: { contentBg: c.surface, headerBg: c.surface, footerBg: c.surface },
    },
  }
}
