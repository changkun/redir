import { Layout, Menu } from 'antd'
import RedirTable from './RedirTable'
import RedirCreate from './RedirCreate'
import './Home.css'

const { Header, Content, Footer } = Layout;

const Home = (props) => {
  return (
    <Layout className="layout">
      <Header>
        <div className="logo">
          <a href="/s">redir</a>
        </div>
        <Menu theme="dark" mode="horizontal" defaultSelectedKeys={['1']}>
          <Menu.Item key="1">Dashboard</Menu.Item>
        </Menu>
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
