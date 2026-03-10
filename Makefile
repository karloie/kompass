.PHONY: test build coverage

GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X github.com/karloie/kompass/pkg/graph.GitVersion=$(GIT_VERSION) -X github.com/karloie/kompass/pkg/graph.GitCommit=$(GIT_COMMIT)

build:
	go build -ldflags "$(LDFLAGS)" -o kompass cmd/kompass/*.go

test: build
	go test ./...

cover: build
	@go test ./... -coverprofile=coverage.out -covermode=atomic >/dev/null 2>&1 || true
	@echo "┌────────────────────────────────────────────────────────────────────┬──────────┐"
	@echo "│ Package                                                            │ Coverage │"
	@echo "├────────────────────────────────────────────────────────────────────┼──────────┤"
	@for pkg in $$(go list ./...); do \
		cov=$$(go test $$pkg -cover 2>&1 | grep -o 'coverage: [0-9.]*%' | cut -d' ' -f2); \
		if [ -n "$$cov" ]; then \
			printf "│ %-66s │ %7s  │\n" $$pkg $$cov; \
		fi; \
	done
	@echo "├────────────────────────────────────────────────────────────────────┼──────────┤"
	@go tool cover -func=coverage.out | grep 'total:' | awk '{printf "│ %-66s │ %7s  │\n", "TOTAL", $$3}'
	@echo "├────────────────────────────────────────────────────────────────────┴──────────┤"
	@echo "│ Function Coverage                                                              │"
	@echo "├────────────────────────────────────────────────────────────────────┬──────────┤"
	@go tool cover -func=coverage.out | grep -v 'total:' | awk '{printf "│ %-66s │ %7s  │\n", substr($$1":"$$2, 1, 66), $$3}'
	@echo "└────────────────────────────────────────────────────────────────────┴──────────┘"

help:
	@go run -ldflags "$(LDFLAGS)" ./cmd/kompass --help

mock:
	@go run -ldflags "$(LDFLAGS)" ./cmd/kompass --mock

real:
	@go run -ldflags "$(LDFLAGS)" ./cmd/kompass
