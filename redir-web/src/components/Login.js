import { Button } from 'antd'

const Login = (props) => {
  if (props.isAdmin) {
    return (
      <Button danger><a href="/s">Logout</a></Button>
    )
  }
  return <Button><a href="/s?mode=admin">Go to Dashboard</a></Button>
}

export default Login