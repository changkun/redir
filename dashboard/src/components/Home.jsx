// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useCallback, useEffect, useState } from 'react'
import { App, Button } from 'antd'
import Shell from './Shell'
import Overview from './Overview'
import LinkTable from './LinkTable'
import LinkForm from './LinkForm'
import { fetchOverview } from '../lib/api'
import { day } from '../lib/time'
import { tokens } from '../theme'

// overviewDays is how far back the strip at the top reaches. Thirty days
// is long enough for a weekly rhythm to be visible without flattening
// what happened this week.
const overviewDays = 30

const Home = (props) => {
  const { message } = App.useApp()
  const [overview, setOverview] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [creating, setCreating] = useState(false)

  const loadOverview = useCallback(async () => {
    if (!props.isAdmin) return
    const end = new Date()
    const start = new Date(end.getTime() - overviewDays * 864e5)
    try {
      setOverview(await fetchOverview(props.devMode, day(start), day(end)))
    } catch (e) {
      message.error(String(e))
    }
  }, [props.isAdmin, props.devMode, message])

  useEffect(() => {
    loadOverview()
  }, [loadOverview, reloadKey])

  const refresh = () => setReloadKey((k) => k + 1)

  const footer = (
    <>
      {props.showImpressum && <a href="./.impressum">Impressum</a>}
      {props.showPrivacy && <a href="./.privacy">Privacy</a>}
      {props.showContact && <a href="./.contact">Contact</a>}
      <span style={{ color: tokens.textFaint }}>
        redir · {props.site}
      </span>
    </>
  )

  return (
    <Shell
      site={props.site}
      isAdmin={props.isAdmin}
      logoutURL={props.logoutURL}
      footer={footer}
      actions={
        props.isAdmin && (
          <Button size="small" type="primary" onClick={() => setCreating(true)}>
            New link
          </Button>
        )
      }
    >
      {props.isAdmin ? (
        <Overview data={overview} days={overviewDays} />
      ) : (
        <div
          style={{
            padding: `${tokens.space(8)}px 0 ${tokens.space(4)}px`,
            borderBottom: `1px solid ${tokens.line}`,
          }}
        >
          <div style={{ fontSize: 20, letterSpacing: '-0.01em' }}>
            Short links
          </div>
          <div style={{ color: tokens.textDim, marginTop: 4 }}>
            Public redirects served by {props.site}.
          </div>
        </div>
      )}

      <LinkTable
        isAdmin={props.isAdmin}
        statsMode={props.statsMode}
        devMode={props.devMode}
        reloadKey={reloadKey}
      />

      {creating && (
        <LinkForm
          record={null}
          onClose={() => setCreating(false)}
          onSaved={() => {
            setCreating(false)
            refresh()
          }}
        />
      )}
    </Shell>
  )
}

export default Home
