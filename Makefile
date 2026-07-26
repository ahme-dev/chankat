.PHONY: build run test fmt clean

build:
	go build -o tt ./cmd/tt

run:
	go run ./cmd/tt

test:
	go test ./...

fmt:
	gofmt -w cmd

clean:
	go clean
	rm -f tt
