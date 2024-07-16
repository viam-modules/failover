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
	go mod download github.com/golangci/golangci-lint
	go mod tidy
	GOBIN=`pwd`/$(TOOL_BIN) go install \
		github.com/golangci/golangci-lint/cmd/golangci-lint

lint: tool-install
	go mod tidy
	golangci-lint run -v --fix --config=./etc/.golangci.yaml


