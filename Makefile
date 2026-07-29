.PHONY: build run test fmt clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	mkdir -p build
	go build -ldflags "-X main.version=$(VERSION)" -o build/chankat ./cmd

run:
	go run ./cmd

test:
	go test ./...

fmt:
	gofmt -w cmd internal

clean:
	go clean
	rm -rf build
