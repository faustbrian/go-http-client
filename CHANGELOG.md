# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Specification Decision Records

- HTTPCLIENT-DEC-001 sha256:97631f28c8d6a17ae9747e9fa3c16d5c902dda58bac4c3c07cddb8115e41849f
- HTTPCLIENT-DEC-002 sha256:0dddb9fae23b8fcbda8bdecc8ed05fc74b66632089b5ca57840b1e731e9ba15b
- HTTPCLIENT-DEC-003 sha256:cdd49ac5871e94a252dba9b396546e67e9e5cc61f9a636f2a89277842d60e28a
- HTTPCLIENT-DEC-004 sha256:e32a0e50c2b4f42d9242a87bbfde2ec51f72e6ae9c40da9054e6c27cdab45d6b
- HTTPCLIENT-DEC-005 sha256:50d3457720427b4b10656b99983284388271841156226d62dd8c98d226762f5e
- HTTPCLIENT-DEC-006 sha256:61d54177f0bee624a4dfd8e207abd7e4773edd4358a94ea3793d016b4aca3c63
- HTTPCLIENT-DEC-007 sha256:4856c8dbfa149a0f1f88a4beb02ff128299c1fe298ef8e12a3a3e489561b8585
- HTTPCLIENT-DEC-008 sha256:8a1593f5ff0f594e3a4631b7b53ad3650c212abeca00d20a73997009bd978f7e
- HTTPCLIENT-DEC-009 sha256:06a81dfa129c2246eafe7762596a87d9b43576cdbc0df7eb975c272202efbf5f
- HTTPCLIENT-DEC-010 sha256:9aef52b08a239291beebd988ae2657c014a6134f56b43c7ca84af652b2f7e3ad
- HTTPCLIENT-DEC-011 sha256:ea9bb215ae9507866c64cce17f07ff665d950b2c11090e105f6d477ea3dc929b
- HTTPCLIENT-DEC-012 sha256:58c67f769e1005593db92f21b67a53b537bc718dae1da292150eabe758b00397
- HTTPCLIENT-DEC-013 sha256:c192d4d4787fa08ae696957a471e85441d8b350ebe115d271553198c11724a4b
- HTTPCLIENT-DEC-014 sha256:81e37fad9a73b7249ea62ba9d5cd90b754cb68abf2d25ae11a44c3b2e1f570af
- HTTPCLIENT-DEC-015 sha256:68f8acef8d28e36c4d4aac7a41389ad88bb0bb7575988aea21402e68a77835e2
- HTTPCLIENT-DEC-016 sha256:abed5656bcbc553f7866271dd3881a39aa6d31b1e33e0a4bc9fa5f428105fd40
- HTTPCLIENT-DEC-017 sha256:dc83d6dfa8316a43a44a04f0fec45f733b1037dbe617c4ce95e1ad442b20d45f
- HTTPCLIENT-DEC-018 sha256:588fb10a30c769f05ee4fe1fc82ddf25df91219e05fbdab23783cbdaf709fe08

[Specification decision register](docs/specification-decisions.md)

### Changed

- Adopt the checksum-verified `go-library-tools` v1.4.0 CLI and immutable
  shared workflow so bootstrap-first dependencies are checked against their
  public module identities before repository gates run.
- Adopt the checksum-verified `go-library-tools` v1.3.0 CLI, schema-v2 cohesion
  metadata, and repository-local cohesion gate while retaining package-owned
  source and evidence.
- Adopt the checksum-verified `go-library-tools` v1.2.0 CLI and immutable
  shared workflow so local and hosted gates enforce specification governance
  while retaining package-owned policy and verification evidence.

### Documentation

- Link the module to the immutable v1.4.0 Golib ecosystem guidance.
- Record RFC 9110 Erratum 9162 as behavior-neutral because repeated HTTP field
  values remain separate and the selected redirect, retry, representation,
  resume, idempotency, attempt, and replay policies do not depend on universal
  comma-folding separator spelling.
- Link the module to the immutable v1.3.0 Golib ecosystem guidance.
- Replace the oversized README, archived monorepo link, and dated pre-release
  verdict with a package-owned documentation index and durable security
  assurance guidance.

## [1.0.0] - 2026-08-25

### Compatibility

