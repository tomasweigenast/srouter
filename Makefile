BINARY    := srouter
CMD       := ./cmd
BUILD_DIR := ./bin
VENDOR    := web/static/vendor

export CGO_ENABLED=1

.PHONY: build dev dev-mac test test-race lint clean assets css copy-assets install

build: install assets
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

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
	cp node_modules/htmx.org/dist/htmx.min.js      $(VENDOR)/htmx.min.js
	cp node_modules/htmx-ext-sse/sse.js             $(VENDOR)/sse.js
	cp node_modules/chart.js/dist/chart.umd.min.js  $(VENDOR)/chart.min.js
