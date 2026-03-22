BINARY := agents
MODULE := notb.re/agents
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w \
	-X $(MODULE)/cmd.Version=$(VERSION) \
	-X $(MODULE)/cmd.Commit=$(COMMIT) \
	-X $(MODULE)/cmd.Date=$(DATE) \
	-X $(MODULE)/internal/coding.HookVersion=$(VERSION)

.PHONY: build run clean install test test-integration dev

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

run:
	go run -ldflags "$(LDFLAGS)" .

clean:
	rm -f $(BINARY)

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

# test-integration builds the binary and runs the hook integration tests.
# These tests validate the full register → update-status flow without needing
# a live tmux session or a running coding agent.
test-integration:
	go test -v -run 'TestHook' ./cmd/

dev:
	air
