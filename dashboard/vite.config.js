// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The Go server embeds this output: server.go embeds build/index.html and
// build/static, then rewrites the "/static" prefix in the HTML to
// "./.static" and serves the files under that route. Keep outDir and
// assetsDir in step with those paths.
export default defineConfig({
  plugins: [react()],
  test: {
    // The console is a rewrite of every component, so the suite renders
    // it. A missing import or a crash on first paint blanks the page,
    // and nothing else in the build catches that.
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.js'],
  },
  build: {
    outDir: 'build',
    assetsDir: 'static',
  },
})
