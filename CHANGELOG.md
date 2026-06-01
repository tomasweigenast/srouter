# Changelog

All notable changes to this project will be documented in this file.

## v1.0.2

### Fixed

- Internet connectivity status now defaults to **Reachable** on startup instead of Unreachable. The cache was zero-initialized to `false`, so the dashboard briefly showed "Unreachable" until the first background ping completed.

## v1.0.1

### Fixed

- Self-update download no longer fails with `context canceled`. The install goroutine was inheriting the HTTP request context, which gets canceled as soon as the response is sent. Switched to `context.Background()` so the download runs to completion independently of the request lifecycle.

## v1.0.0

### Added

- Initial release.
