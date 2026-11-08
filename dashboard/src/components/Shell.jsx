// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { toggleTheme } from '../theme'

// ThemeToggle turns the page between light and dark. The moon shows in
// the light theme and the sun in the dark, as on changkun.de's other
// pages; index.css shows the one for the theme on the root.
const ThemeToggle = () => (
  <button
    className="theme-toggle"
    type="button"
    aria-label="Toggle theme"
    onClick={toggleTheme}
  >
    <svg className="icon-moon" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"
      />
    </svg>
    <svg className="icon-sun" viewBox="0 0 24 24" aria-hidden="true">
      <g
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="5" />
        <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
      </g>
    </svg>
  </button>
)

// Shell is the frame every view sits in: the blog's header, with which
// site you are looking at, the way in or out and the theme, then the view,
// then where the legal pages are.
//
// The site name is shown rather than chosen. One process serves several
// hosts, but which one you administer is decided by the address you came
// in on, and a switcher would imply otherwise.
const Shell = ({ site, isAdmin, logoutURL, children, footer }) => (
  <>
    <header className="site-header">
      <div className="header-inner">
        <p className="brand">
          <a href=".">redir</a>
          <span className="sep">/</span>
          <span className="where">{site}</span>
        </p>
        <nav className="site-nav">
          {isAdmin ? (
            <>
              <a className="wide" href=".">
                Public index
              </a>
              <a href={logoutURL || window.location.pathname}>Sign out</a>
            </>
          ) : (
            <a href={window.location.pathname + '?mode=admin'}>Sign in</a>
          )}
          <a className="wide" href="https://github.com/changkun/redir">
            GitHub
          </a>
        </nav>
        <ThemeToggle />
      </div>
    </header>

    <main>
      {children}

      <p className="foot">{footer}</p>
    </main>
  </>
)

export default Shell
