import { Layout, Menu } from 'antd';
import './Home.css';
import RedirTable from './RedirTable'

const { Header, Content, Footer } = Layout;

export default () => (
  <Layout className="layout">
    <Header>
      <div className="logo">redir</div>
      <Menu theme="dark" mode="horizontal" defaultSelectedKeys={['1']}>
        <Menu.Item key="1">Dashboard</Menu.Item>
      </Menu>
    </Header>
    <Content style={{ padding: '0 50px' }}>
      <div className="site-layout-content">
        <RedirTable />
      </div>
    </Content>
    <Footer style={{ textAlign: 'center' }}>redir ©2020 Created by <a href='https://changkun.de'>Changkun Ou</a>. Open sourced under MIT license at <a href='https://changkun.de/s/redir'>GitHub</a>.</Footer>
  </Layout>
)