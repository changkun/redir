// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { Button } from 'antd'

const Login = (props) => {
  if (props.isAdmin) {
    return <Button danger href={window.location.pathname}>Logout</Button>
  }
  return (
    <Button href={window.location.pathname + '?mode=admin'}>
      Go to Dashboard
    </Button>
  )
}

export default Login
