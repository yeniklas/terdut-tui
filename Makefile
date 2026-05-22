VERSION := $(shell git describe --tags --always --dirty)

.PHONY: build install test

build:
	go build -ldflags "-X main.version=$(VERSION)" -o terdut-tui .

install:
	go install -ldflags "-X main.version=$(VERSION)" .

test:
	go test ./...
