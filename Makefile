.PHONY: test build coverage

GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION_LDFLAGS := -X github.com/karloie/kompass/pkg/graph.GitVersion=$(GIT_VERSION) -X github.com/karloie/kompass/pkg/graph.GitCommit=$(GIT_COMMIT)
LDFLAGS ?=
ARGS ?=
COVERPKG ?= ./...

SNAPSHOT_DIR ?= testdata/fixtures
SNAPSHOT_MOCK_NAMESPACE ?= petshop
SNAPSHOT_TOOL_CONTEXT ?= tool-test-01
SNAPSHOT_TOOL_NAMESPACE ?= applikasjonsplattform

SNAPSHOT_MOCK_JSON ?= $(SNAPSHOT_DIR)/kompass_snapshot_mock.json
SNAPSHOT_TOOL_JSON ?= $(SNAPSHOT_DIR)/kompass_snapshot_tool_app.json
SNAPSHOT_MOCK_TREE ?= $(SNAPSHOT_DIR)/kompass_snapshot_mock.txt
SNAPSHOT_TOOL_TREE ?= $(SNAPSHOT_DIR)/kompass_snapshot_tool_app.txt

build:
	@go build $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") -o kompass cmd/kompass/*.go

build-release: LDFLAGS := $(VERSION_LDFLAGS)
build-release: build

test: build
	@go test ./...

cover: build
	@go test ./... -coverpkg=$(COVERPKG) -coverprofile=coverage.out -covermode=atomic >/dev/null 2>&1 || true
	@echo "┌─────────────────────────────────────────────────────────┬──────────┬──────────┐"
	@echo "│ Package                                                 │  LOCAL   │  CROSS   │"
	@echo "├─────────────────────────────────────────────────────────┼──────────┼──────────┤"
	@for pkg in $$(go list ./...); do \
		local_cov=$$(go test $$pkg -cover 2>&1 | grep -o 'coverage: [0-9.]*%' | cut -d' ' -f2); \
		cross_cov=$$(awk -v p="$$pkg" 'NR>1 { split($$1, a, ":"); file=a[1]; pkg=file; sub("/[^/]+$$", "", pkg); if (pkg==p) { total += $$2; if ($$3 > 0) covered += $$2 } } END { if (total > 0) printf "%.1f%%", (covered/total)*100 }' coverage.out); \
		if [ -n "$$local_cov" ] || [ -n "$$cross_cov" ]; then \
			if [ -z "$$local_cov" ]; then local_cov="-"; fi; \
			if [ -z "$$cross_cov" ]; then cross_cov="-"; fi; \
			printf "│ %-55s │ %7s  │ %7s  │\n" $$pkg $$local_cov $$cross_cov; \
		fi; \
	done
	@echo "├─────────────────────────────────────────────────────────┼──────────┼──────────┤"
	@go tool cover -func=coverage.out | grep 'total:' | awk '{printf "│ %-55s │ %7s  │ %7s  │\n", "TOTAL", "-", $$3}'
	@echo "└─────────────────────────────────────────────────────────┴──────────┴──────────┘"
	@echo
	@echo "┌───────────────────────────────────────────────────────────────────────────────┐"
	@echo "│ Function Coverage                                                             │"
	@echo "├────────────────────────────────────────────────────────────────────┬──────────┤"
	@go tool cover -func=coverage.out | grep -v 'total:' | awk '{printf "│ %-66s │ %7s  │\n", substr($$1":"$$2, 1, 66), $$3}'
	@echo "└────────────────────────────────────────────────────────────────────┴──────────┘"

help:
	@go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass --help

mock:
	@go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass --mock $(ARGS)

real:
	@go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass $(ARGS)

snapshot-json:
	@echo "Generating JSON snapshots for mock and pinned tool context/namespace..."
	@go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass --json --mock -n $(SNAPSHOT_MOCK_NAMESPACE) > $(SNAPSHOT_MOCK_JSON)
	@go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass --json -c $(SNAPSHOT_TOOL_CONTEXT) -n $(SNAPSHOT_TOOL_NAMESPACE) > $(SNAPSHOT_TOOL_JSON)
	@echo "Wrote $(SNAPSHOT_MOCK_JSON)"
	@echo "Wrote $(SNAPSHOT_TOOL_JSON)"

snapshot-tree:
	@echo "Generating tree snapshots for mock and pinned tool context/namespace..."
	@go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass --mock -n $(SNAPSHOT_MOCK_NAMESPACE) > $(SNAPSHOT_MOCK_TREE)
	@go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass -c $(SNAPSHOT_TOOL_CONTEXT) -n $(SNAPSHOT_TOOL_NAMESPACE) > $(SNAPSHOT_TOOL_TREE)
	@echo "Wrote $(SNAPSHOT_MOCK_TREE)"
	@echo "Wrote $(SNAPSHOT_TOOL_TREE)"

snapshot:
	@$(MAKE) snapshot-json
	@$(MAKE) snapshot-tree
