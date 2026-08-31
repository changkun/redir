// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// The console's design decisions, in one place.
//
// Every value a component would otherwise invent lives here, so density,
// type and colour are decided once instead of per file. The palette is
// deliberately short: a console earns its clarity from spacing and
// alignment, and spends colour only on saying something is unusual.

export const tokens = {
  // A near-black ground rather than pure black, so panels can sit above
  // it without a border.
  bg: '#0b0c0e',
  surface: '#121316',
  surfaceHover: '#17181c',
  // Hairlines instead of cards. A box around every group triples the ink
  // and adds no information.
  line: '#232529',
  lineStrong: '#2e3137',

  text: '#e6e7e9',
  textDim: '#9aa0a6',
  textFaint: '#6b7176',

  // One accent, for state and focus only, never for decoration.
  accent: '#4c8dff',
  good: '#3fb950',
  warn: '#d29922',
  bad: '#f85149',

  radius: 6,
  // A 4px rhythm. Every gap is a multiple, so nothing is "almost aligned".
  space: (n) => n * 4,
}

// Identifiers read better in a monospace: an alias and a URL are things
// you compare character by character, not prose.
export const mono =
  "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace"

export const sans =
  "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif"

// antd's theme, driven from the same tokens so the components it does
// render sit inside the design rather than beside it.
export const antdTheme = (algorithm) => ({
  algorithm,
  token: {
    colorPrimary: tokens.accent,
    colorBgBase: tokens.bg,
    colorTextBase: tokens.text,
    colorBorder: tokens.line,
    colorBorderSecondary: tokens.line,
    borderRadius: tokens.radius,
    fontFamily: sans,
    fontSize: 13,
    controlHeight: 30,
  },
  components: {
    Table: {
      // The console is meant to be read a page at a time.
      cellPaddingBlockSM: 6,
      cellPaddingInlineSM: 12,
      headerBg: 'transparent',
      headerColor: tokens.textDim,
      headerSplitColor: 'transparent',
      borderColor: tokens.line,
      rowHoverBg: tokens.surfaceHover,
      bodySortBg: 'transparent',
      headerSortActiveBg: 'transparent',
    },
    Layout: {
      headerBg: tokens.bg,
      bodyBg: tokens.bg,
      footerBg: tokens.bg,
      headerHeight: 48,
      headerPadding: '0 20px',
    },
    Modal: { contentBg: tokens.surface, headerBg: tokens.surface },
    Card: { colorBgContainer: tokens.surface },
    Input: { colorBgContainer: tokens.surface },
    Select: { colorBgContainer: tokens.surface },
    DatePicker: { colorBgContainer: tokens.surface },
    Button: { defaultBg: tokens.surface, defaultBorderColor: tokens.line },
  },
})
