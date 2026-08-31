# Copyright 2021 Changkun Ou. All rights reserved.
# Use of this source code is governed by a MIT
# license that can be found in the LICENSE file.

VERSION = $(shell git describe --always --tags)
IMAGE = redir
BINARY = redir
TARGET = -o $(BINARY)
BUILD_FLAGS = $(TARGET)
GOOS = linux darwin
GOARCH = amd64 arm64

all:
	go build $(BUILD_FLAGS)

$(GOOS): $(GOARCH)
	echo $(VERSION) > internal/version/.version
	for goarch in $^ ; do \
		mkdir -p build/$(BINARY); \
		cp internal/config/config.yml build/$(BINARY)/config.yml; \
		CGO_ENABLED=0 GOARCH=$${goarch} GOOS=$@ go build -o build/$(BINARY)/$(BINARY); \
		zip -r build/redir-$(VERSION)-$@-$${goarch}.zip build/$(BINARY); \
		rm -rf build/$(BINARY); \
	done
# restore
	echo dev > internal/version/.version

run:
	./$(BINARY) -s

# migrate copies the MongoDB data into PostgreSQL from inside the running
# image, so the derived columns are written by the same code the server
# uses. MONGO, POSTGRES and HOST come from the environment; add TRUNCATE=1
# to replace what is already there.
migrate:
	docker compose -f docker/docker-compose.yml exec redir \
		/app/redir-migrate -from "$(MONGO)" -to "$(POSTGRES)" \
		-host "$(HOST)" $(if $(TRUNCATE),-truncate,) $(if $(DRYRUN),-dry-run,)
dashboard:
	cd dashboard && npm ci && npm run build
build:
	docker build -f docker/Dockerfile --build-arg VERSION=$(VERSION) -t $(IMAGE):latest .
# Compose v2. The v1 python client cannot read the image metadata that
# BuildKit writes and fails with KeyError: 'ContainerConfig' after it has
# already removed the running container.
up:
	docker compose -f docker/docker-compose.yml up -d
down:
	docker compose -f docker/docker-compose.yml down


release: $(GOOS)

clean:
	rm -rf $(BINARY) build
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(IMAGE):latest 2> /dev/null; true

.PHONY: $(GOOS) $(GOARCH) run migrate dashboard build up down clean