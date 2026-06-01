# srouter

Web dashboard for managing an Alpine Linux software router running as a VM in Proxmox. Enterprise-grade visibility and control over PPPoE, DHCP/DNS, firewall, port forwarding, bandwidth, logs, and Wake-on-LAN.

## Stack

| Concern | Choice |
|---|---|
| Language | Go 1.25 |
| HTTP router | `go-chi/chi` v5 |
| DI | `samber/do` v2 — interface-based, real vs mock chosen at startup |
| Frontend | Go `html/template` + HTMX + Tailwind CSS v4 + Chart.js (all vendor-local, no CDN) |
| Live data | Server-Sent Events (SSE) |
| Database | SQLite via `modernc.org/sqlite` (pure Go, no CGO) |
| Migrations | `pressly/goose` v3 — SQL files in `internal/migrations/` |
| Config | TOML at `/etc/srouter/config.toml` |
| Auth | Linux-PAM (`msteinert/pam/v2`) — login with existing Alpine system users |
| Logging | `log/slog` text handler to stderr, package-scoped via `internal/logging` |

> **CGO:** `msteinert/pam` is the only CGO dependency. Build requires `gcc` and `linux-pam-dev`.
> All other packages (including SQLite) are pure Go.

## Dashboard pages

| Page | Features |
|---|---|
| Login | PAM auth with system Linux users |
| Dashboard | CPU/memory/disk/uptime, PPPoE WAN status, connected devices (ARP + DHCP merged), live SSE |
| DHCP | Active leases, static reservations CRUD, DHCP range config, dnsmasq reload |
| DNS | Upstream servers, local A records, DNS test lookup |
| Network | All interfaces (IP/MAC/MTU/state/RX/TX), routing table, ARP table, conntrack stats |
| Firewall | UI rule viewer (from kernel) + per-file raw editor for `/etc/firewall.d/*.sh` |
| Port Forwarding | Structured port forward CRUD (reads/writes `50-portforward.sh`, apply to reload) |
| Bandwidth | Real-time Chart.js graphs for `ppp0` + `lan` via SSE |
| Logs | Live-streaming `/var/log/messages` with category filter (firewall/DHCP/PPPoE/system) + search |
| Wake-on-LAN | Saved device list, send magic packet to any LAN device |

## Development

### Requirements

