# Copyright 2021 Changkun Ou. All rights reserved.
# Use of this source code is governed by a MIT
# license that can be found in the LICENSE file.

VERSION = $(shell git describe --always --tags)
IMAGE = redir
BINARY = redir
TARGET = -o $(BINARY)
BUILD_FLAGS = $(TARGET) -mod=vendor
GOOS = linux darwin
GOARCH = amd64 arm64

all:
	go build $(BUILD_FLAGS)

$(GOOS): $(GOARCH)
	echo $(VERSION) > internal/version/.version
	for goarch in $^ ; do \
		mkdir -p build/$(BINARY); \
		cp internal/config/config.yml build/$(BINARY)/config.yml; \
		CGO_ENABLED=0 GOARCH=$${goarch} GOOS=$@ go build -o build/$(BINARY)/$(BINARY) -mod=vendor; \
		zip -r build/redir-$(VERSION)-$@-$${goarch}.zip build/$(BINARY); \
		rm -rf build/$(BINARY); \
	done
# restore
	echo dev > internal/version/.version

run:
	./$(BINARY) -s
dashboard:
	cd dashboard && npm i && npm run build
build:
	CGO_ENABLED=0 GOOS=linux go build $(BUILD_FLAGS)
	docker build -f docker/Dockerfile -t $(IMAGE):latest .
up:
	docker-compose -f docker/docker-compose.yml up -d
down:
	docker-compose -f docker/docker-compose.yml down


release: $(GOOS)

clean:
	rm -rf $(BINARY) build
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(IMAGE):latest 2> /dev/null; true

.PHONY: $(GOOS) $(GOARCH) run dashboard build up down clean