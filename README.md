# Spool Resolver HTTP

[![Go Version](https://img.shields.io/badge/Go-1.26-blue)](https://go.dev/)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)

**HTTPS JSON spool tag resolver for the MuxCore fabric.**

A MuxCore sidecar module that resolves tag definitions from a remote HTTP(S) spool endpoint. Given a spool URL and a tag name, it fetches the tag definition as JSON, validating the response for proper structure.

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
| `MUXCORE_GRPC_ADDR` | – | Core gRPC address |
| `MUXCORE_GRPC_INSECURE` | – | Disable TLS (dev mode) |

## RPCs

- `ResolveTag(spoolUrl, tagName)` — fetch a tag definition from a remote spool over HTTPS

Returns the tag definition as a JSON blob, or an error message if the spool is unreachable, returns a non-200 status, or returns malformed JSON.

## Dependencies

- `google.golang.org/grpc` — gRPC service framework
- `github.com/Muxcore-Media/core` — contracts and module SDK

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
- Implements `contracts.SpoolResolver`
- Configurable HTTP client timeout (default: 30s)

## License

GPL-3.0
