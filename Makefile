APPLICATION_NAME    := github.com/allegro/vaas-registration-hook
APPLICATION_VERSION := $(shell git describe --tags | sed 's/^v\(.*\)/\1/' || echo "unknown")

LDFLAGS := -X main.Version=$(APPLICATION_VERSION)

BUILD_FOLDER := target
DIST_FOLDER := dist
GO_BUILD := go build -v -ldflags "$(LDFLAGS)" -a

BIN = $(CURRENT_DIR)/bin
CURRENT_DIR = $(shell pwd)
PATH := $(BIN):$(PATH)

.PHONY: clean test all build package deps lint lint-deps \
		generate-source generate-source-deps

all: lint test build

build: $(BUILD_FOLDER)
	$(GO_BUILD) -o $(BUILD_FOLDER)/vaas-hook ./cmd/vaas-hook
	chmod 0755 $(BUILD_FOLDER)/vaas-hook

$(BUILD_FOLDER):
	mkdir $(BUILD_FOLDER)

$(DIST_FOLDER):
	mkdir $(DIST_FOLDER)

clean:
	rm -rf $(BUILD_FOLDER)
	rm -rf $(CURRENT_DIR)/bin
	go clean -v ./cmd/vaas-hook

generate-source: generate-source-deps
	go generate -v $$(go list ./... | grep -v /vendor/)

generate-source-deps:
	go get -v -u golang.org/x/tools/cmd/stringer

lint: lint-deps
	$(BIN)/golangci-lint --version
	$(BIN)/golangci-lint run --config=golangcilinter.yaml ./...

lint-deps:
	@which golangci-lint > /dev/null || \
		(curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN) v1.30.0)

package: build $(DIST_FOLDER)
	zip -j $(DIST_FOLDER)/vaas-hook-$(APPLICATION_VERSION)-linux-amd64.zip $(BUILD_FOLDER)/vaas-hook

test: test-deps
	go test -v -coverprofile=$(BUILD_FOLDER)/coverage.txt -covermode=atomic ./...

test-deps: $(BUILD_FOLDER)

integration-test:
	scripts/integration_test.sh
