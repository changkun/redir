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
  build: {
    outDir: 'build',
    assetsDir: 'static',
  },
})
