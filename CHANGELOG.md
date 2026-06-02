# Changelog

All notable changes to this project will be documented in this file.

## v1.1.3

### Added

- Success toast after update installs and the service comes back up.
- Port forward rules now include a LOG entry (`-j LOG --log-prefix "NAME: "`) so new connections appear in the Firewall logs.
- Port Forwarding page explains that it writes to `50-portforward.sh` and that manually written rules need a `# PF:` comment to appear in the UI.

## v1.1.2

### Fixed

- Self-update now works end-to-end on Alpine. The release binary was previously built on Ubuntu and dynamically linked against glibc, so Alpine's `start-stop-daemon` could not exec it (ELF interpreter `/lib64/ld-linux-x86-64.so.2` missing on musl systems). The CI now builds inside a `golang:1.25-alpine` container, producing a musl-linked binary.

## v1.1.1

### Fixed

- Self-updater now always writes the new binary to `/usr/local/bin/srouter` (the OpenRC service path) instead of resolving the path via `os.Executable()`. Previously, if srouter was running from a different path the binary would land in the wrong location and `rc-service srouter start` would fail.

### Changed

- All log lines now carry a `source=<subsystem>` field (e.g. `source=dhcp`, `source=updater`) via `logging.GetLogger`. Previously most packages used bare `slog.*` calls with no source context, making it hard to filter logs by subsystem.
- Log broadcaster (`/var/log/messages` tail) and bandwidth sampler (`/proc/net/dev`) now emit an error log if they fail to open their respective files instead of silently returning empty data.
- Network page now logs warnings when any of the underlying system calls (`GetInterfaces`, `GetARPTable`, `GetRoutes`, `GetConntrackStats`) fail.

## v1.1.0

### Added

- DNS-over-HTTPS (DoH) via `dnscrypt-proxy` and DNS-over-TLS (DoT) via `stubby`. Both can be toggled from the new DoH/DoT page with built-in providers (Cloudflare, Quad9, NextDNS) or a custom server.

### Fixed

- Speedtest latency check now pings `1.1.1.1` instead of `8.8.8.8`.

## v1.0.6

### Fixed

- dnsmasq reload now sends SIGHUP directly (`kill -HUP <pid>`) instead of going through `rc-service dnsmasq reload`. The OpenRC approach triggered the reverse-dependency chain (srouter depends on dnsmasq), causing reload to fail silently and leaving DNS changes unapplied until a manual restart.

## v1.0.4

### Fixed

- Self-update restart now works reliably. The previous approach (`rc-service srouter restart`) caused OpenRC to mark the service as crashed when srouter exited with a non-zero code (10 s shutdown timeout). The fix replaces it with a fully-detached shell script (`Setsid: true`) that sends SIGTERM to srouter, waits until the process is gone, clears any OpenRC ghost state, and then starts the service cleanly.
- Update install progress is now visible in the UI. Clicking "Install & restart" shows a spinner with live status text ("Downloading update…" → "Service restarting…" → "Update complete — reloading…"). The page polls `/ping` (new unauthenticated endpoint) to detect service-down and service-up transitions without requiring a valid auth session.
- Added detailed structured logging (`slog.Info`) throughout the install flow so each step is visible in `/var/log/srouter.log`.

## v1.0.2

### Fixed

- Internet connectivity status now defaults to **Reachable** on startup instead of Unreachable. The cache was zero-initialized to `false`, so the dashboard briefly showed "Unreachable" until the first background ping completed.

## v1.0.1

### Fixed

- Self-update download no longer fails with `context canceled`. The install goroutine was inheriting the HTTP request context, which gets canceled as soon as the response is sent. Switched to `context.Background()` so the download runs to completion independently of the request lifecycle.

## v1.0.0

### Added

- Initial release.
