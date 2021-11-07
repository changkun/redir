// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useRef, Fragment } from 'react'
import { Layout, Row, Col } from 'antd'
import RedirTable from './RedirTable'
import RedirCreate from './RedirCreate'
import './Home.css'
import Login from './Login'

const { Header, Content, Footer } = Layout;

const Home = (props) => {
  const tableRefresh = useRef();

  return (
    <Layout className="layout">
      <Header>
        <Row>
          <Col flex="100px">
            <a href="/s" style={{fontSize: '28px'}}>redir</a>
          </Col>
          <Col>
            <Login isAdmin={props.isAdmin} />
          </Col>
        </Row>
      </Header>
      <Content style={{ padding: '0 50px' }}>
        <div className="layout-content">
          <div className="" style={{
            display: 'flex',
            width: 'max-content',
            justifyContent: 'flex-end',
          }}>
          {props.isAdmin ? <RedirCreate refreshRef={tableRefresh}/> : <div></div>}
          </div>

          <RedirTable isAdmin={props.isAdmin} statsMode={props.statsMode} devMode={props.devMode} refreshRef={tableRefresh}/>
        </div>
      </Content>
      <Footer style={{ textAlign: 'center' }}>
        <div>
        {props.showImpressum ? <span><a href="/s/.impressum">Impressum</a>&nbsp;&nbsp;</span> : ''}
        {props.showPrivacy ? <span><a href="/s/.privacy">Privacy</a>&nbsp;&nbsp;</span> : ''}
        {props.showContact ? <span><a href="/s/.contact">Contact</a></span> : ''}
        </div>
        <div> &copy; 2021 Created by Changkun Ou.</div>
      </Footer>
    </Layout>
  )
}

export default Home;
