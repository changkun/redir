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
    CGO_ENABLED=0 go build -o /redir && \
    CGO_ENABLED=0 go build -o /redir-migrate ./cmd/migrate

FROM alpine:3.22
# Signing in calls the auth service over TLS, which needs a trust store.
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=server /redir ./redir
# The migration runs from this image so that it derives its columns with
# exactly the code the server uses. A copy built separately could drift.
COPY --from=server /redir-migrate ./redir-migrate
# The deployment config is mounted at /app/data/redirconf.yml rather than
# copied in: it holds credentials, and baking it in would put them in
# every layer of a shared image. See docker-compose.yml.
EXPOSE 80
CMD ["/app/redir", "-s"]
