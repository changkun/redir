// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { Button } from 'antd'

const Login = (props) => {
  if (props.isAdmin) {
    // With a real session there is somewhere to send the visitor to end it.
    // Under basic auth there is not, so reloading is all Logout can do.
    return (
      <Button danger href={props.logoutURL || window.location.pathname}>
        Logout
      </Button>
    )
  }
  return (
    <Button href={window.location.pathname + '?mode=admin'}>
      Go to Dashboard
    </Button>
  )
}

export default Login
