import { Layout, Menu, Row, Col, Button } from 'antd'
import RedirTable from './RedirTable'
import RedirCreate from './RedirCreate'
import './Home.css'

const { Header, Content, Footer } = Layout;

const Home = (props) => {
  return (
    <Layout className="layout">
      <Header>
        <Row>
          <Col flex="100px">
            <a href="/s" style={{fontSize: '28px'}}>redir</a>
          </Col>
          <Col>
          {
            props.isAdmin ? 
            <Button danger><a href="/s">Logout</a></Button> :
            <Button><a href="/s?mode=admin">Go to Dashboard</a></Button>
          }
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
          {props.isAdmin ? <RedirCreate /> : <div></div>}
          </div>

          <RedirTable isAdmin={props.isAdmin} />
        </div>
      </Content>
      <Footer style={{ textAlign: 'center' }}>redir &copy; 2020-2021 Created by <a href='https://changkun.de'>Changkun Ou</a>. Open sourced under MIT license at <a href='https://changkun.de/s/redir'>GitHub</a>.</Footer>
    </Layout>
  )
}

export default Home;
