---
title: Shared design
status: complete
depends_on:
  - 006-operator-console
affects:
  - dashboard
  - templates
effort: medium
created: 2026-10-03
updated: 2026-10-03
author: changkun
dispatched_task_id: null
---

# Shared design

## Overview

`006` gave the console a look of its own: dark only, monospace for
identifiers, hairlines instead of cards. Then the urlstat dashboard was
rebuilt, and the two tools, which run on the same host and are opened by
the same person minutes apart, looked like two products.

This puts redir in the design urlstat uses, so the two read as one set of
tools: the same colors, type, cards, tables and controls, and the same
plain words. What the console shows and what it can do are unchanged.

## Decisions

### One set of tokens, light and dark

The colors are urlstat's, named by the job they do: page, surface, ink,
two chart colors, one red for what cannot be undone. Both follow the
system's light or dark setting. `006` ruled light mode out of scope; this
reverses that, because a console that is dark while everything beside it
follows the system is the inconsistency this spec removes.

The two chart colors were checked against both surfaces for lightness,
chroma, colorblind separation and contrast. Visits are blue and visitors
orange in every chart, in both tools.

### Cards, tiles and a system sans

`006` chose hairlines over cards and a monospace for aliases and URLs.
urlstat groups with cards, leads with a row of tiles and sets everything
in the system sans, with tabular figures in columns. redir follows it.
The overview's four figures are tiles, the month is a chart in a card,
and the links are a table in a card with its actions in the card's
heading.

### The chart is the same chart

One axis, two-pixel lines, hairline gridlines on round whole numbers, a
hairline and a readout that follow the pointer or the arrow keys, and the
same numbers as a table one click away. The overview's sparkline became
this chart, since thirty days of two series is more than a sparkline can
say. The per-row sparkline stays: one per row, no axes, is its job.

### antd is kept for what it is good at, and only that

`006` kept antd for validation, date entry and their keyboard and screen
reader behaviour. That still holds, so the form dialog, its fields and
the date pickers remain antd's, themed from the same tokens. Everything
that decides how the page looks is now the console's own: the table, the
buttons, the choices between two options, the tags. The delete
confirmation is the same dialog urlstat asks its question in.

Messages no longer float over the page. What an action did is said once,
beside the list it changed, and a refusal is shown inside the dialog that
caused it, so it can be fixed without the dialog closing.

### The public index is a grid

A visitor gets the link and nothing else, by design. A table with one
column is mostly empty rows, so the links are set side by side.

### The server's pages follow

The warning, the countdown and the legal pages come from one layout in
`templates/base.html`. It carries the same tokens and controls, so a
visitor who meets a warning does not arrive at a different site.

## Verification

1. The console, the public index, the dialogs and every server-rendered
   page render in light and in dark, at 1280 and at 390 pixels wide, with
   no console error and no horizontal overflow.
2. Creating, editing and deleting a link work, including validation, a
   refused duplicate, and cancelling a delete.
3. Filtering, sorting, paging, a row's detail, the chart's readout by
   pointer and by keyboard, and its table all work.
4. The dashboard and Go suites pass.

## Outcome

Completed 2026-10-03.

| Check | Result |
| --- | --- |
| Bundle | 1,051 KB → **678 KB**, gzip 336 KB → 220 KB |
| Views | console, public index, link detail, both dialogs, warning, countdown, three legal pages, each in light and dark and on a phone |
| Link changes | create, validation, refused duplicate, edit, cancelled and confirmed delete, against a local PostgreSQL with made-up links and visits |
| Tests | dashboard 25 → 36; Go suite passes with the race detector |

### Decisions worth keeping

**A refusal is read from the body.** The server names a refusal in its
answer but sends it with a 200, so the console had been reporting a
duplicate alias as created. It now reads the message whatever the status
says. The status itself is the server's to fix.

**A day that has not begun is not drawn.** A link's detail asks for a
range ending tomorrow, so that today is inside it, and the chart used to
draw that last day, which could only read zero. The line fell to the axis
at its right end every day.

### Not verified

Signing in through auth.latere.ai and administering a deployed site. The
local run used `auth.enable: none`; the login is untouched by this spec.

## Out of Scope

- What any figure means, and anything the server sends.
- Removing antd altogether. With the table gone it is the dialogs and the
  date pickers, and replacing those is a separate decision from how the
  console looks.
