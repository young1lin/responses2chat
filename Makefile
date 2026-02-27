.PHONY: build run test clean install docker-build

APP_NAME := responses2chat
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)"

build:
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/$(APP_NAME)

run:
	go run ./cmd/$(APP_NAME) -c configs/config.yaml

test:
	go test -v ./...

clean:
	rm -rf bin/

install: build
	cp bin/$(APP_NAME) /usr/local/bin/

docker-build:
	docker build -t $(APP_NAME):$(VERSION) .

tidy:
	go mod tidy

fmt:
	go fmt ./...

lint:
	golangci-lint run

.DEFAULT_GOAL := build
