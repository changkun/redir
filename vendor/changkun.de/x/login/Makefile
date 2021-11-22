# Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
# Unauthorized using, copying, modifying and distributing, via any
# medium is strictly prohibited.

NAME = login
BUILD_FLAGS = -o $(NAME)

all:
	go build $(BUILD_FLAGS) cmd/login/*.go
run:
	./$(NAME)
initdb:
	cd db && go run initdb.go
build:
	CGO_ENABLED=0 GOOS=linux go build $(BUILD_FLAGS) cmd/login/*.go
	docker build -f docker/Dockerfile -t $(NAME):latest .
up:
	docker-compose -f docker/docker-compose.yml up -d
down:
	docker-compose -f docker/docker-compose.yml down
clean:
	rm -rf $(NAME)
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(NAME):latest 2> /dev/null; true