.PHONY: build run seed test fmt clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DEV_DATA_PATH ?= $(CURDIR)/.dev-data/data.sqlite

build:
	mkdir -p build
	go build -ldflags "-X main.version=$(VERSION)" -o build/chankat ./cmd

run:
	CHANKAT_DATA_PATH="$(DEV_DATA_PATH)" go run ./cmd

seed:
	CHANKAT_DATA_PATH="$(DEV_DATA_PATH)" go run ./cmd/seed --reset

test:
	go test ./...

fmt:
	gofmt -w cmd internal

clean:
	go clean
	rm -rf build
