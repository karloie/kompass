.PHONY: test build build-release coverage dev web-build snapshot mock real tui service help

GIT_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION_LDFLAGS := -X github.com/karloie/kompass/pkg/graph.GitVersion=$(GIT_VERSION) -X github.com/karloie/kompass/pkg/graph.GitCommit=$(GIT_COMMIT)
RELEASE_LDFLAGS := -s -w $(VERSION_LDFLAGS)
LDFLAGS ?=
ARGS    ?=
COVERPKG ?= ./...
GO_RUN   = go run $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") ./cmd/kompass
GOW     ?= gow

SNAP_DIR            ?= testdata/fixtures
SNAP_MOCK_NAMESPACE ?= petshop
SNAP_REAL_CONTEXT   ?= tool-test-01
SNAP_REAL_NAMESPACE ?= applikasjonsplattform

build: test
	go build $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") -o kompass ./cmd/kompass
	@OUT_SIZE=$$(du -hs kompass | cut -f1); OUT_PATH=$$(realpath kompass); \
	echo "\n$$OUT_PATH $(GIT_VERSION) # $(GIT_COMMIT) ~ $$OUT_SIZE"

build-release: LDFLAGS := $(RELEASE_LDFLAGS)
build-release: test web-build
	go build -tags webembed $(if $(strip $(LDFLAGS)),-ldflags "$(LDFLAGS)") -o kompass ./cmd/kompass
	@OUT_SIZE=$$(du -hs kompass | cut -f1); OUT_PATH=$$(realpath kompass); \
	echo "\n$$OUT_PATH $(GIT_VERSION) # $(GIT_COMMIT) ~ $$OUT_SIZE"

test:
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

web-build:
	npm run build

dev:
	@set -eu; \
	echo "Cleaning stale dev processes on :8080 and :8081..."; \
	for port in 8080 8081; do \
		pids=$$(ss -ltnp "( sport = :$$port )" 2>/dev/null | awk -F'pid=' 'NR>1 { split($$2, a, ","); if (a[1] != "") print a[1] }' | sort -u); \
		if [ -n "$$pids" ]; then \
			echo "Killing stale listeners on :$$port -> $$pids"; \
			kill $$pids >/dev/null 2>&1 || true; \
		fi; \
	done; \
	sleep 1; \
	for port in 8080 8081; do \
		if ss -ltn "( sport = :$$port )" | awk 'NR>1 { found=1 } END { exit(found ? 0 : 1) }'; then \
			echo "Error: port :$$port is still in use"; \
			exit 1; \
		fi; \
	done; \
	$(GOW) run ./cmd/kompass --mock --service $(ARGS) & backend_pid=$$!; \
	npm run dev & frontend_pid=$$!; \
	trap 'echo "\nStopping dev processes..."; kill $$backend_pid $$frontend_pid >/dev/null 2>&1 || true' INT TERM EXIT; \
	wait

help:    ; @$(GO_RUN) --help
mock:    ; @$(GO_RUN) --mock $(ARGS)
real:    ; @$(GO_RUN) $(ARGS)
tui:     ; @$(GO_RUN) --tui $(ARGS)
service: ; @$(GO_RUN) --mock --service $(ARGS)

snapshot:
	$(GO_RUN) --json --mock -n $(SNAP_MOCK_NAMESPACE) > $(SNAP_DIR)/mock.json
	$(GO_RUN) --json  -c $(SNAP_REAL_CONTEXT) -n $(SNAP_REAL_NAMESPACE) > $(SNAP_DIR)/real.json
	$(GO_RUN)        --mock -n $(SNAP_MOCK_NAMESPACE) > $(SNAP_DIR)/mock.txt
	$(GO_RUN)         -c $(SNAP_REAL_CONTEXT) -n $(SNAP_REAL_NAMESPACE) > $(SNAP_DIR)/real.txt
