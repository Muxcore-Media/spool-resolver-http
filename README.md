# Spool Resolver HTTP

[![Go Version](https://img.shields.io/badge/Go-1.26-blue)](https://go.dev/)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)

**HTTPS JSON spool tag resolver for the MuxCore fabric (SSRF-hardened).**

A MuxCore sidecar module that resolves tag definitions from a remote HTTP(S) spool endpoint. Given a spool URL and a tag name, it fetches the tag definition as JSON, decoding into `contracts.TagDefinition`.

## How It Works

```
Tag request ──→ spool-resolver-http (gRPC) ──→ GET <spool_url>/<tag_name>
                                                    │
                                                    ▼
                                              JSON TagDefinition returned
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SPOOL_RESOLVER_GRPC_ADDR` | `:9670` | gRPC listen address |
| `SPOOL_RESOLVER_ALLOWED_HOSTS` | *(required, empty=deny)* | Comma-separated host allowlist for spool URLs |
| `SPOOL_RESOLVER_ALLOW_HTTP` | `false` | Permit `http://` (local/dev only) |
| `SPOOL_RESOLVER_ALLOW_PRIVATE` | `false` | Permit private/link-local resolved IPs |
| `MUXCORE_GRPC_ADDR` | – | Core gRPC / mesh address (required) |
| `MUXCORE_MODULE_ID` | `spool-resolver-http` | Module ID override (SDK) |
| `MUXCORE_INSECURE_DISABLE_TLS` | unset | Set to `true` to disable TLS (dev only) |

## RPCs

- `ResolveTag(spoolUrl, tagName)` — `GET <spoolUrl>/<tagName>` and return a tag definition

On success, response carries `tag_definition_json`. Fetch/decode failures set the response `error` field (gRPC status remains OK).

## Dependencies

- `google.golang.org/grpc` — gRPC service framework
- `github.com/Muxcore-Media/core/pkg/contracts` — `TagDefinition`, module contracts
- `github.com/Muxcore-Media/core/sdk/go/module` — sidecar lifecycle SDK
- `github.com/Muxcore-Media/core` — spoolresolver proto

## Build & Develop

```bash
make build   # compile the module binary
make test    # run tests with race detection
make lint    # golangci-lint
make fmt     # format Go source
make docker  # build Docker image
```

## Capabilities

- Registers with capability: `spool.resolver`
- Declares contract: `SpoolResolver` (`muxcore.json` / `ModuleInfo`)
- Serves gRPC `SpoolResolverService` (`ResolveTag`)
- HTTP client timeout defaults to 30s (`Config.Timeout`; not env-exposed)

## License

GPL-3.0
