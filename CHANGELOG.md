# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0/).

## [0.1.5] - 2026-10-05

### Changed
- CI runs on GitHub-hosted runners from the umbrella template; retired-origin workflows removed.
- Dependencies resolve from published GitHub tags (no filesystem `replace`); requires core v0.6.0.

## [0.1.5] — 2026-08-31

### Added

- `/tags/{tagName}.json` fetch path aligned with `core/internal/spool.FetchTag`
- DNS-resolved private/loopback/metadata IP blocking (`blockPrivateHost`, `DialContext`)
- 1 MiB tag body cap via `io.LimitReader`
- `SPOOL_RESOLVER_TIMEOUT` env and live `timeout` setting
- SSRF tests: redirect re-validation, userinfo URLs, empty tag name, `allow_private` toggle, unknown setting key
- CI: sibling `core` checkout, `golangci-lint`, `go test -race`

### Changed

- Default gRPC bind `127.0.0.1:9675` (was `:9675`)
- `Health` fails when listener/server not serving; `Stop` closes listener
- `Info().HTTPAddr` empty (gRPC-only sidecar)
- `_mvp/run-host.sh` passes official-spool allowlist when enabled
- Core dependency and `SpoolResolver` contract **v0.5.8** (was v0.4.0)

## [0.1.4] — 2026-08-10

### Added

- Advertise `settings` capability so admin-ui discovers SettingsProvider without ListAll probing.

## [0.1.3] — 2026-08-10

### Added

- `RegisterSettings` / `SettingsUpdater` for live SSRF knobs: `allowed_hosts`, `allow_http`, `allow_private`
- Pin `core` / contracts / `sdk/go/module` to **v0.5.2**

## [0.1.2] — 2026-08-10

### Fixed

- Sync Info()/muxcore.json version to **0.1.2**.

## v0.1.1 (2026-08-10)

- HTTPS-by-default + `SPOOL_RESOLVER_ALLOWED_HOSTS` allowlist (default-deny)
- Safe tag names; redirect re-validation; private IP denial unless opted in
- Pin core/sdk to v0.5.1

## v0.1.0 (2026-08-09)

- Initial HTTPS JSON spool tag resolver
