// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useEffect, useState } from 'react'
import { theme } from 'antd'

// The console's colors, for the components antd renders.
//
// Everything the console draws itself reads the custom properties in
// index.css, which follow the system's light or dark setting on their
// own. antd computes its shades in script, so it needs the same values as
// plain strings, and needs telling which of the two is in use.
export const palette = {
  light: {
    page: '#f9f9f7',
    surface: '#fcfcfb',
    ink: '#0b0b0b',
    ink2: '#52514e',
    muted: '#898781',
    border: 'rgba(11, 11, 11, 0.1)',
    accent: '#2a78d6',
    down: '#b3261e',
  },
  dark: {
    page: '#0d0d0d',
    surface: '#1a1a19',
    ink: '#ffffff',
    ink2: '#c3c2b7',
    muted: '#898781',
    border: 'rgba(255, 255, 255, 0.1)',
    accent: '#3987e5',
    down: '#e66767',
  },
}

export const sans = 'system-ui, -apple-system, "Segoe UI", sans-serif'

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

// useDark follows the system's light or dark setting.
export const useDark = () => useMedia('(prefers-color-scheme: dark)')

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
