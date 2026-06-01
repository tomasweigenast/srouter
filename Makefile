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

# Build for Alpine Linux inside a throwaway Docker container.
# Produces bin/srouter-linux + bin/srouter-linux.tar.gz (binary + install.sh).
# Named volumes cache Go modules and node_modules between runs.
build-linux:
	@sh scripts/build-linux.sh

# Build and deploy directly to the router in one step.
# Set ROUTER_HOST to override the default root@192.168.0.1.
deploy:
	@DEPLOY=1 sh scripts/build-linux.sh

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
	cp node_modules/htmx.org/dist/htmx.min.js           $(VENDOR)/htmx.min.js
	cp node_modules/htmx-ext-sse/sse.js                  $(VENDOR)/sse.js
	cp node_modules/chart.js/dist/chart.umd.min.js       $(VENDOR)/chart.min.js
	cp node_modules/codemirror/lib/codemirror.js          $(VENDOR)/codemirror.js
	cp node_modules/codemirror/lib/codemirror.css         $(VENDOR)/codemirror.css
	cp node_modules/codemirror/mode/shell/shell.js        $(VENDOR)/codemirror-shell.js
	cp node_modules/codemirror/theme/monokai.css          $(VENDOR)/codemirror-monokai.css
