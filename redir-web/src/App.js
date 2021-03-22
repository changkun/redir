import React from 'react';
import './App.css';

import Home from './components/Home'

const App = () => {
    const root = document.getElementById('root')
    const isAdmin = root.getAttribute('is-admin')
    return <Home isAdmin={isAdmin === 'true' ? true : false}/>
}

export default App