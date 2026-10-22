# Retry Safety and Backoff

[Documentation index](README.md) · [Repository entry](../README.md)

Retry is disabled by default. Register `NewRetryMiddleware` only for endpoints
whose duplicate-request contract is understood. The middleware runs once per
logical operation and invokes the complete physical-attempt pipeline for every
exchange.

```go
retry, err := httpclient.NewRetryMiddleware(httpclient.RetryOptions{
	Name:              "read-widget-retry",
	Layer:             httpclient.MiddlewareEndpoint,
	MaximumAttempts:   3,
	MaximumElapsed:    10 * time.Second,
	BaseDelay:         100 * time.Millisecond,
	MaximumDelay:      2 * time.Second,
	MaximumRetryAfter: 30 * time.Second,
})
```

`MaximumAttempts` is required and includes the initial physical exchange. It
must be between 2 and 100. Zero duration fields select the finite defaults in
the example. The client-wide total timeout and caller deadline remain outer
bounds.

Provider contracts with a non-exponential schedule can set `Delays` instead:

```go
retry, err := httpclient.NewRetryMiddleware(httpclient.RetryOptions{
	Name:            "provider-retry",
	MaximumAttempts: 3,
	Delays:          []time.Duration{250 * time.Millisecond, time.Second},
})
```

`Delays` is copied during construction, must contain exactly one positive
duration for every possible retry, and cannot be combined with `BaseDelay` or
`MaximumDelay`.

## Executable safety rules

The default policy retries only when the request body is absent or has a
working `GetBody` replay factory. It recognizes `GET`, `HEAD`, `OPTIONS`,
`TRACE`, `PUT`, and `DELETE` as safe or idempotent HTTP methods. Eligible
outcomes are non-cancellation transport failures and statuses `408`, `425`,
`429`, `500`, `502`, `503`, and `504`.

`POST`, `PATCH`, and other unsafe methods are not retried merely because a key
is present. They require all of the following:

- endpoint `RetryUnsafeWithIdempotency` opt-in;
- idempotency middleware applied to this logical operation;
- a validated caller or generated key selected by that middleware; and
- a replayable body.

A context key without endpoint idempotency middleware is insufficient. This
prevents a correlation value from being mistaken for a provider duplicate-
suppression contract.

`RetryPolicy` is the escape hatch for provider-specific classification. Its
callback receives a bodyless request snapshot, response or failure, physical
attempt number, replayability, and verified idempotency-policy state. It must
not consume or close the response body. A custom policy owns the safety
consequences of returning true.

## Delay and `Retry-After`

Absent usable server direction, delay uses the configured explicit sequence or
grows exponentially from `BaseDelay` to `MaximumDelay`. Full jitter from
`github.com/faustbrian/go-retry/v3` selects a value from zero through the
current exponential bound to spread concurrent clients; explicit delays are
not jittered. The default random source is seeded from `crypto/rand` per
middleware; inject `RetryOptions.Random` for deterministic tests.

`Retry-After` is parsed by `go-retry/v3/adapters/http.ParseRetryAfter`, which
accepts delta seconds and HTTP dates within a 128-byte raw bound and saturates
oversized seconds. A valid value is a minimum wait: it lengthens a shorter
backoff but never shortens it. Every delay, including the hint, is capped by
`MaximumRetryAfter`, which must be at least `MaximumDelay` and every explicit
delay. Malformed or oversized values are treated as absent. A prospective wait
that would exceed `MaximumElapsed` is never started.

Every wait uses `RetryClock.Wait` with the operation context. The default uses
a stopped local timer, creates no goroutine, and returns promptly for caller
cancellation, deadline expiry, or client shutdown.

## Bodies, responses, and errors

Before another attempt, the middleware drains at most 64 KiB plus one byte and
closes the discarded response. Small responses can therefore return their
connection to the pool without allowing an unbounded error body to stall or
allocate. Drain or close failures stop retry.

Each replay calls the original request's `GetBody` and installs an independent
reader. A non-replayable stream never receives a second physical attempt.
Redirect replay remains under `net/http`; retry does not weaken its rules.

When a retryable exchange cannot continue, the middleware returns the
`go-retry/v3` terminal error: `ExhaustedError` for the attempt limit,
`BudgetError` for the elapsed budget, `PermanentError` for replayability or
body-lifecycle failures, and `ContextError` for cancellation during a wait or
between attempts. Cancellation during an exchange or response drain is a failed
exchange the policy does not retry, so it returns the original failure, which
still matches `context.Canceled`. A custom clock that reads earlier than the
first exchange stops with an elapsed `BudgetError` before any replay.
`ExhaustedError`,
`BudgetError` and `ContextError` expose `Result()` with attempt counts and
elapsed time. For attempt-limit, replay and body-lifecycle stops the last HTTP
status is reachable through `errors.As` to a value with `StatusCode() int`; for
budget and cancellation stops it is in the last `Result().History` entry. A
custom `RetryClock.Wait` that fails without a context error stops with a
`PermanentError` that does not carry the status. A `BudgetError` found after a
completed wait also matches `context.DeadlineExceeded` through `errors.Is`. Under
the default policy one found before the wait does not, but a custom policy that
retries a deadline failure carries that failure, so test for `BudgetError`
before treating a deadline as the caller's own. `Outcome()` on these errors
describes whether this middleware dispatched a retry, not whether the server
applied the request. None renders transport text,
URLs, headers, keys, or payloads. A failed exchange that policy does not retry
returns its original error, and non-retryable HTTP responses remain ordinary
`http.Response` values.

## Operation composition

One operation identity and idempotency key remain stable across retries. Every
physical attempt reruns session, idempotency propagation, authentication,
signing, and attempt telemetry. This refreshes expiring credentials and
signatures without regenerating logical identity.

Do not nest provider SDK retries, generated-client retries, and this middleware.
Choose one retry owner so maximum attempts cannot multiply. Direct calls to
`Client.HTTPClient().Do` bypass this policy.

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
