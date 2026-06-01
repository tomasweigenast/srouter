BINARY    := srouter
CMD       := ./cmd
BUILD_DIR := ./bin
VENDOR    := web/static/vendor

export CGO_ENABLED=1

.PHONY: build build-linux deploy dev dev-mac test test-race lint clean assets css copy-assets install

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/tomasweigenast/srouter/internal/system.AppVersion=$(VERSION)

build: install assets
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD)

dev:
	air

# macOS development: mock data, no PAM, no /proc required
dev-mac:
	SROUTER_DEV_MODE=true air

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run ./...

ROUTER_HOST ?= root@192.168.0.1
LINUX_BINARY := $(BUILD_DIR)/srouter-linux
LINUX_ARCHIVE := $(BUILD_DIR)/srouter-linux.tar.gz

# Build for Alpine Linux inside a throwaway Docker container.
# scripts/build-inner.sh runs inside the container.
# Named volumes cache Go modules and node_modules between runs.
build-linux:
	@echo "==> Creating Docker cache volumes..."
	@docker volume create srouter-gomod    >/dev/null
	@docker volume create srouter-npmcache >/dev/null
	@echo "==> Building inside golang:1.25-alpine..."
	docker run --rm \
	  --platform linux/amd64 \
	  -v "$(CURDIR)":/build \
	  -v srouter-gomod:/root/go/pkg/mod \
	  -v srouter-npmcache:/build/node_modules \
	  -w /build \
	  golang:1.25-alpine \
	  sh scripts/build-inner.sh
	@echo "==> Packaging $(LINUX_ARCHIVE)..."
	@mkdir -p $(BUILD_DIR)
	tar -czf $(LINUX_ARCHIVE) \
	  -C $(BUILD_DIR) srouter-linux \
	  -C "$(CURDIR)/scripts" install.sh
	@echo "==> Done: $(LINUX_ARCHIVE)"
	@echo ""
	@echo "Deploy: scp $(LINUX_ARCHIVE) $(ROUTER_HOST):~/ && ssh $(ROUTER_HOST) 'tar xzf srouter-linux.tar.gz && sh install.sh'"

# Build + SCP + install on the router in one step.
deploy: build-linux
	scp $(LINUX_ARCHIVE) $(ROUTER_HOST):~/srouter-linux.tar.gz
	ssh $(ROUTER_HOST) "tar xzf ~/srouter-linux.tar.gz && sh ~/install.sh"

clean:
	rm -rf $(BUILD_DIR) tmp $(VENDOR)

install:
	bun install

assets: css copy-assets

css:
	mkdir -p $(VENDOR)
	bunx tailwindcss -i web/static/input.css -o $(VENDOR)/tailwind.css --minify

copy-assets:
	mkdir -p $(VENDOR)
	cp node_modules/chart.js/dist/chart.umd.min.js       $(VENDOR)/chart.min.js
	cp node_modules/codemirror/lib/codemirror.js          $(VENDOR)/codemirror.js
	cp node_modules/codemirror/lib/codemirror.css         $(VENDOR)/codemirror.css
	cp node_modules/codemirror/mode/shell/shell.js        $(VENDOR)/codemirror-shell.js
	cp node_modules/codemirror/theme/monokai.css          $(VENDOR)/codemirror-monokai.css
