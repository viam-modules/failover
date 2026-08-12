BIN_OUTPUT_PATH = bin
TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)
UNAME_S ?= $(shell uname -s)

build:
	rm -f $(BIN_OUTPUT_PATH)/failover
	go build $(LDFLAGS) -o $(BIN_OUTPUT_PATH)/failover main.go

module.tar.gz: build
	rm -f $(BIN_OUTPUT_PATH)/module.tar.gz
	tar czf $(BIN_OUTPUT_PATH)/module.tar.gz $(BIN_OUTPUT_PATH)/failover

test:
	sudo apt install libnlopt-dev
	go test ./...


tool-install:
	# Pin tool versions so installs don't build against this module's
	# transitive deps (which can be incompatible with the tool's own go.mod).
	GOBIN=`pwd`/$(TOOL_BIN) go install github.com/edaniels/golinters/cmd/combined@v0.0.5-0.20220906153528-641155550742
	GOBIN=`pwd`/$(TOOL_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
	GOBIN=`pwd`/$(TOOL_BIN) go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.8

lint: tool-install
	go mod tidy
	$(TOOL_BIN)/golangci-lint run -v --fix --config=./etc/.golangci.yaml
