.PHONY: build run test fmt clean

build:
	mkdir -p build
	go build -o build/chansat ./cmd

run:
	go run ./cmd

test:
	go test ./...

fmt:
	gofmt -w cmd

clean:
	go clean
	rm -rf build
