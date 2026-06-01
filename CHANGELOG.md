# Changelog

All notable changes to this project will be documented in this file.

## v1.0.1

### Fixed

- Self-update download no longer fails with `context canceled`. The install goroutine was inheriting the HTTP request context, which gets canceled as soon as the response is sent. Switched to `context.Background()` so the download runs to completion independently of the request lifecycle.

## v1.0.0

### Added

- Initial release.
