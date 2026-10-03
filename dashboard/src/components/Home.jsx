// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { useCallback, useEffect, useState } from 'react'
import Shell from './Shell'
import Overview from './Overview'
import LinkTable from './LinkTable'
import LinkForm from './LinkForm'
import { fetchOverview } from '../lib/api'
import { day } from '../lib/time'

// overviewDays is how far back the totals and the chart reach. Thirty
// days is long enough for a weekly rhythm to be visible without
// flattening what happened this week.
const overviewDays = 30

const Home = (props) => {
  const [overview, setOverview] = useState(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [creating, setCreating] = useState(false)
  // What the last action did, said once beside the list it changed. An
  // error stays until the next action; the rest is not worth a dialog.
  const [status, setStatus] = useState({ text: '', error: false })
  const notify = useCallback(
    (text, error = false) => setStatus({ text, error }),
    [],
  )

  const loadOverview = useCallback(async () => {
    if (!props.isAdmin) return
    const end = new Date()
    const start = new Date(end.getTime() - overviewDays * 864e5)
    try {
      setOverview(await fetchOverview(props.devMode, day(start), day(end)))
    } catch (e) {
      notify(`Could not load the totals: ${e.message ?? e}`, true)
    }
  }, [props.isAdmin, props.devMode, notify])

  useEffect(() => {
    loadOverview()
  }, [loadOverview, reloadKey])

  const refresh = () => setReloadKey((k) => k + 1)

  const footer = (
    <>
      {props.showImpressum && <a href="./.impressum">Impressum</a>}
      {props.showPrivacy && <a href="./.privacy">Privacy</a>}
      {props.showContact && <a href="./.contact">Contact</a>}
      <span>redir on {props.site}</span>
    </>
  )

  return (
    <Shell
      site={props.site}
      isAdmin={props.isAdmin}
      logoutURL={props.logoutURL}
      footer={footer}
    >
      {props.isAdmin && (
        <Overview data={overview} days={overviewDays} site={props.site} />
      )}

      <LinkTable
        isAdmin={props.isAdmin}
        statsMode={props.statsMode}
        devMode={props.devMode}
        reloadKey={reloadKey}
        status={status}
        notify={notify}
        onNew={() => setCreating(true)}
        onChanged={refresh}
      />

      {creating && (
        <LinkForm
          record={null}
          onClose={() => setCreating(false)}
          onSaved={(alias) => {
            setCreating(false)
            notify(`${alias} created.`)
            refresh()
          }}
        />
      )}
    </Shell>
  )
}

export default Home
