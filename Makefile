BINARY   := srouter
CMD      := ./cmd
BUILD_DIR := ./bin

.PHONY: build dev test lint clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

dev:
	air

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)