- Regenerate the exported API baseline with the repository's Go 1.26
  toolchain so JSON-backed contracts retain their intended stable identity.

### Changed

- Exclude intentional nested modules from root local-proxy archives so local,
  bootstrap, CI, and public module checksums describe the same source
  boundary.

- Track the pinned documentation-tool lockfile so clean CI checkouts install
  the exact validated cspell dependency.

- Reconcile standalone dependency checksums against deterministic current
  module archives so CI, local verification, and release consumers resolve
  identical content.

- Harden standalone documentation validation with deterministic spelling and
  link checks, package-specific documentation gates, and repository-local
  contributor guidance.

### Documentation

- Link the package README to package-owned documentation.

- Link the conformance source matrix directly to the canonical specification
  decision register.
- Record HTTP, authentication, retry, caching, range, pagination, cookie,
  response, and trace-context decisions against pinned normative sources and
  executable conformance evidence.

### Fixed

- Remove a redundant token-bucket timestamp assignment so static analysis
  reflects the reservation state that is actually committed.
- Mark the deliberate nil-context boundary assertion explicitly so strict
  static analysis does not reject the defensive contract test.
- Keep conditional revalidation header filtering deterministic regardless of
  validation-header map iteration order.
- Reject persisted fixtures whose expiry precedes their recording time during
  both serialization and loading.
- Stop retries safely when a custom retry clock moves backwards instead of
  allowing elapsed-budget subtraction to overflow.
- Allow a finite slice pool to complete successfully when its input count is
  exactly equal to `MaximumRequests`.
- Validate cached and client-credential OAuth2 tokens with the same injected
  clock used by their source instead of rechecking them against ambient wall
  time in the request editor.

### Compatibility

- Pin the circuit-breaker module to its published source revision so clean
  consumers can resolve the HTTP client before the first tagged release.
