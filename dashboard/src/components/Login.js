// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

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