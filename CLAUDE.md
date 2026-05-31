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
CGO_ENABLED=1 go build ./...    # build (CGO required for PAM)
make build                       # build + compile Tailwind CSS
make dev                         # hot reload via air
make download-assets             # download HTMX, Chart.js, SSE ext to web/static/vendor/
make css                         # compile Tailwind CSS (requires tailwindcss CLI)
go test ./...                    # all tests
go test -run TestName ./...      # single test
go test -race ./...              # race detection
```

## Architecture

**Flat architecture, manual dependency injection.**

```
cmd/main.go               — wire all deps, start HTTP server, launch background workers
internal/
  config/                 — TOML config + SROUTER_* env overrides (reflection-based)
  db/                     — SQLite open + goose migrations
  env/                    — config path helper
  handler/                — HTTP handlers; one struct per page, Routes() chi.Router method
  logging/                — GetLogger(name) returns *slog.Logger with source attribute
  middleware/             — RequireAuth(db) session cookie middleware
  migrations/             — *.sql files embedded via //go:embed
  session/                — session CRUD on SQLite (Create/Get/Delete/CleanupLoop)
  system/                 — all router subsystem access (file I/O, exec, /proc reads)
web/
  embed.go                — //go:embed + FuncMap (navItem, mb, gb, uptimeFmt)
  render.go               — MustParsePage, MustParseStandalone, RenderPartial, Render
  static/vendor/          — htmx.min.js, sse.js, chart.min.js, tailwind.css (gitignored)
  templates/layout.html   — sidebar nav, loads vendor assets
  templates/*.html        — one file per page
  templates/partials/     — HTMX-swapped fragments, SSE fragments
```

All wiring in `cmd/main.go`. Handlers call `system/` directly. Background workers (`Broadcaster`, `LogBroadcaster`) are started in `main` and injected into handlers that need them.

## Storage

SQLite via `modernc.org/sqlite` (pure Go, no CGO). DB file at `/var/lib/srouter/data.db`. Use standard `database/sql`.

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

db, err := sql.Open("sqlite", "/var/lib/srouter/data.db")
```

Schema migrations with `github.com/pressly/goose/v3`. SQL migration files live in `internal/migrations/`, embedded in the binary, and run automatically on startup via `internal/db.Open()`.

### Tables

| Table | Migration | Purpose |
|---|---|---|
| `sessions` | `001_initial.sql` | Authenticated dashboard sessions. UUID v4 PK, stores `username` (Linux PAM user) and `expires_at` (Unix timestamp). `CleanupLoop` purges expired rows hourly. |
| `wol_devices` | `002_wol_devices.sql` | Saved LAN devices for Wake-on-LAN. Stores `name` (display label), `mac` (colon-separated, unique), and optional `ip` (display hint only — the magic packet always goes to broadcast `255.255.255.255:9`). |

### Key Go types

| Type | Package | Maps to |
|---|---|---|
| `session.Session` | `internal/session` | `sessions` table |
| `system.WoLDevice` | `internal/system` | `wol_devices` table |

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

## Frontend patterns

**No CDN.** All JS/CSS assets are vendor-local in `web/static/vendor/` (gitignored, populated by `make download-assets` + `make css`). Templates reference them as `/static/vendor/...`.

**HTMX CRUD (partial swap):**
```html
<form hx-post="/dhcp/reservations" hx-target="#list" hx-swap="beforeend">
<button hx-delete="/dhcp/reservations/{{.MAC}}" hx-target="closest tr" hx-swap="outerHTML">
```

**SSE named events (dashboard stats, devices):**
```html
<div hx-ext="sse" sse-connect="/events/dashboard">
  <div id="stats" sse-swap="stats" hx-swap="innerHTML">...</div>
</div>
```
Server: `fmt.Fprintf(w, "event: stats\ndata: %s\n\n", html)` + `flusher.Flush()`.

**SSE to JS (bandwidth charts):** The `bwdata` SSE event carries JSON, not HTML. A JS listener on `sse:bwdata` calls into Chart.js. See `bandwidth.html`.

**SSE append (logs):** `hx-swap="beforeend"` on the log lines container, connected to `/logs/events?category=...&search=...`.

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
