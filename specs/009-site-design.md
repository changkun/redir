---
title: The site's design
status: complete
depends_on:
  - 007-shared-design
affects:
  - dashboard
  - templates
effort: small
created: 2026-11-08
updated: 2026-11-08
author: changkun
dispatched_task_id: null
---

# The site's design

## Overview

`007` put redir in the design the urlstat dashboard used, so the two tools
read as one set. changkun.de has since settled on one design for all of
its pages, the blog's: the ink palette of white and neutral grays, Inter,
navy for little more than a link under the pointer, and a header with the
name on the left and plain links and a theme button on the right. redir is
served from changkun.de at `/s/`, and a visitor who follows a short link
from the blog lands on its warning or countdown page. This puts the
console and the server's pages in that design. What they show and what
they do are unchanged.

## Decisions

### The ink palette, and the chart colors kept

The page, surfaces, ink and grays are the blog's, light and dark. Cards
keep their hairline border on the page's own ground rather than a tinted
surface. The two chart colors stay: they carry meaning, visits blue and
visitors orange, and were checked for contrast and colorblind separation
in `007`. The antd dialogs and date pickers are themed from the same
values, with navy as their accent.

### Inter, from the binary

Inter is served from the console's static route, at
`<prefix>.static/fonts/`, rather than from a font host, so loading a page
of a link shortener sends a visitor's address nowhere a link did not.
The three weights the pages use, 400, 500 and 600, are about 68 KB.

### The blog's header and its theme button

The console and the server's pages open with the blog's header: `redir /
<site>` on the left; the console's links, then a 36px theme button, on
the right, 16px from the top and right edges as on the home page. The
choice of theme is kept in `localStorage` under `theme`, the key the home
page, the blog and the books use, so it holds across the site on one
origin. Without a choice the system's setting decides, and keeps deciding
while a page is open. The theme is set on the root before the page
paints, and the media query still covers a page without script.

## Verification

1. The console, the public index, a dialog, and the warning, countdown
   and legal pages render in light and dark, at 1280 and 390 pixels wide,
   with no horizontal overflow.
2. The theme button turns the page and keeps the choice; the antd dialog
   follows it.
3. The dashboard and Go suites pass.

## Outcome

Completed 2026-11-08, against a local PostgreSQL with made-up links and
visits and `auth.enable: none`.

| Check | Result |
| --- | --- |
| Views | console, public index, new-link dialog, warning, countdown, impressum, privacy, contact; light and dark; 1280 and 390 |
| Theme | the button sets `data-theme` and `theme`; a dialog opened after it is dark; another page opens in the chosen theme |
| Tests | dashboard 36 → 37 (the theme button); the Go test of the served layout checks the new ground, the theme key and the self-hosted font |

### Not verified

Signing in through auth.latere.ai on a deployed site, as in `007`. The
README's screenshots in `.github/assets` show the previous design.
