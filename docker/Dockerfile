# Copyright 2020 Changkun Ou. All rights reserved.
# Use of this source code is governed by a MIT
# license that can be found in the LICENSE file.

# The dashboard and the server are built here rather than on the deploy
# host, so a deployment needs nothing but Docker. Keeping the Go and Node
# toolchains in step with go.mod and dashboard/package.json on every host
# that ships this is otherwise a standing source of drift.

FROM node:22-alpine AS dashboard
WORKDIR /src/dashboard
COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm ci
COPY dashboard/ ./
RUN npm run build

FROM golang:1.27-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The server embeds the dashboard, so it has to be in place before the
# build. See the go:embed directives in server.go.
COPY --from=dashboard /src/dashboard/build ./dashboard/build
ARG VERSION=dev
RUN echo "$VERSION" > internal/version/.version && \
    CGO_ENABLED=0 go build -o /redir

FROM alpine:3.22
# The SSO login path makes outbound TLS calls, which need a trust store.
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=server /redir ./redir
# REDIR_CONF resolves inside this directory. If the file is missing the
# server falls back to the embedded default config without failing, so a
# deployment would come up silently misconfigured.
COPY data ./data
EXPOSE 80
CMD ["/app/redir", "-s"]
