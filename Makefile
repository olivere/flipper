.PHONY: install setup build test image

BIN := bin/flipper
MODULE := github.com/olivere/flipper

install:
	go install ./cmd/flipper

setup:
	go mod tidy

build:
	go build -o $(BIN) ./cmd/flipper

test:
	go test ./...

image:
	docker build -t flipper .
