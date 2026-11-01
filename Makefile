.PHONY: test test-integration lint bench quality\:pr golangci-lint

GOLANGCI_VERSION ?= v1.64.8
GOPATH_BIN := $(shell go env GOPATH)/bin
GOLANGCI := $(GOPATH_BIN)/golangci-lint

test:
	go test ./... -count=1

test-integration:
	@run_integration=0; \
	if [ "$${ARGUS_TEST_REDIS:-}" = "1" ]; then run_integration=1; \
	elif command -v redis-cli >/dev/null 2>&1 && redis-cli ping 2>/dev/null | grep -q PONG; then run_integration=1; \
	fi; \
	if [ "$$run_integration" = "1" ]; then \
		go test ./test/integration/... -count=1 -tags=integration; \
	else \
		echo "Skipping integration tests (set ARGUS_TEST_REDIS=1 or start Redis)"; \
	fi

golangci-lint:
	@if [ ! -x "$(GOLANGCI)" ]; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION); \
	fi

lint: golangci-lint
	"$(GOLANGCI)" run ./...

bench:
	go test ./... -bench=. -benchtime=100ms -run=^$$ -count=1

quality\:pr: lint test test-integration bench
