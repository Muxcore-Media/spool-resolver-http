# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0/).

## [Unreleased]

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
