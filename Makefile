.PHONY: test build coverage

GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION_LDFLAGS := -X github.com/karloie/kompass/pkg/graph.GitVersion=$(GIT_VERSION) -X github.com/karloie/kompass/pkg/graph.GitCommit=$(GIT_COMMIT)
RELEASE_LDFLAGS := -s -w $(VERSION_LDFLAGS)
LDFLAGS ?=
ARGS ?=
COVERPKG ?= ./...
GO_RUN = go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass

SNAP_DIR ?= testdata/fixtures

SNAP_MOCK_JSON ?= $(SNAP_DIR)/mock.json
SNAP_MOCK_TREE ?= $(SNAP_DIR)/mock.txt
SNAP_MOCK_NAMESPACE ?= petshop

SNAP_REAL_CONTEXT ?= tool-test-01
SNAP_REAL_NAMESPACE ?= applikasjonsplattform
SNAP_REAL_JSON ?= $(SNAP_DIR)/real.json
SNAP_REAL_TREE ?= $(SNAP_DIR)/real.txt

build:
	@echo
	go build $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") -o kompass cmd/kompass/*.go
	@OUT_SIZE=$$(du -hs kompass | cut -f1); \
	OUT_PATH=$$(realpath kompass); \
	echo "\n$$OUT_PATH $(GIT_VERSION) # $(GIT_COMMIT) ~ $$OUT_SIZE"

build-release: LDFLAGS := $(RELEASE_LDFLAGS)
build-release: build
	@echo

test: build
	@echo
	go test -count=1 ./...

coverage: build
	@go test -count=1 ./... -coverpkg=$(COVERPKG) -coverprofile=coverage.out -covermode=atomic >/dev/null 2>&1 || true
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

coverage-func: build
	@go test ./... -coverpkg=$(COVERPKG) -coverprofile=coverage.out -covermode=atomic >/dev/null 2>&1 || true
	@echo "┌───────────────────────────────────────────────────────────────────────────────┐"
	@echo "│ Function Coverage                                                             │"
	@echo "├────────────────────────────────────────────────────────────────────┬──────────┤"
	@go tool cover -func=coverage.out | grep -v 'total:' | awk '{printf "│ %-66s │ %7s  │\n", substr($$1":"$$2, 1, 66), $$3}'
	@echo "└────────────────────────────────────────────────────────────────────┴──────────┘"

help:
	@$(GO_RUN) --help

mock:
	@$(GO_RUN) --mock $(ARGS)

real:
	@$(GO_RUN) $(ARGS)

tui:
	@$(GO_RUN) --tui $(ARGS)

service:
	@$(GO_RUN) --mock --service $(ARGS)

snapshot:
	@$(GO_RUN) --json --mock -n $(SNAP_MOCK_NAMESPACE) > $(SNAP_MOCK_JSON)
	@echo "Wrote $(SNAP_MOCK_JSON)"
	@$(GO_RUN) --json -c $(SNAP_REAL_CONTEXT) -n $(SNAP_REAL_NAMESPACE) > $(SNAP_REAL_JSON)
	@echo "Wrote $(SNAP_REAL_JSON)"
	@$(GO_RUN) --mock -n $(SNAP_MOCK_NAMESPACE) > $(SNAP_MOCK_TREE)
	@echo "Wrote $(SNAP_MOCK_TREE)"
	@$(GO_RUN) -c $(SNAP_REAL_CONTEXT) -n $(SNAP_REAL_NAMESPACE) > $(SNAP_REAL_TREE)
	@echo "Wrote $(SNAP_REAL_TREE)"
