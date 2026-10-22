# Migration Guide

[Documentation index](README.md) · [Repository entry](../README.md)

## v2 to v3 clean replacement

Version 3 keeps one owner for each mechanism and removes the superseded API
instead of retaining compatibility switches or parallel implementations:

| Removed in v3 | Replacement |
| --- | --- |
| Local Retry-After parsing | `github.com/faustbrian/go-retry/v3/adapters/http.ParseRetryAfter`, used by retry and both rate-limit observers |
| HTTP-owned backoff, jitter, budget and exhaustion engine | `github.com/faustbrian/go-retry/v3` executes the schedule; the middleware keeps HTTP eligibility, replay and response cleanup |
| `RetryOptions.Jitter`, `RetryJitter`, `RetryJitterFunc` | `RetryOptions.Random` (a `retry.Random`), applied to exponential delays only |
| `RetryExhaustedError`, `ErrRetryExhausted` | `retry.ExhaustedError`, `retry.BudgetError`, `retry.PermanentError`, `retry.ContextError`; see [retries](retries.md) for where each exposes the last status |
| `ReadOptions.IgnoreCloseError` | none: close failures are always reported with the complete body; to accept a close-only failure, use the returned body when `errors.As` finds a `ResponseBodyError` whose `Operation` is `"close"` and no read or limit error is present |
| `TelemetryObserver`, `TelemetryOptions.Observer`, `SlogTelemetryObserver.Start` | `BeginTelemetryObserver`, `TelemetryOptions.BeginObserver`, `Begin` |
| `Config.Timeout` | `Config.Policy.OperationTimeout` |

Behavior changes to review when adopting v3:

- Retry-After is a minimum wait: it lengthens a shorter backoff but no longer
  shortens it, so `Retry-After: 0` waits the backoff. Every delay, including a
  hint, is capped by `MaximumRetryAfter`; `MaximumDelay` and explicit `Delays`
  must not exceed it. An unset `MaximumDelay` defaults to the lower of 2s and
  `MaximumRetryAfter`, and an unset `BaseDelay` to the lower of 100ms and
  `MaximumDelay`, so lowering only `MaximumRetryAfter` stays valid.
- A custom `RetryClock` that moves backwards stops with
  `ErrRetryClockRegressed` (telemetry `retry_exhausted`) instead of the removed
  exhaustion error.
- Retry-After values longer than 128 raw bytes are ignored. Numeric values
  above 2^31-1 seconds are accepted instead of rejected, saturating only beyond
  the duration range, and are then capped by `MaximumRetryAfter`. Vendor
  remaining/reset parsing is unchanged.
- Explicit `Delays` are no longer jittered.
- A failed exchange that policy does not retry, such as an unsafe `POST`
  without idempotency proof, returns its original transport error instead of
  an exhaustion error. Non-retryable responses are returned unchanged as
  before.
- `MaximumElapsed` stops retries between exchanges; it is still not a deadline
  for the exchange itself. A wait that ends at or past the budget now stops
  without another exchange, and that `BudgetError` also matches
  `context.DeadlineExceeded`; check `BudgetError` first.
- The default jitter source is one generator per middleware seeded from
  `crypto/rand`, instead of a `crypto/rand` read per delay. Inject
  `RetryOptions.Random` to control it.
- Telemetry: failed exchanges that policy does not retry, such as an unsafe
  `POST` without idempotency proof, and replay, non-replayable-body,
  response-cleanup and custom `RetryClock.Wait` stops report `failure` or `transport_error` instead of
  `retry_exhausted`, and
  attempt-limit and elapsed-budget stops report `retry_exhausted` ahead of
  cancellation, rate-limit and circuit outcomes.

Update imports to `github.com/faustbrian/go-http-client/v3` and replace each
removed identifier in the same change. Roll back by reverting the consumer
revision as a whole.

## Root v2 import identity

For root `v2.0.0`, select
`github.com/faustbrian/go-http-client/v2@v2.0.0` and change application imports
to `github.com/faustbrian/go-http-client/v2`. There are no independent nested
modules or version-specific source directories.

Root v1 and v2 exported types have distinct Go identities. Migrate each
composed request/client boundary together. A published wrapper that exports
`RequestSpec` or `LayerOptions`, including Localized v4, cannot substitute v2
types in a compatible patch; it needs an explicit compatible route or a
separately authorized major migration. Maintained HTTP and Service integration
fixtures adopt the actual public v2 module only after publication. Existing v1
consumers do not acquire v2 credential bounds by retaining their old imports.

## Finite credential admission

Built-in authentication now rejects oversized ordinary-valid credentials and
returned OAuth token fields instead of scanning, encoding or caching them
without a credential policy. This intentionally narrows previously accepted
inputs and requires a new major release from main, with the corresponding Go
major module/import suffix; do not treat it as a patch release.

Existing constructor function signatures are unchanged. Existing keyed options
literals keep working with finite defaults; update unkeyed literals for the new
`CredentialPolicy` field. Applications that need different supported budgets
must use the additive `WithPolicy` constructors or the option field, and handle
secret-safe admission errors. Zero means finite defaults, not unlimited. Review
both aggregate raw sizes and encoded expansion before selecting limits.

Use a consistent policy on a token source/cache and its editor if they should
accept the same credentials. A stricter editor can reject a token admitted by a
more generous cache, without changing the request. Opaque Extra metadata keeps
its previous shallow sharing semantics; it is not a deep independent copy or a
bounded metadata representation. Its trusted source/application owner must
bound acquisition and metadata, as described in the authentication cookbook.

## Adopting the module

1. Keep vendor DTOs and endpoint methods in the existing vendor package.
2. Replace ad hoc transports with one shared `httpclient.Client` and preserve a
   close path owned by that wrapper.
3. Move credentials into attempt-scoped authentication middleware.
   Credential origins now require HTTPS by default. Use `AllowInsecure` only
   for local test servers and migrate production HTTP endpoints before adding
   authentication.
4. Define endpoint retry and idempotency policy explicitly; remove nested retry
   loops from generated clients or outer application wrappers.
5. Classify status before bounded success decoding and make response ownership
   visible on every exit.
6. Add scope dimensions before enabling cache, cookies, OAuth refresh,
   coalescing, limiters, or breakers in multi-tenant code.
7. Prove the integration with strict scripted fixtures and `httptest.Server`.

## Upgrading releases

Read every changelog entry between versions, then review the compatibility
surfaces listed in `compatibility.md`. Re-run contract fixtures, redirect and
credential tests, retry/idempotency tests, race tests, and downstream compile
tests. Do not copy profile defaults into application code; inspect resolved
values and provenance instead.

Persisted fixture schemas never migrate implicitly. Register an explicit
`FixtureMigrator` for each understood source version, sanitize its output, and
rewrite it using the current recorder format. Expiry remains enforced after
migration.

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
