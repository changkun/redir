// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import Spark from './Spark'
import Stats from './Stats'
import LinkForm from './LinkForm'
import Confirm from './Confirm'
import { aliasPath, aliasURL } from '../lib/paths'
import { fetchIndex, remove } from '../lib/api'
import { nextSort, sortRows, targetLabel } from '../lib/format'
import { useNarrow } from '../theme'

const fmt = (n) => (n ?? 0).toLocaleString()

// Copy puts a link's full address on the clipboard, and says so in its
// own place rather than in a message somewhere else.
const Copy = ({ alias }) => {
  const [done, setDone] = useState(false)
  return (
    <button
      type="button"
      className={`link copy${done ? ' done' : ''}`}
      title={`Copy ${aliasURL(alias)}`}
      onClick={async (e) => {
        e.stopPropagation()
        try {
          await navigator.clipboard.writeText(aliasURL(alias))
          setDone(true)
          setTimeout(() => setDone(false), 1500)
        } catch {
          // No clipboard without a secure context; the link is still
          // there to be selected.
        }
      }}
    >
      {done ? 'Copied' : 'Copy'}
    </button>
  )
}

// held says whether a link is still waiting for its start time.
const held = (r) =>
  r.valid_from && new Date(r.valid_from).getTime() > Date.now()

