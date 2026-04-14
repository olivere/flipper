.PHONY: help install setup build test image

BIN := bin/flipper
MODULE := github.com/olivere/flipper

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

install: ## Install flipper to $GOPATH/bin
	go install ./cmd/flipper

setup: ## Tidy module dependencies
	go mod tidy

build: ## Build to bin/flipper
	go build -o $(BIN) ./cmd/flipper
	codesign -s "Developer ID Application: Oliver Eilhard (BSWQ3M7W67)" -f $(BIN)

test: ## Run all tests
	go test ./...

image: ## Build Docker image
	docker build -t flipper .
