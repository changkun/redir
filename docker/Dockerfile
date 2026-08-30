# Copyright 2020 Changkun Ou. All rights reserved.
# Use of this source code is governed by a MIT
# license that can be found in the LICENSE file.
FROM alpine:3.22
# The server talks to the SSO endpoint over TLS, which needs a trust store.
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY . .
EXPOSE 80
CMD ["/app/redir", "-s"]