// LinkTable is the console's list of links.
//
// Editing happens in a form rather than in the row: a row is for reading.
// The columns are the ones an operator scans, and the rest are one click
// away, in the row's detail or in the form.
const LinkTable = ({
  isAdmin,
  statsMode,
  devMode,
  reloadKey,
  status,
  notify,
  onNew,
  onChanged,
}) => {
  const [rows, setRows] = useState([])
  const [series, setSeries] = useState({})
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [loaded, setLoaded] = useState(false)
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState(null)
  const [open, setOpen] = useState(() => new Set())
  const [editing, setEditing] = useState(null)
  const [deleting, setDeleting] = useState(null)

  // A phone keeps the link, its visits and what can be done to it. The
  // rest is in the row's detail and in the form. The columns are left
  // out rather than hidden, so the detail below a row spans exactly the
  // columns there are.
  const wide = !useNarrow()

  // Every link at once, so that the list can be read and filtered as a
  // whole. A pager appears only past this many, which no index here is
  // near.
  const pageSize = 500
  const pages = Math.max(1, Math.ceil(total / pageSize))
  const paged = pages > 1

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchIndex({ isAdmin, devMode, page, pageSize })
      setRows(data.data ?? [])
      setSeries(data.series ?? {})
      setTotal(data.total ?? 0)
    } catch (e) {
      notify?.(`Could not load the links: ${e.message ?? e}`, true)
    } finally {
      setLoading(false)
      setLoaded(true)
    }
  }, [isAdmin, devMode, page, notify])

  useEffect(() => {
    load()
  }, [load, reloadKey])

  // The filter is client side and deliberately so: it narrows the list in
  // front of you as you type, with no round trip. The public index is in
  // alphabetical order, which is how a list of names is looked through.
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    const shown = q
      ? rows.filter(
          (r) =>
            r.alias.toLowerCase().includes(q) ||
            (r.url ?? '').toLowerCase().includes(q),
        )
      : rows
    if (!isAdmin) {
      return [...shown].sort((a, b) =>
        a.alias.localeCompare(b.alias, undefined, { sensitivity: 'base' }),
      )
    }
    return sortRows(shown, sort)
  }, [rows, query, sort, isAdmin])

  // A row opens to its detail where there are statistics to show.
  const opens = isAdmin && statsMode
  const toggle = (alias) =>
    setOpen((was) => {
      const now = new Set(was)
      now.has(alias) ? now.delete(alias) : now.add(alias)
      return now
    })

  const columns = (opens ? 1 : 0) + (wide ? 7 : 3)
  const sorter = (key, name) => (
    <th
      className={`num ${key}`}
      aria-sort={
        sort?.key === key
          ? sort.dir === 'desc'
            ? 'descending'
            : 'ascending'
          : undefined
      }
    >
      <button
        type="button"
        title={`Sort this page by ${name.toLowerCase()}`}
        onClick={() => setSort(nextSort(sort, key))}
      >
        {name}
        {sort?.key === key ? (sort.dir === 'desc' ? ' ↓' : ' ↑') : ''}
      </button>
    </th>
  )

  return (
    <section className="card links" aria-label="Links">
      <div className="head">
        <h2>Links</h2>
        <span className="count">
          {query
            ? `${visible.length} of ${rows.length}${paged ? ' on this page' : ''}`
            : fmt(total)}
        </span>
        <div className="right">
          <span
            className={`status${status?.error ? ' error' : ''}`}
            role="status"
          >
            {status?.text}
          </span>
          {isAdmin && (
            <button type="button" className="btn primary" onClick={onNew}>
              New link
            </button>
          )}
          <input
            className="search"
            type="search"
            placeholder={paged ? 'Filter this page' : 'Filter links'}
            aria-label={paged ? 'Filter this page' : 'Filter links'}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
      </div>

      <div className={loading && loaded ? 'busy' : undefined}>
        {visible.length === 0 ? (
          <div className="empty">
            {!loaded
              ? 'Loading…'
              : query
                ? paged
                  ? 'No link on this page matches.'
                  : 'No link matches.'
                : 'No links yet.'}
          </div>
        ) : !isAdmin ? (
          // A visitor needs the link and nothing else: no target, by
          // design, since listing it would let the index be used to
          // enumerate where every link goes. With one thing to show per
          // link, a table would be a single column of mostly empty rows,
          // so the links are set side by side.
          <ul className="grid">
            {visible.map((r) => (
              <li key={r.alias}>
                <a href={aliasPath(r.alias)} title={aliasURL(r.alias)}>
                  {aliasPath(r.alias)}
                </a>
                <Copy alias={r.alias} />
              </li>
            ))}
          </ul>
        ) : (
          <table>
            <thead>
              <tr>
                {opens && <th className="open" />}
                <th>Link</th>
                {isAdmin && (
                  <>
                    {wide && <th className="target">Target</th>}
                    {sorter('pv', 'Visits')}
                    {wide && sorter('uv', 'Visitors')}
                    {wide && <th className="spark">14 days</th>}
                    {wide && <th className="flags" />}
                    <th className="acts" />
                  </>
                )}
              </tr>
            </thead>
            <tbody>
              {visible.map((r) => {
                const shown = opens && open.has(r.alias)
                return (
                  <Fragment key={r.alias}>
                    <tr
                      className={`row${opens ? ' opens' : ''}`}
                      aria-expanded={opens ? shown : undefined}
                      onClick={opens ? () => toggle(r.alias) : undefined}
                    >
                      {opens && (
                        <td className="open">
                          <button
                            type="button"
                            className="twist"
                            aria-label={`${shown ? 'Hide' : 'Show'} the detail of ${r.alias}`}
                            onClick={(e) => {
                              e.stopPropagation()
                              toggle(r.alias)
                            }}
                          >
                            {shown ? '▼' : '▶'}
                          </button>
                        </td>
                      )}
                      {/* The alias is a link on the public page, because a
                          visitor is there to follow one. In the console it
                          is not: a row opens to its detail, and a click
                          that sometimes navigates away and sometimes opens
                          a panel is a click you cannot trust. */}
                      <td className="alias">
                        <div className="name">
                          {isAdmin ? (
                            <span title={aliasURL(r.alias)}>
                              {aliasPath(r.alias)}
                            </span>
                          ) : (
                            <a href={aliasPath(r.alias)}>
                              {aliasPath(r.alias)}
                            </a>
                          )}
                          <Copy alias={r.alias} />
                        </div>
                      </td>
                      {/* The public listing carries no target, by design:
                          it would let the index be used to enumerate where
                          every link goes. */}
                      {isAdmin && (
                        <>
                          {wide && (
                            <td className="target" title={r.url}>
                              {r.url ? targetLabel(r.url) : '—'}
                            </td>
                          )}
                          <td className="num">{fmt(r.pv)}</td>
                          {wide && <td className="num uv">{fmt(r.uv)}</td>}
                          {wide && (
                            <td className="spark">
                              <Spark data={series[r.alias] ?? []} />
                            </td>
                          )}
                          {wide && (
                            <td className="flags">
                              {r.private && (
                                <span
                                  className="tag"
                                  title="Not listed on the public index"
                                >
                                  private
                                </span>
                              )}
                              {!r.trust && (
                                <span
                                  className="tag"
                                  title="Shows a warning before leaving the site"
                                >
                                  warns
                                </span>
                              )}
                              {held(r) && (
                                <span
                                  className="tag"
                                  title={`Shows a countdown until ${new Date(r.valid_from).toLocaleString()}`}
                                >
                                  held
                                </span>
                              )}
                            </td>
                          )}
                          <td className="acts">
                            <button
                              type="button"
                              className="link"
                              onClick={(e) => {
                                e.stopPropagation()
                                setEditing(r)
                              }}
                            >
                              Edit
                            </button>
                            <button
                              type="button"
                              className="link danger"
                              onClick={(e) => {
                                e.stopPropagation()
                                setDeleting(r)
                              }}
                            >
                              Delete
                            </button>
                          </td>
                        </>
                      )}
                    </tr>
                    {shown && (
                      <tr className="detail">
                        <td colSpan={columns}>
                          <Stats alias={r.alias} devMode={devMode} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}
            </tbody>
          </table>
        )}
      </div>

      {paged && (
        <div className="more">
          <button
            type="button"
            className="btn"
            disabled={page <= 1}
            onClick={() => setPage(page - 1)}
          >
            Previous
          </button>
          <button
            type="button"
            className="btn"
            disabled={page >= pages}
            onClick={() => setPage(page + 1)}
          >
            Next
          </button>
          <span>
            Page {page} of {pages}
          </span>
        </div>
      )}

      {editing && (
        <LinkForm
          record={editing}
          onClose={() => setEditing(null)}
          onSaved={(alias) => {
            setEditing(null)
            notify?.(`${alias} saved.`)
            load()
            onChanged?.()
          }}
        />
      )}

      {deleting && (
        <Confirm
          title={`Delete ${aliasPath(deleting.alias)}?`}
          text="The link stops working at once. Its recorded visits are kept."
          what={aliasPath(deleting.alias)}
          detail={deleting.url ? targetLabel(deleting.url) : ''}
          action="Delete"
          onClose={() => setDeleting(null)}
          onConfirm={async () => {
            const err = await remove(deleting.alias)
            if (err) return err
            notify?.(`${deleting.alias} deleted.`)
            setDeleting(null)
            load()
            onChanged?.()
            return null
          }}
        />
      )}
    </section>
  )
}

export default LinkTable
