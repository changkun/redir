// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { Button, Dropdown } from 'antd'
import { tokens, mono } from '../theme'

// Shell is the frame every view sits in: who you are, which site you are
// looking at, and where the legal pages are.
//
// The site name is shown rather than chosen. One process serves several
// hosts, but which one you administer is decided by the address you came
// in on, and a switcher would imply otherwise.
const Shell = ({ site, isAdmin, logoutURL, actions, children, footer }) => (
  <div
    style={{
      minHeight: '100vh',
      display: 'flex',
      flexDirection: 'column',
      background: tokens.bg,
    }}
  >
    <header
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: tokens.space(3),
        height: 48,
        padding: `0 ${tokens.space(6)}px`,
        borderBottom: `1px solid ${tokens.line}`,
        position: 'sticky',
        top: 0,
        zIndex: 10,
        background: tokens.bg,
      }}
    >
      <a
        href="."
        style={{
          fontFamily: mono,
          fontSize: 14,
          fontWeight: 600,
          letterSpacing: '-0.01em',
        }}
      >
        redir
      </a>
      <span style={{ color: tokens.line }}>/</span>
      <span style={{ color: tokens.textDim, fontFamily: mono, fontSize: 13 }}>
        {site}
      </span>
      {isAdmin && (
        <span
          style={{
            fontSize: 11,
            color: tokens.accent,
            border: `1px solid ${tokens.line}`,
            borderRadius: 4,
            padding: '1px 6px',
          }}
        >
          admin
        </span>
      )}

      <div style={{ marginLeft: 'auto', display: 'flex', gap: tokens.space(2) }}>
        {actions}
        {isAdmin ? (
          <Dropdown
            menu={{
              items: [
                { key: 'public', label: <a href=".">Public index</a> },
                { type: 'divider' },
                {
                  key: 'logout',
                  danger: true,
                  label: <a href={logoutURL || window.location.pathname}>Sign out</a>,
                },
              ],
            }}
          >
            <Button size="small">Account</Button>
          </Dropdown>
        ) : (
          <Button
            size="small"
            type="primary"
            href={window.location.pathname + '?mode=admin'}
          >
            Sign in
          </Button>
        )}
      </div>
    </header>

    <main
      style={{
        flex: 1,
        width: '100%',
        maxWidth: 1180,
        margin: '0 auto',
        padding: `0 ${tokens.space(6)}px ${tokens.space(12)}px`,
      }}
    >
      {children}
    </main>

    <footer
      style={{
        borderTop: `1px solid ${tokens.line}`,
        padding: `${tokens.space(4)}px ${tokens.space(6)}px`,
        color: tokens.textFaint,
        fontSize: 12,
        display: 'flex',
        gap: tokens.space(4),
        justifyContent: 'center',
        flexWrap: 'wrap',
      }}
    >
      {footer}
    </footer>
  </div>
)

export default Shell
