#!/bin/sh
# Build srouter for Alpine Linux inside a Docker container.
# The container is removed after the build. Named volumes cache
# Go modules and node_modules so subsequent builds are fast.
#
# Usage:
#   ./scripts/build-linux.sh          # build only
#   DEPLOY=1 ./scripts/build-linux.sh # build + deploy to router

set -e
cd "$(dirname "$0")/.."

BINARY=bin/srouter-linux
ARCHIVE=bin/srouter-linux.tar.gz
ROUTER_HOST=${ROUTER_HOST:-root@192.168.0.1}
ROUTER_DEST=/usr/local/bin/srouter

echo "==> Creating cache volumes (no-op if they already exist)..."
docker volume create srouter-gomod   >/dev/null
docker volume create srouter-npmcache >/dev/null

echo "==> Building inside golang:1.25-alpine container..."
docker run --rm \
  -v "$(pwd)":/build \
  -v srouter-gomod:/root/go/pkg/mod \
  -v srouter-npmcache:/build/node_modules \
  -w /build \
  golang:1.25-alpine \
  sh << 'INNER'

set -e
echo "  -> deps..."
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
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
CGO_ENABLED=1 go build \
  -ldflags "-X github.com/tomasweigenast/srouter/internal/system.AppVersion=${VERSION}" \
  -o bin/srouter-linux ./cmd

INNER

echo "==> Binary: ${BINARY}"

echo "==> Packaging ${ARCHIVE}..."
mkdir -p bin
tar -czf "${ARCHIVE}" \
  -C bin srouter-linux \
  -C "$(pwd)" scripts/install.sh
echo "    Contents: srouter-linux, scripts/install.sh"

echo ""
echo "==> Done."
echo ""
echo "Deploy to router:"
echo "  scp ${ARCHIVE} ${ROUTER_HOST}:~/"
echo "  ssh ${ROUTER_HOST} 'tar xzf srouter-linux.tar.gz && sh install.sh'"
echo ""
echo "Or set DEPLOY=1 to do it automatically:"
echo "  DEPLOY=1 ./scripts/build-linux.sh"

if [ "${DEPLOY}" = "1" ]; then
  echo ""
  echo "==> Deploying to ${ROUTER_HOST}..."
  scp "${ARCHIVE}" "${ROUTER_HOST}:~/srouter-linux.tar.gz"
  ssh "${ROUTER_HOST}" "tar xzf ~/srouter-linux.tar.gz -C ~/ && sh ~/install.sh"
  echo "==> Deployed and restarted."
fi
