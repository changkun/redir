// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { App as AntApp, ConfigProvider, theme } from 'antd'
import enUS from 'antd/locale/en_US'
import Home from './components/Home'
import { antdTheme } from './theme'

// The server renders index.html and fills these attributes on #root.
const flag = (root, name) => root.getAttribute(name) === 'true'

// One provider tree for the whole console. ConfigProvider carries the
// theme and locale; AntApp puts message and modal under the same theme
// context as everything else, so a dialog is not lit differently from
// the page behind it.
const App = () => {
  const root = document.getElementById('root')
  return (
    <ConfigProvider locale={enUS} theme={antdTheme(theme.darkAlgorithm)}>
      <AntApp>
        <Home
          site={window.location.host}
          isAdmin={flag(root, 'is-admin')}
          statsMode={flag(root, 'stats-mode')}
          devMode={flag(root, 'dev-mode')}
          showImpressum={flag(root, 'show-impressum')}
          showPrivacy={flag(root, 'show-privacy')}
          showContact={flag(root, 'show-contact')}
          logoutURL={root.getAttribute('logout-url') || ''}
        />
      </AntApp>
    </ConfigProvider>
  )
}

export default App
