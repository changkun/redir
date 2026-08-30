// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { App as AntApp, ConfigProvider, theme } from 'antd'
import enUS from 'antd/locale/en_US'
import { ProConfigProvider, enUSIntl } from '@ant-design/pro-components'
import './App.css'

import Home from './components/Home'

// The server renders index.html and fills these attributes on #root.
const flag = (root, name) => root.getAttribute(name) === 'true'

// antd 5 replaces the dark stylesheet with an algorithm, and takes the
// two Layout surfaces the old CSS overrode as theme tokens instead.
const dark = {
  algorithm: theme.darkAlgorithm,
  components: {
    Layout: {
      headerBg: '#1f1f1f',
      bodyBg: '#000',
      footerBg: '#000',
    },
  },
}

// One provider tree for the whole dashboard: ConfigProvider carries the
// theme and antd's locale, ProConfigProvider carries the pro-components
// locale, which is separate and defaults to Chinese, and AntApp puts
// message and modal under the same theme context as everything else.
const App = () => {
  const root = document.getElementById('root')
  return (
    <ConfigProvider locale={enUS} theme={dark}>
      <ProConfigProvider intl={enUSIntl} dark>
        <AntApp>
          <Home
            isAdmin={flag(root, 'is-admin')}
            statsMode={flag(root, 'stats-mode')}
            devMode={flag(root, 'dev-mode')}
            showImpressum={flag(root, 'show-impressum')}
            showPrivacy={flag(root, 'show-privacy')}
            showContact={flag(root, 'show-contact')}
          logoutURL={root.getAttribute('logout-url') || ''}
          />
        </AntApp>
      </ProConfigProvider>
    </ConfigProvider>
  )
}

export default App
