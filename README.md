# http-client

[![CI](https://github.com/faustbrian/go-http-client/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/faustbrian/go-http-client/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/badge/CodeQL-required-blue)](https://github.com/faustbrian/go-http-client/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Mutation](https://img.shields.io/badge/mutation-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Documentation](https://img.shields.io/badge/docs-checked_in_CI-blue)](docs/)
[![Go Reference](https://pkg.go.dev/badge/github.com/faustbrian/go-http-client.svg)](https://pkg.go.dev/github.com/faustbrian/go-http-client)
[![Release](https://img.shields.io/github/v/release/faustbrian/go-http-client?sort=semver)](https://github.com/faustbrian/go-http-client/releases)
[![Go](https://img.shields.io/badge/go-1.26.6-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`http-client` is a policy layer for typed outbound HTTP integrations. It builds
on `net/http` and adds finite transport defaults, immutable request
specifications, deterministic middleware, origin-bound authentication,
retries, rate limits, circuit breakers, caching, pagination, request pools,
transfers, telemetry, and explicit response ownership.

Use it when multiple integrations need the same security and lifecycle rules.
Use `net/http` directly when a small integration does not need those policies.
Vendor request and response models remain application-owned.

## Installation

```sh
go get github.com/faustbrian/go-http-client
```

## Quick start

```go
client, err := httpclient.New(httpclient.Config{})
if err != nil {
	return err
}
defer client.Close()

request, err := http.NewRequestWithContext(
	ctx,
	http.MethodGet,
	"https://api.example.com/widgets",
	nil,
)
if err != nil {
	return err
}

response, err := client.Do(request)
if err != nil {
	return err
}
defer response.Body.Close()
```

For reusable endpoints, build requests from an immutable `RequestSpec`. Each
build owns its URL, headers, query values, and body state.

## Guarantees

- Default clients use finite total and phase timeouts.
- Operation and attempt middleware execute in deterministic order.
- Authentication is HTTPS-only and same-origin unless explicitly widened.
- Retries require replayable bodies and drain discarded responses.
- Egress policy validates redirects, proxies, DNS answers, and dial targets.
- Cache, limiter, breaker, cookie, token, and telemetry state can be isolated
  by explicit policy scopes.
- Final raw responses remain caller-owned; consuming helpers close them.
- Errors and built-in telemetry omit credentials, bodies, query values, and
  arbitrary dependency diagnostics.

## Limitations

- `HTTPClient()` intentionally returns a reduced-guarantee `*http.Client` and
  bypasses package middleware.
- Custom transports and extension callbacks own their documented concurrency,
  cancellation, secrecy, and resource behavior.
- Retry elapsed policy does not interrupt an active attempt; use client or
  request deadlines for hard operation bounds.
- The package does not define vendor payloads, business retries, or endpoint
  idempotency semantics.

## Documentation

Start with the [documentation index](docs/README.md). The
[API reference](docs/api-reference.md), [transport guide](docs/transport.md),
[integration guide](docs/integrations.md), [security guide](docs/security.md),
and [specification decisions](docs/specification-decisions.md) define the main
contracts.

## Development

Run `make check` for the repository contract. Network, interoperability, and
performance changes must also pass their applicable focused gates.

## License

MIT. See [LICENSE](LICENSE).
