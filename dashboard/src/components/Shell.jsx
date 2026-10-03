// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Shell is the frame every view sits in: which site you are looking at,
// the way in or out, and where the legal pages are.
//
// The site name is shown rather than chosen. One process serves several
// hosts, but which one you administer is decided by the address you came
// in on, and a switcher would imply otherwise.
const Shell = ({ site, isAdmin, logoutURL, children, footer }) => (
  <main>
    <header className="top">
      <h1>
        <a href=".">redir</a>
      </h1>
      <p>Short links on {site}</p>
      <div className="who">
        {isAdmin ? (
          <>
            <a className="link wide" href=".">
              Public index
            </a>
            <a className="link" href={logoutURL || window.location.pathname}>
              Sign out
            </a>
          </>
        ) : (
          <a className="link" href={window.location.pathname + '?mode=admin'}>
            Sign in
          </a>
        )}
        <a className="wide" href="https://github.com/changkun/redir">
          GitHub
        </a>
      </div>
    </header>

    {children}

    <p className="foot">{footer}</p>
  </main>
)

export default Shell
