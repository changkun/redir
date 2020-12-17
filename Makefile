# Copyright 2020 Changkun Ou. All rights reserved.
# Use of this source code is governed by a MIT
# license that can be found in the LICENSE file.

VERSION = $(shell git describe --always --tags)
IMAGE = redir
BINARY = redir
TARGET = -o $(BINARY)
BUILD_FLAGS = $(TARGET) -mod=vendor

all:
	go build $(BUILD_FLAGS)
run:
	./$(BINARY) -s
build:
	GOOS=linux go build $(BUILD_FLAGS)
	docker build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest -f docker/Dockerfile .
up: down
	docker-compose -f docker/deploy.yml up -d
down:
	docker-compose -f docker/deploy.yml down
clean:
	rm -rf $(BINARY)
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(IMAGE):latest $(IMAGE):$(VERSION) 2> /dev/null; true