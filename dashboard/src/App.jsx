// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { ConfigProvider } from 'antd'
import enUS from 'antd/locale/en_US'
import Home from './components/Home'
import { antdTheme, useDark } from './theme'

// The server renders index.html and fills these attributes on #root.
const flag = (root, name) => root.getAttribute(name) === 'true'

// One provider for the whole console. It carries the locale and the
// theme of the parts antd renders, the dialogs and the date pickers, in
// the light or dark the system asks for, so a dialog is not lit
// differently from the page behind it.
const App = () => {
  const root = document.getElementById('root')
  const dark = useDark()
  return (
    <ConfigProvider locale={enUS} theme={antdTheme(dark)}>
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
    </ConfigProvider>
  )
}

export default App
