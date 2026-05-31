# srouter

Web dashboard for managing an Alpine Linux software router running as a VM in Proxmox. Provides visibility and control over PPPoE, DHCP/DNS, firewall rules, and port forwarding.

## Stack

| Concern | Choice |
|---|---|
| Language | Go 1.25 |
| HTTP router | `go-chi/chi` v5 |
| Database | SQLite via `modernc.org/sqlite` (pure Go, no CGO) |
| Migrations | `pressly/goose` v3 — SQL files in `internal/migrations/` |
| Config | TOML at `/etc/srouter/config.toml` |
| Auth | Linux-PAM — login with existing Alpine system users |
| Frontend | HTML templates + plain CSS/JS, SSE for live data |
| Logging | `log/slog` text handler to stderr |

> **Note:** PAM auth is the only CGO dependency. All other packages are pure Go.

## Development

### Requirements

- Go 1.25+
- [`air`](https://github.com/air-verse/air) for hot reload: `go install github.com/air-verse/air@latest`

### Run with hot reload

```bash
make dev
```

Watches `.go`, `.toml`, `.html`, `.css`, and `.js` files. Rebuilds and restarts on change.

### Build

```bash
make build        # outputs to ./bin/srouter
```

### Test

```bash
make test         # all tests
make test-race    # with race detector
```

### Lint

```bash
make lint         # requires golangci-lint
```

## Configuration

The server looks for `/etc/srouter/config.toml`. If the file does not exist, defaults are used. Environment variables prefixed with `SROUTER_` override any value from the config file.

```toml
port    = ":8080"
db_path = "/var/lib/srouter/data.db"
```

| Env var | Overrides | Default |
|---|---|---|
| `SROUTER_CONFIG_PATH` | path to config file | `/etc/srouter/config.toml` |
| `SROUTER_PORT` | `port` | `:8080` |
| `SROUTER_DB_PATH` | `db_path` | `/var/lib/srouter/data.db` |

For local development you can skip the config file entirely and use env vars:

```bash
SROUTER_PORT=:9090 SROUTER_DB_PATH=/tmp/srouter.db make dev
```

## Production setup (Alpine Linux)

### 1. Build on your development machine

```bash
GOOS=linux GOARCH=amd64 make build
```

### 2. Install on the router

```bash
scp bin/srouter root@192.168.0.1:/usr/local/bin/srouter
ssh root@192.168.0.1
```

On the router:

```bash
# PAM is the only native dependency
apk add linux-pam

# Create data directory
mkdir -p /var/lib/srouter

# Create config
mkdir -p /etc/srouter
cat > /etc/srouter/config.toml <<EOF
port    = ":8080"
db_path = "/var/lib/srouter/data.db"
EOF
```

### 3. OpenRC service

Create `/etc/init.d/srouter`:

```sh
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

Dashboard will be available at `http://192.168.0.1:8080`.

## Project layout

```
cmd/main.go               — entry point: wires deps, starts server
internal/
  config/                 — TOML config loader
  db/                     — SQLite open + goose migrations
  handler/                — HTTP handlers (one file per subsystem)
  middleware/             — PAM auth middleware
  migrations/             — SQL migration files + embed
  system/                 — shell/file access for router subsystems
web/
  static/                 — CSS, JS
  templates/              — HTML templates
```

## Router subsystems

| Subsystem | Config on router | Reload |
|---|---|---|
| DHCP/DNS | `/etc/dnsmasq.conf`, `/etc/dnsmasq.d/*.conf` | `rc-service dnsmasq restart` |
| Firewall | `/etc/firewall.sh` | `/etc/firewall.sh` |
| PPPoE | `/etc/ppp/peers/provider` | `rc-service pppoe restart` |
| Interfaces | `/etc/network/interfaces` | `rc-service networking restart` |
