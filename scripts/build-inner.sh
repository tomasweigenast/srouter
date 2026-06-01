#!/bin/sh
# Runs INSIDE the srouter-builder Docker container.
# gcc, musl-dev, linux-pam-dev and bun are pre-installed in the image.
set -e

echo "  -> frontend deps..."
bun install --silent

echo "  -> assets..."
mkdir -p web/static/vendor
bunx tailwindcss -i web/static/input.css -o web/static/vendor/tailwind.css --minify 2>/dev/null
cp node_modules/htmx.org/dist/htmx.min.js          web/static/vendor/htmx.min.js
cp node_modules/htmx-ext-sse/sse.js                 web/static/vendor/sse.js
cp node_modules/chart.js/dist/chart.umd.min.js      web/static/vendor/chart.min.js
cp node_modules/codemirror/lib/codemirror.js         web/static/vendor/codemirror.js
cp node_modules/codemirror/lib/codemirror.css        web/static/vendor/codemirror.css
cp node_modules/codemirror/mode/shell/shell.js       web/static/vendor/codemirror-shell.js
cp node_modules/codemirror/theme/monokai.css         web/static/vendor/codemirror-monokai.css

echo "  -> go build..."
mkdir -p bin
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X github.com/tomasweigenast/srouter/internal/system.AppVersion=${VERSION}" \
  -o bin/srouter-linux ./cmd
echo "  -> done: bin/srouter-linux"
