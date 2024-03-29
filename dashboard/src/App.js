// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import React from 'react';
import './App.css';

import Home from './components/Home'

const App = () => {
    const root = document.getElementById('root')
    const isAdmin = root.getAttribute('is-admin')
    const statsMode = root.getAttribute('stats-mode')
    const devMode = root.getAttribute('dev-mode')
    const showImpressum = root.getAttribute('show-impressum')
    const showPrivacy = root.getAttribute('show-privacy')
    const showContact = root.getAttribute('show-contact')
    return <Home
        isAdmin={isAdmin === 'true' ? true : false}
        statsMode={statsMode === 'true' ? true : false}
        devMode={devMode === 'true' ? true : false}
        showImpressum={showImpressum === 'true' ? true : false}
        showPrivacy={showPrivacy === 'true' ? true : false}
        showContact={showContact === 'true' ? true : false}
    />
}

export default App