- Go 1.25+
- `gcc` and `libpam-dev` (macOS: included in Xcode CLT; Alpine: `apk add linux-pam-dev gcc`)
- [`air`](https://github.com/air-verse/air) for hot reload: `go install github.com/air-verse/air@latest`
- [bun](https://bun.sh) — manages all frontend dependencies (Tailwind, HTMX, Chart.js)

### Build

```bash
make build
```

That's the only command needed. It runs in order:
1. `bun install` — installs Tailwind, HTMX, Chart.js, SSE ext, CodeMirror 5
2. `bunx tailwindcss` — compiles `web/static/input.css` → `web/static/vendor/tailwind.css`
3. `cp` — copies JS/CSS libs from `node_modules/` → `web/static/vendor/`
4. `go build -ldflags "-X ...AppVersion=$(git describe)"` — embeds `vendor/` → `./bin/srouter`

The binary version shown in the dashboard is set automatically from `git describe --tags`.

### Develop on macOS (mock mode)

All Linux-specific calls (`/proc`, `rc-service`, PAM, `/etc/dnsmasq.conf`) are replaced by realistic fake data. Login with any non-empty username/password.

```bash
make dev-mac    # SROUTER_DEV_MODE=true air
```

### Develop on Linux / router

```bash
make dev        # requires PAM + Linux /proc
```

Watches `.go`, `.toml`, `.html`, `.css`, `.js`. Rebuilds and restarts. Uses `./tmp/data.db`.

### Test

```bash
make test         # all tests
make test-race    # with race detector
```

## Configuration

The server looks for `/etc/srouter/config.toml`. If the file does not exist, defaults are used. `SROUTER_*` env vars override any TOML value (derived from the field's toml tag: `db_path` → `SROUTER_DB_PATH`).

```toml
port    = ":8080"
db_path = "/var/lib/srouter/data.db"
```

| Env var | Overrides | Default |
|---|---|---|
| `SROUTER_CONFIG_PATH` | path to config file | `/etc/srouter/config.toml` |
| `SROUTER_PORT` | `port` | `:8080` |
| `SROUTER_DB_PATH` | `db_path` | `/var/lib/srouter/data.db` |
| `SROUTER_DEV_MODE` | `dev_mode` | `false` |
| `SROUTER_UPDATE_INTERVAL_MS` | `update_interval_ms` | `2000` |

Setting `SROUTER_DEV_MODE=true` bypasses PAM auth (any username works) and replaces all router system calls with mock data — used with `make dev-mac` on macOS.

## Production setup (Alpine Linux)

### Build for Linux (from macOS, requires Docker)

```bash
make build-linux
```

This runs a throwaway Alpine Linux Docker container that installs all build dependencies, compiles everything, and produces:
- `bin/srouter-linux` — the binary
- `bin/srouter-linux.tar.gz` — binary + `install.sh` bundled together

Named Docker volumes (`srouter-gomod`, `srouter-npmcache`) cache Go modules and node packages between builds so subsequent runs are fast.

### Install on the router

```bash
scp bin/srouter-linux.tar.gz root@192.168.0.1:~/
ssh root@192.168.0.1 'tar xzf srouter-linux.tar.gz && sh install.sh'
```

`install.sh` handles everything: installs `linux-pam`, copies the binary, creates the config (if missing), registers and starts the OpenRC service.

### Build + deploy in one command

```bash
make deploy                                    # deploys to root@192.168.0.1
ROUTER_HOST=admin@192.168.1.1 make deploy     # custom host
```

### 3. Install on the router (manual alternative)

```bash
scp bin/srouter root@192.168.0.1:/usr/local/bin/srouter
ssh root@192.168.0.1
```

On the router:

```bash
# Runtime PAM dependency
apk add linux-pam

# Create data directory and config
mkdir -p /var/lib/srouter /etc/srouter
cat > /etc/srouter/config.toml <<EOF
port    = ":8080"
db_path = "/var/lib/srouter/data.db"
EOF
```

### 4. OpenRC service

```sh
# /etc/init.d/srouter
#!/sbin/openrc-run

command="/usr/local/bin/srouter"
command_background=true
pidfile="/run/srouter.pid"
output_log="/var/log/srouter.log"
error_log="/var/log/srouter.log"

depend() {
    need net dnsmasq
    after firewall
}
```

```bash
chmod +x /etc/init.d/srouter
rc-update add srouter default
rc-service srouter start
```

Dashboard available at `http://192.168.0.1:8080`. Login with any Linux system user.

## Project layout

```
cmd/main.go               — entry point: wires all deps, starts HTTP server
internal/
  config/                 — TOML config loader with SROUTER_* env overrides
  db/                     — SQLite open + goose auto-migration
  env/                    — config path from SROUTER_CONFIG_PATH
  handler/                — HTTP handlers, one file per page
  logging/                — slog setup + GetLogger(name) helper
  middleware/             — PAM session auth middleware
  migrations/             — SQL files embedded in binary
  session/                — session CRUD on SQLite (24h TTL)
  system/                 — shell/file access for all router subsystems
web/
  embed.go                — //go:embed for templates + static
  render.go               — MustParsePage, RenderPartial helpers
  static/vendor/          — htmx.min.js, sse.js, chart.min.js, tailwind.css (gitignored)
  templates/              — layout.html, per-page templates, partials/
```

## Router subsystems reference

| Subsystem | Config on router | Reload |
|---|---|---|
| DHCP/DNS | `/etc/dnsmasq.conf`, `/etc/dnsmasq.d/reservas.conf` | `rc-service dnsmasq reload` |
| Firewall | `/etc/firewall.sh` (orchestrator), `/etc/firewall.d/*.sh` (rules) | `/etc/firewall.sh` |
| Port Forwarding | `/etc/firewall.d/50-portforward.sh` | `/etc/firewall.sh` |
| PPPoE | `/etc/ppp/peers/provider` | `rc-service pppoe restart` |
| Interfaces | `/etc/network/interfaces` | `rc-service networking restart` |
| Logs | `/var/log/messages` | — |
