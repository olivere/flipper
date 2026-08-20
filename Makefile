.PHONY: help install setup build test lint image

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

# Package directories rather than ".", so a git worktree checked out
# under .claude/ is not formatted as part of this module.
PKG_DIRS = $$(go list -f '{{.Dir}}' ./...)

lint: ## Run vet, staticcheck, govulncheck, and format checks
	go vet ./...
	@command -v staticcheck >/dev/null || { echo "staticcheck not found: go install honnef.co/go/tools/cmd/staticcheck@latest"; exit 1; }
	staticcheck ./...
	@command -v govulncheck >/dev/null || { echo "govulncheck not found: go install golang.org/x/vuln/cmd/govulncheck@latest"; exit 1; }
	govulncheck ./...
	@command -v gofumpt >/dev/null || { echo "gofumpt not found: go install mvdan.cc/gofumpt@latest"; exit 1; }
	@dirs="$(PKG_DIRS)"; \
	unformatted=$$( { gofmt -l $$dirs; gofumpt -l $$dirs; } | sort -u ); \
	if [ -n "$$unformatted" ]; then \
		echo "unformatted files (fix with: gofumpt -w . && gofmt -w .):"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

image: ## Build Docker image
	docker build -t flipper .
