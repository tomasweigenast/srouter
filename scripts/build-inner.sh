#!/bin/sh
# Runs INSIDE the Alpine Linux Docker container — do not run directly on macOS.
set -e

echo "  -> build deps..."
apk add --no-cache gcc linux-pam-dev musl-dev curl git 2>/dev/null

echo "  -> bun..."
curl -fsSL https://bun.sh/install | sh >/dev/null 2>&1
export PATH="$HOME/.bun/bin:$PATH"

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
CGO_ENABLED=1 go build \
  -ldflags "-X github.com/tomasweigenast/srouter/internal/system.AppVersion=${VERSION}" \
  -o bin/srouter-linux ./cmd
echo "  -> done: bin/srouter-linux"
