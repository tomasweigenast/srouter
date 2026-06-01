# Changelog

All notable changes to this project will be documented in this file.

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
