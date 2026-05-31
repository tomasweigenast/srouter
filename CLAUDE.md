# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`srouter` is a Go dashboard for managing an Alpine Linux software router running in a Proxmox VM. The router handles PPPoE (ISP connection), DHCP/DNS (dnsmasq), NAT/firewall (iptables), and port forwarding. The dashboard provides visibility and control over these subsystems.

Router details:
- LAN interface: `lan` at `192.168.0.1/24`, DHCP pool `192.168.0.100–200`
- WAN: PPPoE over `wan` → creates `ppp0` with public IP
- Port forwarding: TCP 25565 and UDP 24454 → `192.168.0.10` (Minecraft server)
- SSH access: `root@192.168.0.1`

## Commands

```bash
go build ./...          # build
go test ./...           # all tests
go test -run TestName ./pkg/...   # single test
go test -race ./...     # race detection
go vet ./...            # static analysis
```

## Architecture

**Flat architecture, manual dependency injection.**

```
srouter/
├── cmd/main.go             ← wire all dependencies, start HTTP server
├── internal/
│   ├── handler/            ← HTTP handlers; parse request, render template
│   │   ├── dashboard.go
│   │   ├── dhcp.go
│   │   └── firewall.go
│   └── system/             ← system access: shell commands, file I/O
│       ├── dnsmasq.go
│       ├── iptables.go
│       └── pppoe.go
└── web/
    └── templates/          ← HTML templates
```

Dependencies are passed as constructor arguments — no DI library. All wiring happens in `cmd/main.go`. Handlers call `system/` packages directly; no service layer unless a handler needs to orchestrate multiple system calls in non-trivial ways.

## Storage

SQLite via `modernc.org/sqlite` (pure Go, no CGO). DB file at `/var/lib/srouter/data.db`. Use standard `database/sql`.

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

db, err := sql.Open("sqlite", "/var/lib/srouter/data.db")
```

Schema migrations with `github.com/pressly/goose/v3`. SQL migration files live in `migrations/`. Run `goose.Up(db, "migrations")` on startup.

## Configuration

TOML config file at `/etc/srouter/config.toml`, parsed with `github.com/BurntSushi/toml`.

```toml
port = ":8080"
db_path = "/var/lib/srouter/data.db"
```

## Authentication

Linux-PAM via `github.com/msteinert/pam` (CGO required). Users authenticate with their existing Alpine Linux system credentials. This is the only CGO dependency — build requires `gcc` and `linux-pam-dev` on Alpine.

Alpine setup: `apk add linux-pam-dev gcc`

## HTTP Router

`github.com/go-chi/chi/v5`. Use chi middleware for request logging (`middleware.Logger`) and the PAM auth middleware. Mount handler groups by subsystem.

```go
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Use(authMiddleware)

r.Get("/", dashboardHandler.Index)
r.Route("/dhcp", func(r chi.Router) {
    r.Get("/", dhcpHandler.Index)
    r.Post("/reservations", dhcpHandler.AddReservation)
    r.Delete("/reservations/{mac}", dhcpHandler.DeleteReservation)
})
```

## Live Updates

Use Server-Sent Events (SSE) for auto-updating dashboard data. No WebSocket needed. Handlers write `text/event-stream` and the browser `EventSource` API consumes them.

## Logging

Use the `internal/logging` package. Never configure `slog` directly in other packages — `logging.init()` sets the global default.

Get a package-scoped logger with a `source` attribute:
```go
var logger = logging.GetLogger("dhcp")

logger.Info("dnsmasq reloaded", "leases", count)
logger.Error("iptables rule failed", "rule", rule, "err", err)
```

Do not call `slog.SetDefault` or construct `slog.Handler` anywhere outside `internal/logging`.

## README maintenance

**Always update `README.md` when you make changes that affect:**
- Build, dev, or run workflow
- Dependencies or tool requirements
- Production setup or deployment steps
- New subsystems, features, or configuration options

Keep README.md as the human-facing source of truth. CLAUDE.md is for AI context; README.md is for developers.

## Installed Skills

This project has Go coding skills in `.agents/skills/`. They define the authoritative conventions for this codebase:

| Skill | Governs |
|---|---|
| `golang-code-style` | Line length, control flow, variable declarations |
| `golang-naming` | Package, type, variable naming |
| `golang-error-handling` | Error wrapping (`%w`), single-handling rule, `slog` logging |
| `golang-testing` | Table-driven tests, `t.Parallel()`, build tags for integration tests |
| `golang-project-layout` | Directory structure, module naming |
| `golang-concurrency` | Goroutine patterns, `goleak` for leak detection |

Key conventions enforced by the skills:
- Errors wrapped with `fmt.Errorf("context: %w", err)`; logged OR returned, never both
- Slices and maps always initialized explicitly (never nil)
- Early returns for errors; no unnecessary `else` after `return`
- Integration tests use `//go:build integration` build tag
- Use `slog` for structured logging, not `fmt.Println` / `log.Printf`
- `samber/lo` for collection operations (filter, group-by, etc.)

## Domain Context

When building features, the dashboard interacts with these router subsystems:

| Subsystem | Config file | Reload command |
|---|---|---|
| Network interfaces | `/etc/network/interfaces` | `rc-service networking restart` |
| DHCP/DNS | `/etc/dnsmasq.conf`, `/etc/dnsmasq.d/*.conf` | `rc-service dnsmasq restart` |
| Firewall | `/etc/firewall.sh` (or `/etc/firewall/`) | `/etc/firewall.sh` |
| PPPoE | `/etc/ppp/peers/provider` | `rc-service pppoe restart` |

DHCP reservations go in `/etc/dnsmasq.d/reservas.conf` using `dhcp-host=MAC,IP,name` format. Reserved IPs must be in `192.168.0.2–99` (outside the dynamic pool).

Firewall rules are applied via iptables at boot through `/etc/local.d/firewall.start` → symlink to `/etc/firewall.sh`. The firewall starts before PPPoE so `ppp0` is always protected from first packet.