- Added a pinned module export baseline so incompatible public API changes
  fail the canonical repository gate.

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-http-client` identity while preserving its documented API and behavior.
- Replace the obsolete owned-module pseudo-version pin with the monorepo's
  local `v0.0.0` source-proxy coordinate; release tooling continues to emit
  the exact `v1.0.0` dependency version.
- Consolidate stale cache refresh and conditional revalidation onto one
  coalesced fetch path while preserving caller-supplied validators.
- Make cache capacity, freshness, invalidation, parser, and request-flight
  boundaries independently regression-tested at exact limits.
- Make fixture replay limits, persisted overrides, response policy, body
  capture, and canonicalization independently regression-tested.
- Make rate-limit defaults, admission bounds, window rollover, token refill,
  and reset parsing independently regression-tested at exact boundaries.
- Use a deterministic execution budget for default fuzz smoke campaigns while
  preserving explicit duration overrides for extended fuzzing.
- Normalized standalone module metadata against the canonical owned dependency
  graph, including complete checksums for clean consumer resolution.

### Security

- Reject padding-only and non-trailing-padding bearer tokens as malformed
  instead of accepting credentials without a token payload.
- Reject IPv6 authorities with an empty explicit port instead of interpreting
  them as the scheme default.
- Require HTTPS for trusted authentication origins by default, with an
  explicit insecure opt-in limited to local test endpoints.
- Verify that the insecure-origin opt-in permits HTTP only and still rejects
  non-HTTP credential origins.
- Reject direct-request origin userinfo and malformed ports before
  credential, session, scope, telemetry, or fixture origin policy uses them.
- Contain caller decoder and transfer-progress panics as typed secret-safe
  failures while preserving deterministic response-body closure.
- Close transformed request bodies when middleware short circuits before a
  physical transport, preventing blocked compression workers.

### Changed

- Expand the fuzz smoke gate with redirect credential-boundary and retry-policy
  targets plus a retained empty-method corpus case.
- Expand allocation benchmarks across request policy, pagination, pools,
  cache states and stampedes, limiter/breaker composition, body processing,
  request construction and serialization, policy scopes, and large fixtures.
- Clarify that `Client.HTTPClient()` bypasses operation identity, middleware,
  and target-URL egress policy.
- Pin current analyzer versions and keep pull-request and tag-release workflow
  prerequisites identical.

### Added

- Allow owned standard transports to configure the response-header timeout and
  disable environment proxy inheritance for direct-only egress contracts.
- Allow owned standard transports to configure TLS-handshake,
  idle-connection, and expect-continue timeouts for provider-specific
  connection-pool contracts.
- Reject standard-transport timeout controls when a caller-provided transport
  would otherwise silently ignore them.
- Allow retry policy to own an exact immutable delay sequence when provider
  contracts do not use exponential backoff.
- Add a context-aware cache for caller-owned OAuth2 token sources with
  coordinated refresh, independent token copies, client-bounded cancellation,
  and exact observed-token invalidation for provider-directed refreshes.
- Add public API, transport, typed integration, error, testing, security,
  compatibility, migration, performance, FAQ, and troubleshooting guides.
- Add executable GitHub REST and Ethereum JSON-RPC adoption examples that use
  deterministic local TLS servers and keep vendor types, status semantics, and
  protocol errors outside core.
- Add aggregate process-exit goroutine leak detection to normal, race, and
  uncached release-gate test execution.
- Add MIT licensing plus contribution, conduct, governance, security, support,
  issue, pull-request, attribution, and release policies for OSS operation.
- Add CI and local gates for format, vet, lint, tests, race detection, complete
  production coverage, fuzz smoke tests, benchmarks, docs, vulnerabilities,
  `GO-SAFETY-1`, and tagged GitHub releases.
- Fuzz hostile URLs, headers, authentication inputs and challenges, and bounded
  vendor error payload classification.
- Add live HTTP/1.1, HTTP/2, proxy, connection-reuse, and total-timeout
  integration coverage for the standard client contract.
- Add context-aware circuit outcome classification so caller cancellation is
  distinguishable from dependency-produced cancellation errors.
- Add an outbound HTTP client with finite total, connection, TLS handshake,
  response-header, idle-connection, and response-header-size limits.
- Add immutable egress policy for schemes, hosts, ports, origins, CIDRs,
  private address classes, metadata services, redirects, proxies, and
  connection-time DNS rebinding defense.
- Add immutable TLS policy for protocol minimums, private roots, fixed server
  names, client certificates, and rotating SHA-256 SPKI pins.
- Add opaque resource-specific policy scopes for origin, host, endpoint,
  credential, tenant, account, and caller-defined dimensions, with cache and
  coalescing identity separation.
- Add named versioned workload profiles with finite policy defaults,
  deterministic client and request overrides, and operation/attempt
  provenance inspection.
- Add optional operation/attempt telemetry, safe `slog` hooks, W3C Trace
  Context propagation, baggage allowlists, trust-boundary stripping, and
  closed low-cardinality metric labels.
- Add strict ordered scripted HTTP fixtures plus bounded sanitized recording,
  versioned persistence, explicit migration and expiry, stable failure modes,
  response trailers, and unused-interaction verification.
- Add explicit borrowed and owned transport lifecycles.
- Add deterministic client shutdown that cancels pending requests, closes
  active response bodies, and drains owned idle connection pools.
- Add typed transport errors that retain their cause without displaying URL
  credentials, query parameters, or fragments.
- Add immutable request specifications with same-origin URL resolution,
  non-aliasing request builds, and explicit metadata precedence.
- Add repeated, comma-delimited, space-delimited, pipe-delimited, deep-object,
  null, empty, omitted, and structurally custom query serialization.
- Add replayable byte and factory bodies plus explicitly one-shot streaming
  bodies with content metadata, `GetBody`, and typed open failures.
- Add canonical replayable form URL encoding with snapshotted values,
  deterministic key ordering, and preserved repeated-value order.
- Add deterministic bounded multipart request bodies with explicit part
  metadata, replay-derived retry safety, exact known lengths, streaming limit
  enforcement, and joined reader ownership.
- Add immutable layered request trailers with prohibited-field validation,
  independent request snapshots, replay preservation, and proven standard
  transport delivery.
- Add immutable operation and attempt middleware pipelines with explicit
  stages, registration layers, priorities, names, and resolved inspection.
- Add request and transport short-circuiting, response replacement, error
  recovery, completion hooks, cancellation propagation, and panic containment.
- Run attempt middleware for every physical `RoundTrip`, including redirects,
  while operation middleware runs once around the logical client call.
- Add immutable Basic, bearer, header API-key, explicit query API-key, and
  vendor-configurable HMAC request editors.
- Add origin-bound authentication middleware that reapplies credentials per
  attempt and strips sensitive headers across untrusted redirects.
- Add `golang.org/x/oauth2` token-source editors and a context-aware outbound
  client-credentials source with coordinated refresh and cancelable waiters.
- Send client-credentials token requests through the hardened standard
  transport while bypassing integration middleware, cookie jars, and
  integration retries.
- Add opt-in per-client cookie jars with a public-suffix default, same-origin
  redirect policy, explicit custom-jar ownership, and no ambient global jar.
- Add bounded session persistence loading, manual load/save operations, and
  save-on-close lifecycle with secret-safe typed errors.
- Add cryptographically random logical operation identity that remains stable
  across physical attempts and changes for every distinct client call.
- Add explicit endpoint idempotency middleware with caller and generated keys,
  entropy and length validation, provenance, redaction, and redirect policy.
- Order middleware by stage, priority, layer, and name so identity and
  idempotency policy can precede authentication across registration layers.
- Add bounded operation retry middleware with replay and method safety,
  explicit unsafe-endpoint opt-in, exponential jitter, `Retry-After`, and
  context-aware deterministic delay seams.
- Add typed, secret-safe retry exhaustion errors and bounded draining of every
  response discarded before another physical attempt.
- Add fixed-window, sliding-window, token-bucket, and bounded leaky-bucket
  admission controllers with context-aware maximum waits.
- Add per-attempt rate-limit middleware that observes RFC `Retry-After` and
  configurable vendor remaining/reset headers to defer future admission.
- Add logical-operation circuit-breaker middleware, HTTP outcome
  classification, fail-fast typed rejection, and a first-party
  `circuit-breaker` adapter without duplicating breaker state.
- Move initial limiter admission ahead of breaker admission while preserving
  exactly one reservation for every retry and redirect transport attempt.
- Add lazy typed pagination with resumable buffered state and finite page,
  item, byte, elapsed-time, empty-page, and continuation bounds.
- Add page-number, offset/limit, opaque-cursor, RFC Link-header, and custom
  continuation strategies with cycle detection and deterministic errors.
- Add typed bounded request pools for slices, generators, and channels with
  backpressure, stable keys, configurable result ordering, partial failures,
  cancellation, dynamic concurrency, and finite run-wide budgets.
- Add optional RFC-aware HTTP caching with finite in-memory storage, freshness
  and age calculation, validation, protected `Vary` identities, coalescing,
  bounded stale behavior, explicit controls, and same-origin invalidation.
- Add bounded streaming JSON response decoding with media-type validation,
  strict document boundaries, explicit empty semantics, and consume-and-close
  ownership.
- Add bounded caller-selected typed response codecs with explicit media-type
  allowlists, unread trailing-data policy, and the same consume-and-close
  ownership contract.
- Add shared declared response-length validation for JSON and custom codecs,
  including explicit-zero, unknown-length, and semantic-empty behavior.
- Add a public bounded response drain-and-close helper with exact-limit EOF
  detection and secret-safe typed read, close, and overflow failures.
- Add independent HTTP status classification with accepted-body preservation,
  bounded rejection draining, mandatory excerpt redaction, vendor mapping,
  request identity, and retryability metadata.
- Add explicit streaming gzip request and response policy with replay-safe
  request factories, deterministic worker shutdown, absolute decoded-size
  limits, and compressed-to-decompressed ratio protection.
- Add bounded response-to-writer transfers with explicit ownership, context
  cancellation, throttled progress, length checks, and constant-time SHA-256
  or SHA-512 validation.
- Add atomic response-to-file replacement with same-directory temporary files,
  restrictive modes, validation before rename, durable sync, and cleanup on
  failure.
- Add strict byte-range request and response policy with strong `If-Range`
  validators, `Content-Range` checks, and explicit continue, restart, or
  already-complete dispositions.
- Add resumable file downloads with persistent same-directory partials,
  validator-safe append, automatic full-response restart, append rollback,
  whole-file digest validation, and atomic publication.

[Unreleased]: https://github.com/faustbrian/go-http-client/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/faustbrian/go-http-client/releases/tag/v1.0.0
