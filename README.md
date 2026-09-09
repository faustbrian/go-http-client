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

## Status and lifecycle

The module is a stable v1 library. The current stable release is `v1.0.0`,
the minimum supported Go version is 1.26.6, and development remains active.
It contains one public package and no independently versioned subpackages.

Construct a `Client` with `New`, share it across goroutines, and call `Close`
when its package-owned transport resources are no longer needed. Callers own
the context supplied to each operation and the body of every successful raw
response. Consuming response helpers take and close body ownership.

## Installation

```sh
go get github.com/faustbrian/go-http-client
```

## Quick start

```go
package main

import (
	"context"
	"log"
	"net/http"

	httpclient "github.com/faustbrian/go-http-client"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
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

	return nil
}
```

For reusable endpoints, build requests from an immutable `RequestSpec`. Each
build owns its URL, headers, query values, and body state.

## Package map

| Import path | Package | Use |
| --- | --- | --- |
| `github.com/faustbrian/go-http-client` | `httpclient` | Client lifecycle, immutable requests and bodies, middleware, authentication, retries and admission policies, caching, pagination, bounded response handling, transfers, egress and TLS policy, telemetry, and test fixtures. |

There are no public subpackages or adapter modules. Compose these companions
only at their distinct ownership boundaries:

- [HTTP Signature](https://github.com/faustbrian/go-http-signature) signs and
  verifies HTTP messages according to RFC 9421 and RFC 9530 profiles.
- [Idempotency](https://github.com/faustbrian/go-idempotency) provides durable
  ownership, fencing, and bounded replay across application workloads.
- [Rate Limit](https://github.com/faustbrian/go-rate-limit) owns inbound and
  application admission, not outbound HTTP pacing.
- [Retry](https://github.com/faustbrian/go-retry) supplies transport-neutral
  retry policy when work outside HTTP needs the same bounded semantics.
- [Hedge](https://github.com/faustbrian/go-hedge) owns explicitly replay-safe,
  finite concurrent duplicate attempts for tail-latency control.

This package remains the owner of outbound HTTP retries and rate pacing. A
composition must select one owner for each retry, limiter, breaker, bulkhead,
or hedge layer.

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

Start with the [documentation index](docs/README.md). Use the
[API reference](docs/api-reference.md), [adoption examples](docs/adoption-examples.md),
[integration guide](docs/integrations.md), and [testing helpers](docs/testing-fixtures.md)
to adopt the package. The [compatibility policy](COMPATIBILITY.md),
[migration guide](docs/migration.md), [performance guide](docs/performance.md),
[operations and troubleshooting FAQ](docs/faq-troubleshooting.md), and
[changelog](CHANGELOG.md) describe ongoing operation and upgrades.

See [support](SUPPORT.md) for usage and defect reports. Report vulnerabilities
through the private process in the [security policy](SECURITY.md); the
[security guide](docs/security.md) and
[specification decisions](docs/specification-decisions.md) define the package's
trust and protocol contracts.

## Development

Run `make inventory` for manifest consistency, `make check` for the complete
shared repository contract, and `make ci` for both repository and module gates.
The pinned `go-library-tools` release in `.golib.yaml` owns generic
verification; package-specific HTTP conformance remains in the repository.
See the versioned [Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)
and [package-family selection guidance](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/design-language.md#package-families-and-selection)
for the shared design language this module follows.

## License

MIT. See [LICENSE](LICENSE).
