package httpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	goretry "github.com/faustbrian/go-retry/v3"
)

func TestRetryReplaysSafeRequestsWithDeterministicBackoff(t *testing.T) {
	t.Parallel()

	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	retry, err := NewRetryMiddleware(RetryOptions{
		Name: "vendor-retry", Layer: MiddlewareEndpoint, MaximumAttempts: 3,
		BaseDelay: time.Second, MaximumDelay: 10 * time.Second, Clock: clock,
		Random: maxRetryRandom{},
	})
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}
	attempts := 0
	client, err := New(Config{
		Middleware: []Middleware{retry},
		Transport: TransportFunc(func(request *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("temporary network failure")
			}

			return &http.Response{StatusCode: http.StatusNoContent}, nil
		}),
	})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	defer closeTestClient(t, client)

	request, err := http.NewRequest(http.MethodGet, "https://api.example.test/items", nil)
	if err != nil {
		t.Fatalf("construct request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if got := clock.Delays(); len(got) != 2 || got[0] != time.Second || got[1] != 2*time.Second {
		t.Fatalf("backoff delays = %v", got)
	}
}

func TestRetryUsesExplicitOwnedDelaySequence(t *testing.T) {
	t.Parallel()

	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	delays := []time.Duration{250 * time.Millisecond, time.Second}
	retry, err := NewRetryMiddleware(RetryOptions{
		Name: "provider-retry", MaximumAttempts: 3, Delays: delays,
		Clock: clock, Random: zeroRetryRandom{},
	})
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}
	delays[0] = 10 * time.Second
	attempts := 0
	client, err := New(Config{
		Middleware: []Middleware{retry},
		Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("temporary")
			}
			return &http.Response{StatusCode: http.StatusNoContent}, nil
		}),
	})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}
	if got := clock.Delays(); len(got) != 2 || got[0] != 250*time.Millisecond || got[1] != time.Second {
		t.Fatalf("explicit delays = %v", got)
	}
}

func TestRetryRequiresReplayableBodyAndExplicitUnsafeOptIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		body       io.Reader
		idempotent bool
		contextKey bool
		unsafe     bool
		want       int
	}{
		{name: "safe get", method: http.MethodGet, want: 2},
		{name: "post has no endpoint contract", method: http.MethodPost, body: strings.NewReader("body"), idempotent: true, want: 1},
		{name: "post contract and key", method: http.MethodPost, body: strings.NewReader("body"), idempotent: true, unsafe: true, want: 2},
		{name: "post contract without key", method: http.MethodPost, body: strings.NewReader("body"), unsafe: true, want: 1},
		{name: "context key without endpoint policy", method: http.MethodPost, body: strings.NewReader("body"), contextKey: true, unsafe: true, want: 1},
		{name: "non replayable body", method: http.MethodPut, body: &retryOneShotReader{Reader: strings.NewReader("body")}, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
			retry, err := NewRetryMiddleware(RetryOptions{
				Name: "retry", Layer: MiddlewareEndpoint, MaximumAttempts: 2,
				BaseDelay: time.Millisecond, MaximumDelay: time.Millisecond,
				Clock: clock, RetryUnsafeWithIdempotency: test.unsafe,
			})
			if err != nil {
				t.Fatalf("construct retry middleware: %v", err)
			}
			middleware := []Middleware{retry}
			if test.idempotent {
				idempotency, keyErr := NewIdempotencyMiddleware(IdempotencyOptions{
					Name: "idempotency", Layer: MiddlewareEndpoint,
					Generator: IdentifierGeneratorFunc(func(context.Context) (GeneratedIdentifier, error) {
						return GeneratedIdentifier{Value: "stable-key", EntropyBits: 128}, nil
					}),
				})
				if keyErr != nil {
					t.Fatalf("construct idempotency middleware: %v", keyErr)
				}
				middleware = append(middleware, idempotency...)
			}
			attempts := 0
			temporary := errors.New("temporary")
			client, err := New(Config{
				Middleware: middleware,
				Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
					attempts++

					return nil, temporary
				}),
			})
			if err != nil {
				t.Fatalf("construct client: %v", err)
			}
			defer closeTestClient(t, client)
			request, err := http.NewRequest(test.method, "https://api.example.test", test.body)
			if err != nil {
				t.Fatalf("construct request: %v", err)
			}
			if test.contextKey {
				ctx, keyErr := WithIdempotencyKey(request.Context(), "context-only-key")
				if keyErr != nil {
					t.Fatalf("attach context key: %v", keyErr)
				}
				request = request.WithContext(ctx)
			}
			_, err = client.Do(request)
			if !errors.Is(err, temporary) {
				t.Fatalf("retry error = %#v, want the transport failure", err)
			}
			var exhausted *goretry.ExhaustedError
			if retried := errors.As(err, &exhausted); retried != (test.want > 1) {
				t.Fatalf("exhaustion = %v for %d attempts", retried, test.want)
			}
			if attempts != test.want {
				t.Fatalf("attempts = %d, want %d", attempts, test.want)
			}
		})
	}
}

func TestRetryUsesBoundedRetryAfterAndClosesDiscardedResponses(t *testing.T) {
	t.Parallel()

	clock := &retryTestClock{now: time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)}
	retry, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", Layer: MiddlewareEndpoint, MaximumAttempts: 3,
		BaseDelay: time.Second, MaximumDelay: 4 * time.Second,
		MaximumRetryAfter: 4 * time.Second, Clock: clock,
	})
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}
	closed := make([]bool, 2)
	attempts := 0
	client, err := New(Config{
		Middleware: []Middleware{retry},
		Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			if attempts <= 2 {
				index := attempts - 1
				header := "30"
				if attempts == 2 {
					header = clock.now.Add(3 * time.Second).Format(http.TimeFormat)
				}

				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": {header}},
					Body:       &retryCloseBody{Reader: strings.NewReader("retry"), closed: &closed[index]},
				}, nil
			}

			return &http.Response{StatusCode: http.StatusNoContent}, nil
		}),
	})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	_ = response.Body.Close()
	if got := clock.Delays(); len(got) != 2 || got[0] != 4*time.Second || got[1] != 3*time.Second {
		t.Fatalf("Retry-After delays = %v", got)
	}
	if !closed[0] || !closed[1] {
		t.Fatalf("discarded response closure = %v", closed)
	}
}

func TestRetryReplaysBodyAndIdentityThroughRealServer(t *testing.T) {
	t.Parallel()

	var bodies []string
	var keys []string
	var identities []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		content, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		bodies = append(bodies, string(content))
		keys = append(keys, request.Header.Get("Idempotency-Key"))
		identities = append(identities, request.Header.Get("X-Test-Operation"))
		if len(bodies) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)

			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	idempotency, err := NewIdempotencyMiddleware(IdempotencyOptions{
		Name: "idempotency", Layer: MiddlewareEndpoint,
		Generator: IdentifierGeneratorFunc(func(context.Context) (GeneratedIdentifier, error) {
			return GeneratedIdentifier{Value: "server-key", EntropyBits: 128}, nil
		}),
	})
	if err != nil {
		t.Fatalf("construct idempotency middleware: %v", err)
	}
	identityHeader := mustRequestMiddleware(t, MiddlewareOptions{
		Name: "identity-header", Scope: ScopeAttempt, Layer: MiddlewareClient, Priority: -1400,
	}, func(request *http.Request, next Next) (*http.Response, error) {
		identity, ok := OperationIdentityFromContext(request.Context())
		if !ok {
			t.Fatal("attempt has no operation identity")
		}
		request.Header.Set("X-Test-Operation", identity.ID)

		return next(request)
	})
	retry, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", Layer: MiddlewareEndpoint, MaximumAttempts: 2,
		BaseDelay: time.Millisecond, MaximumDelay: time.Millisecond,
		Clock:                      &retryTestClock{now: time.Unix(1_700_000_000, 0)},
		RetryUnsafeWithIdempotency: true,
	})
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}
	client, err := New(Config{Middleware: append([]Middleware{retry, identityHeader}, idempotency...)})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	defer closeTestClient(t, client)
	request, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("stable-body"))
	if err != nil {
		t.Fatalf("construct request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}
	if len(bodies) != 2 || bodies[0] != "stable-body" || bodies[1] != bodies[0] {
		t.Fatalf("request bodies = %v", bodies)
	}
	if keys[0] != "server-key" || keys[1] != keys[0] {
		t.Fatalf("idempotency keys = %v", keys)
	}
	if identities[0] == "" || identities[1] != identities[0] {
		t.Fatalf("operation identities = %v", identities)
	}
}

func TestRetryHonorsCancellationAndReportsSecretSafeExhaustion(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	secret := errors.New("network secret do-not-render")
	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0), wait: func(context.Context, time.Duration) error {
		cancel()

		return context.Canceled
	}}
	retry, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", Layer: MiddlewareEndpoint, MaximumAttempts: 3,
		BaseDelay: time.Second, MaximumDelay: time.Second, Clock: clock,
	})
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}
	client, err := New(Config{Middleware: []Middleware{retry}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
		return nil, secret
	})})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.test", nil)
	_, err = client.Do(request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if strings.Contains(err.Error(), "do-not-render") {
		t.Fatalf("retry error rendered cause: %q", err)
	}
}

func TestRetryRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	var typedNilClock *retryTestClock
	var typedNilRandom *retryTestRandom
	var typedNilPolicy *retryTestPolicy
	for _, options := range []RetryOptions{
		{},
		{Name: "retry", MaximumAttempts: 1},
		{Name: "retry", MaximumAttempts: maximumRetryAttempts + 1},
		{Name: "retry", MaximumAttempts: 2, BaseDelay: -1},
		{Name: "retry", MaximumAttempts: 2, Delays: []time.Duration{time.Second}, BaseDelay: time.Second},
		{Name: "retry", MaximumAttempts: 3, Delays: []time.Duration{time.Second}},
		{Name: "retry", MaximumAttempts: 2, Delays: []time.Duration{0}},
		{Name: "retry", MaximumAttempts: 2, Delays: []time.Duration{31 * time.Second}},
		{Name: "retry", MaximumAttempts: 2, BaseDelay: time.Second, MaximumDelay: time.Millisecond},
		{Name: "retry", MaximumAttempts: 2, MaximumDelay: 31 * time.Second},
		{Name: "retry", MaximumAttempts: 2, MaximumDelay: 2 * time.Second, MaximumRetryAfter: time.Second},
		{Name: "retry", MaximumAttempts: 2, BaseDelay: time.Second, MaximumDelay: time.Second, MaximumElapsed: -1},
		{Name: "retry", MaximumAttempts: 2, BaseDelay: time.Second, MaximumDelay: time.Second, MaximumRetryAfter: -1},
		{Name: "retry", MaximumAttempts: 2, BaseDelay: time.Second, MaximumDelay: time.Second, Clock: typedNilClock},
		{Name: "retry", MaximumAttempts: 2, BaseDelay: time.Second, MaximumDelay: time.Second, Random: typedNilRandom},
		{Name: "retry", MaximumAttempts: 2, BaseDelay: time.Second, MaximumDelay: time.Second, Policy: typedNilPolicy},
	} {
		if _, err := NewRetryMiddleware(options); !errors.Is(err, ErrInvalidRetryPolicy) {
			t.Fatalf("invalid options accepted: %#v, error %v", options, err)
		}
	}
	for _, options := range []RetryOptions{
		{Name: "retry", MaximumAttempts: 2},
		{Name: "retry", MaximumAttempts: maximumRetryAttempts},
		{Name: "retry", MaximumAttempts: 2, Delays: []time.Duration{time.Nanosecond}},
		{Name: "retry", MaximumAttempts: 2, Delays: []time.Duration{30 * time.Second}},
		{Name: "retry", MaximumAttempts: 2, MaximumDelay: 30 * time.Second},
	} {
		if _, err := NewRetryMiddleware(options); err != nil {
			t.Fatalf("valid options rejected: %#v, error %v", options, err)
		}
	}
	middleware, err := NewRetryMiddleware(RetryOptions{Name: "priority-retry", Priority: 7, MaximumAttempts: 2})
	if err != nil {
		t.Fatalf("construct priority retry: %v", err)
	}
	if middleware.information.Priority != -493 {
		t.Fatalf("retry middleware priority = %d", middleware.information.Priority)
	}
}

// TestRetryAfterSelectsDelayWithinCaps covers canonical Retry-After parsing and
// its combination with backoff: a hint can lengthen but not shorten the
// selected delay, every delay is capped by MaximumRetryAfter, and a value past
// the parser's 128-byte raw bound is treated as absent.
func TestRetryAfterSelectsDelayWithinCaps(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name                   string
		backoff, retryAfterCap time.Duration
		random                 goretry.Random
		header                 string
		want                   time.Duration
	}{
		{name: "beyond 31-bit seconds capped", backoff: 4 * time.Second, retryAfterCap: 4 * time.Second, random: zeroRetryRandom{}, header: "2147483648", want: 4 * time.Second},
		{name: "beyond uint64 capped", backoff: 4 * time.Second, retryAfterCap: 4 * time.Second, random: zeroRetryRandom{}, header: "18446744073709551616", want: 4 * time.Second},
		{name: "128-byte padded value parsed", backoff: 500 * time.Millisecond, retryAfterCap: 30 * time.Second, random: maxRetryRandom{}, header: strings.Repeat(" ", 127) + "1", want: time.Second},
		{name: "129-byte padded value absent", backoff: 500 * time.Millisecond, retryAfterCap: 30 * time.Second, random: maxRetryRandom{}, header: strings.Repeat(" ", 128) + "1", want: 500 * time.Millisecond},
		{name: "malformed value absent", backoff: 500 * time.Millisecond, retryAfterCap: 30 * time.Second, random: maxRetryRandom{}, header: "soon", want: 500 * time.Millisecond},
		{name: "longer hint honored above backoff cap", backoff: time.Second, retryAfterCap: 30 * time.Second, random: maxRetryRandom{}, header: "5", want: 5 * time.Second},
		{name: "hint capped by maximum retry delay", backoff: time.Second, retryAfterCap: 10 * time.Second, random: maxRetryRandom{}, header: "30", want: 10 * time.Second},
		{name: "zero does not shorten backoff", backoff: time.Second, retryAfterCap: 30 * time.Second, random: maxRetryRandom{}, header: "0", want: time.Second},
		{name: "past date does not shorten backoff", backoff: time.Second, retryAfterCap: 30 * time.Second, random: maxRetryRandom{}, header: now.Add(-time.Minute).Format(http.TimeFormat), want: time.Second},
		{name: "future date honored", backoff: time.Second, retryAfterCap: 30 * time.Second, random: maxRetryRandom{}, header: now.Add(3 * time.Second).Format(http.TimeFormat), want: 3 * time.Second},
		{name: "hint is not jittered", backoff: 7 * time.Second, retryAfterCap: 30 * time.Second, random: zeroRetryRandom{}, header: "3", want: 3 * time.Second},
		{name: "jitter applies to backoff", backoff: 7 * time.Second, retryAfterCap: 30 * time.Second, random: zeroRetryRandom{}, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			clock := &retryTestClock{now: now}
			closed := false
			response, calls, err := runRetry(t, RetryOptions{
				MaximumAttempts: 2, BaseDelay: test.backoff, MaximumDelay: test.backoff,
				MaximumRetryAfter: test.retryAfterCap, MaximumElapsed: time.Minute,
				Clock: clock, Random: test.random,
			}, func(attempt int) (*http.Response, error) {
				if attempt == 1 {
					return retryResponse(http.StatusServiceUnavailable, test.header, &closed), nil
				}
				return &http.Response{StatusCode: http.StatusNoContent}, nil
			})
			if err != nil || response.StatusCode != http.StatusNoContent || calls != 2 {
				t.Fatalf("response = %v, err = %v, calls = %d", response, err, calls)
			}
			if delays := clock.Delays(); !slices.Equal(delays, []time.Duration{test.want}) {
				t.Fatalf("delays = %v, want [%v]", delays, test.want)
			}
			if !closed {
				t.Fatal("discarded response was not closed before the wait")
			}
		})
	}
}

func TestRetryBackoffUsesCappedExponentialJitter(t *testing.T) {
	t.Parallel()

	failing := func(int) (*http.Response, error) {
		return retryResponse(http.StatusServiceUnavailable, "", new(bool)), nil
	}
	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	_, calls, err := runRetry(t, RetryOptions{
		MaximumAttempts: 5, BaseDelay: time.Second, MaximumDelay: 8 * time.Second,
		MaximumElapsed: time.Hour, Clock: clock, Random: maxRetryRandom{},
	}, failing)
	var exhausted *goretry.ExhaustedError
	if !errors.As(err, &exhausted) || calls != 5 || exhausted.Result().Attempts != 5 ||
		!slices.Equal(clock.Delays(), []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}) {
		t.Fatalf("exponential delays = %v, calls = %d, err = %v", clock.Delays(), calls, err)
	}

	clock = &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	_, calls, _ = runRetry(t, RetryOptions{
		MaximumAttempts: maximumRetryAttempts, BaseDelay: time.Second, MaximumDelay: 8 * time.Second,
		MaximumElapsed: time.Hour, Clock: clock, Random: maxRetryRandom{},
	}, failing)
	delays := clock.Delays()
	if calls != maximumRetryAttempts || len(delays) != maximumRetryAttempts-1 || slices.Max(delays) != 8*time.Second {
		t.Fatalf("maximum attempts = %d calls, %d delays, max %v", calls, len(delays), slices.Max(delays))
	}

	clock = &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	_, _, _ = runRetry(t, RetryOptions{
		MaximumAttempts: 5, BaseDelay: time.Second, MaximumDelay: 4 * time.Second,
		MaximumElapsed: time.Hour, Clock: clock, Random: halfRetryRandom{},
	}, failing)
	if delays := clock.Delays(); len(delays) != 4 || delays[3] > 2*time.Second+time.Nanosecond {
		t.Fatalf("jitter must apply after the cap: %v", delays)
	}

	clock = &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	_, _, _ = runRetry(t, RetryOptions{
		MaximumAttempts: 9, BaseDelay: time.Second, MaximumDelay: time.Second,
		MaximumElapsed: time.Hour, Clock: clock,
	}, failing)
	delays = clock.Delays()
	if len(delays) != 8 || slices.Max(delays) > time.Second || slices.Max(delays) == 0 || slices.Min(delays) < 0 {
		t.Fatalf("default random jitter delays = %v", delays)
	}

	clock = &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	_, _, _ = runRetry(t, RetryOptions{
		MaximumAttempts: 3, Delays: []time.Duration{250 * time.Millisecond, time.Second},
		Clock: clock, Random: zeroRetryRandom{},
	}, failing)
	if delays := clock.Delays(); !slices.Equal(delays, []time.Duration{250 * time.Millisecond, time.Second}) {
		t.Fatalf("explicit delays must not be jittered: %v", delays)
	}
}

func TestRetryAttemptLimitAndPolicyNumbering(t *testing.T) {
	t.Parallel()

	for _, maximum := range []int{2, 3, maximumRetryAttempts} {
		var seen []int
		clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
		_, calls, err := runRetry(t, RetryOptions{
			MaximumAttempts: maximum, MaximumElapsed: time.Hour, Clock: clock, Random: zeroRetryRandom{},
			Policy: RetryPolicyFunc(func(attempt RetryAttempt) bool {
				seen = append(seen, attempt.Attempt)
				return true
			}),
		}, func(int) (*http.Response, error) {
			return retryResponse(http.StatusServiceUnavailable, "", new(bool)), nil
		})
		var exhausted *goretry.ExhaustedError
		var status interface{ StatusCode() int }
		if !errors.As(err, &exhausted) || exhausted.Result().Attempts != uint(maximum) || calls != maximum ||
			len(clock.Delays()) != maximum-1 || len(seen) != maximum || seen[maximum-1] != maximum {
			t.Fatalf("maximum %d: calls %d, delays %d, seen %v, err %v", maximum, calls, len(clock.Delays()), seen, err)
		}
		if !errors.As(err, &status) || status.StatusCode() != http.StatusServiceUnavailable {
			t.Fatalf("last status is not reachable from %v", err)
		}
	}
}

func TestRetryLeavesIneligibleExchangesUnchanged(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		response bool
	}{{name: "response"}, {name: "transport failure"}} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			middleware, err := NewRetryMiddleware(RetryOptions{Name: "retry", MaximumAttempts: 3, Clock: &retryTestClock{now: time.Unix(1_700_000_000, 0)}})
			if err != nil {
				t.Fatal(err)
			}
			transportFailure := errors.New("connection lost after dispatch")
			calls := 0
			client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if test.name == "response" {
					return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("payload"))}, nil
				}
				return nil, transportFailure
			})})
			if err != nil {
				t.Fatal(err)
			}
			defer closeTestClient(t, client)
			request, _ := http.NewRequest(http.MethodPost, "https://api.example.test", strings.NewReader("body"))
			response, err := client.Do(request)
			if calls != 1 {
				t.Fatalf("unsafe POST calls = %d, want 1", calls)
			}
			if test.name == "response" {
				body, _ := io.ReadAll(response.Body)
				if err != nil || response.StatusCode != http.StatusServiceUnavailable || string(body) != "payload" {
					t.Fatalf("ineligible response = %v %q, err %v", response, body, err)
				}
				return
			}
			if !errors.Is(err, transportFailure) || response != nil {
				t.Fatalf("ineligible failure = %v, want the original transport failure", err)
			}
			if errors.As(err, new(*goretry.PermanentError)) || errors.As(err, new(*goretry.ExhaustedError)) {
				t.Fatalf("ineligible failure was wrapped as a retry outcome: %#v", err)
			}
			baseline, baselineErr := New(Config{Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
				return nil, transportFailure
			})})
			if baselineErr != nil {
				t.Fatal(baselineErr)
			}
			defer closeTestClient(t, baseline)
			plain, _ := http.NewRequest(http.MethodPost, "https://api.example.test", strings.NewReader("body"))
			_, want := baseline.Do(plain)
			if err.Error() != want.Error() {
				t.Fatalf("ineligible failure text = %q, want the unwrapped client's %q", err, want)
			}
		})
	}
}

func TestRetryCustomPolicyCannotReplayOneShotBody(t *testing.T) {
	t.Parallel()

	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	middleware, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", MaximumAttempts: 3, Clock: clock,
		Policy: RetryPolicyFunc(func(RetryAttempt) bool { return true }),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	closed := false
	client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return retryResponse(http.StatusServiceUnavailable, "", &closed), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequest(http.MethodPut, "https://api.example.test", &retryOneShotReader{Reader: strings.NewReader("body")})
	_, err = client.Do(request)
	var permanent *goretry.PermanentError
	var status interface{ StatusCode() int }
	if calls != 1 || len(clock.Delays()) != 0 || !errors.As(err, &permanent) || !closed ||
		!errors.As(err, &status) || status.StatusCode() != http.StatusServiceUnavailable {
		t.Fatalf("one-shot body: calls %d, delays %v, closed %t, err %v", calls, clock.Delays(), closed, err)
	}
}

func TestRetryNestedRetryableFailureCannotReplayOneShotBody(t *testing.T) {
	t.Parallel()

	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	middleware, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", MaximumAttempts: 3, Clock: clock,
		Policy: RetryPolicyFunc(func(RetryAttempt) bool { return true }),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, goretry.Retryable(errors.New("inner retry failure"))
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequest(http.MethodPut, "https://api.example.test", &retryOneShotReader{Reader: strings.NewReader("body")})
	_, err = client.Do(request)
	if calls != 1 || len(clock.Delays()) != 0 || !errors.As(err, new(*goretry.PermanentError)) || errors.As(err, new(*MiddlewarePanicError)) {
		t.Fatalf("nested retryable failure: calls %d, delays %v, err %v", calls, clock.Delays(), err)
	}
}

func TestRetryStopsOnMissingExchangeResult(t *testing.T) {
	t.Parallel()

	middleware, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", MaximumAttempts: 3, Clock: &retryTestClock{now: time.Unix(1_700_000_000, 0)},
		Policy: RetryPolicyFunc(func(RetryAttempt) bool { return true }),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	_, err = middleware.around(httptest.NewRequest(http.MethodGet, "https://api.example.test", nil), func(*http.Request) (*http.Response, error) {
		calls++
		return nil, nil
	})
	if calls != 1 || !errors.Is(err, ErrInvalidMiddlewareResult) || !errors.As(err, new(*goretry.PermanentError)) {
		t.Fatalf("missing result: calls %d, err %v", calls, err)
	}
}

func TestRetryReplayFailureKeepsLastStatus(t *testing.T) {
	t.Parallel()

	replayFailure := errors.New("replay failure")
	middleware, err := NewRetryMiddleware(RetryOptions{Name: "retry", MaximumAttempts: 3, Clock: &retryTestClock{now: time.Unix(1_700_000_000, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return retryResponse(http.StatusServiceUnavailable, "", new(bool)), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequest(http.MethodPut, "https://api.example.test", strings.NewReader("body"))
	request.GetBody = func() (io.ReadCloser, error) { return nil, replayFailure }
	_, err = client.Do(request)
	var opened *BodyOpenError
	var status interface{ StatusCode() int }
	if calls != 1 || !errors.As(err, &opened) || !errors.Is(err, replayFailure) ||
		!errors.As(err, &status) || status.StatusCode() != http.StatusServiceUnavailable {
		t.Fatalf("replay stop: calls %d, err %v", calls, err)
	}
}

// TestRetryElapsedBudgetDoesNotCutSlowExchanges uses wall time: a slow
// exchange that crosses MaximumElapsed completes, is recorded, and the budget
// then stops further retries with that exchange's status in history.
func TestRetryElapsedBudgetDoesNotCutSlowExchanges(t *testing.T) {
	t.Parallel()

	middleware, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", MaximumAttempts: 5, MaximumElapsed: 300 * time.Millisecond,
		BaseDelay: time.Millisecond, MaximumDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return retryResponse(http.StatusTooManyRequests, "", new(bool)), nil
		}
		time.Sleep(600 * time.Millisecond)
		return retryResponse(http.StatusServiceUnavailable, "", new(bool)), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
	_, err = client.Do(request)
	var budget *goretry.BudgetError
	if calls != 2 || !errors.As(err, &budget) || budget.Kind != goretry.BudgetElapsed {
		t.Fatalf("slow exchange: calls %d, err %v", calls, err)
	}
	var status interface{ StatusCode() int }
	history := budget.Result().History
	if len(history) != 1 || !errors.As(history[0].Err, &status) || status.StatusCode() != http.StatusServiceUnavailable {
		t.Fatalf("history = %+v, want the slow 503 exchange", history)
	}
}

func TestRetryTerminalErrorsReportOutcomeAndStatus(t *testing.T) {
	t.Parallel()

	closeFailure := errors.New("close failure")
	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	_, _, err := runRetry(t, RetryOptions{MaximumAttempts: 3, Clock: clock}, func(int) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Body: retryFailingBody{closeErr: closeFailure}}, nil
	})
	var status interface{ StatusCode() int }
	if !errors.As(err, new(*goretry.PermanentError)) || !errors.As(err, &status) || status.StatusCode() != http.StatusTooManyRequests {
		t.Fatalf("close failure status = %v", err)
	}

	observer := &telemetryRecordingBeginObserver{}
	middleware, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", MaximumAttempts: 5, MaximumElapsed: 10 * time.Second,
		Clock: &retryTestClock{now: time.Unix(1_700_000_000, 0)}, Random: zeroRetryRandom{},
	})
	if err != nil {
		t.Fatal(err)
	}
	exchanges := 0
	client, err := New(Config{
		Middleware: []Middleware{middleware}, Telemetry: &TelemetryOptions{BeginObserver: observer},
		Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
			exchanges++
			return retryResponse(http.StatusServiceUnavailable, "5", new(bool)), nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
	_, err = client.Do(request)
	var budget *goretry.BudgetError
	if !errors.As(err, &budget) || budget.Kind != goretry.BudgetElapsed {
		t.Fatalf("budget stop = %v", err)
	}
	// A budget found exhausted after a completed wait carries the deadline cause;
	// callers must test BudgetError before context.DeadlineExceeded.
	if !errors.Is(err, context.DeadlineExceeded) || exchanges != 2 {
		t.Fatalf("post-wait budget stop = %v after %d exchanges, want DeadlineExceeded identity after 2", err, exchanges)
	}
	history := budget.Result().History
	if len(history) == 0 || !errors.As(history[len(history)-1].Err, &status) || status.StatusCode() != http.StatusServiceUnavailable {
		t.Fatalf("last status is not in the budget history: %+v", history)
	}
	last := observer.events[len(observer.events)-1]
	if last.Scope != TelemetryOperation || last.Outcome != TelemetryOutcomeRetryFailure {
		t.Fatalf("operation outcome = %+v, want %s", last, TelemetryOutcomeRetryFailure)
	}
}

func TestRetryReplayFailuresStopWithoutAnotherExchange(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		getBody func() (io.ReadCloser, error)
		want    error
	}{
		{name: "open failure", getBody: func() (io.ReadCloser, error) { return nil, errors.New("replay failure") }},
		{name: "nil body", getBody: func() (io.ReadCloser, error) { return nil, nil }, want: ErrInvalidBody},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			middleware, err := NewRetryMiddleware(RetryOptions{Name: "retry", MaximumAttempts: 3, Clock: &retryTestClock{now: time.Unix(1_700_000_000, 0)}})
			if err != nil {
				t.Fatal(err)
			}
			calls, opens := 0, 0
			client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, errors.New("temporary")
			})})
			if err != nil {
				t.Fatal(err)
			}
			defer closeTestClient(t, client)
			request, _ := http.NewRequest(http.MethodPut, "https://api.example.test", strings.NewReader("body"))
			request.GetBody = func() (io.ReadCloser, error) {
				opens++
				return test.getBody()
			}
			_, err = client.Do(request)
			var opened *BodyOpenError
			if calls != 1 || opens != 1 || !errors.As(err, &opened) || test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("calls %d opens %d err %v", calls, opens, err)
			}
		})
	}

	t.Run("body opener removed by inner middleware", func(t *testing.T) {
		t.Parallel()
		middleware, err := NewRetryMiddleware(RetryOptions{Name: "retry", MaximumAttempts: 3, Clock: &retryTestClock{now: time.Unix(1_700_000_000, 0)}})
		if err != nil {
			t.Fatal(err)
		}
		dropOpener, err := NewTransportMiddleware(MiddlewareOptions{
			Name: "drop-opener", Scope: ScopeOperation, Layer: MiddlewareClient, Priority: -400,
		}, func(request *http.Request, next Next) (*http.Response, error) {
			request.GetBody = nil

			return next(request)
		})
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		client, err := New(Config{Middleware: []Middleware{middleware, dropOpener}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("temporary")
		})})
		if err != nil {
			t.Fatal(err)
		}
		defer closeTestClient(t, client)
		request, _ := http.NewRequest(http.MethodPut, "https://api.example.test", strings.NewReader("body"))
		_, err = client.Do(request)
		var opened *BodyOpenError
		var panicked *MiddlewarePanicError
		if calls != 1 || !errors.As(err, &opened) || !errors.Is(err, ErrInvalidBody) || errors.As(err, &panicked) {
			t.Fatalf("calls %d err %v", calls, err)
		}
	})
}

func TestRetryDrainsAndClosesBeforeWaiting(t *testing.T) {
	t.Parallel()

	body := &retryCountingBody{reader: bytes.NewReader(make([]byte, 70_000))}
	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	clock.wait = func(context.Context, time.Duration) error {
		if body.read != 65_537 || !body.closed {
			t.Errorf("body state at wait = %d read, closed %t", body.read, body.closed)
		}
		return nil
	}
	_, calls, err := runRetry(t, RetryOptions{MaximumAttempts: 2, Clock: clock, Random: zeroRetryRandom{}}, func(attempt int) (*http.Response, error) {
		if attempt == 1 {
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: body}, nil
		}
		return &http.Response{StatusCode: http.StatusNoContent}, nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("drain retry err = %v, calls = %d", err, calls)
	}

	closeFailure := errors.New("close failure")
	clock = &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	_, calls, err = runRetry(t, RetryOptions{MaximumAttempts: 3, Clock: clock}, func(int) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: retryFailingBody{closeErr: closeFailure}}, nil
	})
	if !errors.Is(err, closeFailure) || calls != 1 || len(clock.Delays()) != 0 {
		t.Fatalf("close failure = %v, calls %d, delays %v", err, calls, clock.Delays())
	}

	closed := false
	response, _, err := runRetry(t, RetryOptions{MaximumAttempts: 2, Clock: &retryTestClock{now: time.Unix(1_700_000_000, 0)}}, func(int) (*http.Response, error) {
		return retryResponse(http.StatusServiceUnavailable, "", &closed), nil
	})
	var exhausted *goretry.ExhaustedError
	if response != nil || !errors.As(err, &exhausted) || !closed {
		t.Fatalf("final exhausted response = %v, err %v, closed %t", response, err, closed)
	}
}

func TestRetryBackwardClockStopsBeforeAnotherExchange(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		duringWait bool
		wantDelays int
	}{
		{name: "during exchange", wantDelays: 0},
		{name: "during wait", duringWait: true, wantDelays: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
			stepBack := func() {
				clock.mu.Lock()
				clock.now = clock.now.Add(-time.Hour)
				clock.mu.Unlock()
			}
			if test.duringWait {
				clock.wait = func(context.Context, time.Duration) error { stepBack(); return nil }
			}
			_, calls, err := runRetry(t, RetryOptions{MaximumAttempts: 5, Clock: clock, Random: zeroRetryRandom{}}, func(int) (*http.Response, error) {
				if !test.duringWait {
					stepBack()
				}
				return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
			})
			var permanent *goretry.PermanentError
			var status interface{ StatusCode() int }
			if calls != 1 || len(clock.Delays()) != test.wantDelays || !errors.As(err, &permanent) || !errors.Is(err, ErrRetryClockRegressed) ||
				!errors.As(err, &status) || status.StatusCode() != http.StatusServiceUnavailable {
				t.Fatalf("backward clock = %v, calls %d, delays %v", err, calls, clock.Delays())
			}
			if outcome := telemetryOutcome(nil, err); outcome != TelemetryOutcomeRetryFailure {
				t.Fatalf("backward clock telemetry = %q", outcome)
			}
		})
	}
}

func TestRetryDefaultMaximumDelayFollowsLowerRetryAfterCap(t *testing.T) {
	t.Parallel()

	for _, maximumRetryAfter := range []time.Duration{500 * time.Millisecond, 50 * time.Millisecond} {
		clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
		_, calls, err := runRetry(t, RetryOptions{MaximumAttempts: 3, MaximumRetryAfter: maximumRetryAfter, Clock: clock, Random: maxRetryRandom{}}, func(int) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
		})
		var exhausted *goretry.ExhaustedError
		if calls != 3 || !errors.As(err, &exhausted) {
			t.Fatalf("cap %v = %v, calls %d", maximumRetryAfter, err, calls)
		}
		for _, delay := range clock.Delays() {
			if delay > maximumRetryAfter {
				t.Fatalf("delay %v exceeds the lowered cap %v", delay, maximumRetryAfter)
			}
		}
	}
}

func TestRetryCancellationDuringExchangeReturnsOriginalFailure(t *testing.T) {
	t.Parallel()

	middleware, err := NewRetryMiddleware(RetryOptions{Name: "retry", MaximumAttempts: 3, Clock: &retryTestClock{now: time.Unix(1_700_000_000, 0)}, Random: zeroRetryRandom{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	transportFailure := fmt.Errorf("dial: %w", context.Canceled)
	calls := 0
	client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
		}
		cancel()

		return nil, transportFailure
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.test", nil)
	_, err = client.Do(request)
	var contextErr *goretry.ContextError
	if calls != 2 || !errors.Is(err, context.Canceled) || errors.As(err, &contextErr) {
		t.Fatalf("cancellation during exchange = %v, calls %d", err, calls)
	}
}

func TestRetryClockWaitFailureStopsPermanently(t *testing.T) {
	t.Parallel()

	waitFailure := errors.New("scheduler stopped")
	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	clock.wait = func(context.Context, time.Duration) error { return waitFailure }
	_, calls, err := runRetry(t, RetryOptions{MaximumAttempts: 3, Clock: clock, Random: zeroRetryRandom{}}, func(int) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
	})
	var permanent *goretry.PermanentError
	var status interface{ StatusCode() int }
	if calls != 1 || !errors.As(err, &permanent) || !errors.Is(err, waitFailure) || errors.As(err, &status) {
		t.Fatalf("wait failure = %v, calls %d", err, calls)
	}
	if outcome := telemetryOutcome(nil, err); outcome != TelemetryOutcomeFailure {
		t.Fatalf("wait failure telemetry = %q", outcome)
	}
}

func TestRetryElapsedBudgetUsesDefaultsAndStopsBeforeWaiting(t *testing.T) {
	t.Parallel()

	secret := "network secret do-not-render"
	for _, test := range []struct {
		name        string
		options     RetryOptions
		header      string
		wantCalls   int
		wantDelays  []time.Duration
		wantElapsed bool
	}{
		{name: "hint beyond budget", options: RetryOptions{MaximumAttempts: 3, MaximumElapsed: 10 * time.Second}, header: "30", wantCalls: 1, wantElapsed: true},
		{name: "budget exhausted after waits", options: RetryOptions{MaximumAttempts: 5, MaximumElapsed: 10 * time.Second}, header: "4", wantCalls: 3, wantDelays: []time.Duration{4 * time.Second, 4 * time.Second}, wantElapsed: true},
		{name: "default elapsed budget", options: RetryOptions{MaximumAttempts: 3}, header: "11", wantCalls: 1, wantElapsed: true},
		{name: "default retry-after cap", options: RetryOptions{MaximumAttempts: 2, MaximumElapsed: time.Minute}, header: "31", wantCalls: 2, wantDelays: []time.Duration{30 * time.Second}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
			options := test.options
			options.Clock, options.Random = clock, zeroRetryRandom{}
			closed := false
			_, calls, err := runRetry(t, options, func(int) (*http.Response, error) {
				return retryResponse(http.StatusServiceUnavailable, test.header, &closed), nil
			})
			var budget *goretry.BudgetError
			// A budget found exhausted before a wait carries the operation error,
			// not a deadline cause, under the default policy.
			if calls != test.wantCalls || !slices.Equal(clock.Delays(), test.wantDelays) || errors.As(err, &budget) != test.wantElapsed || !closed ||
				errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("calls %d delays %v err %v closed %t", calls, clock.Delays(), err, closed)
			}
			if test.wantElapsed && budget.Kind != goretry.BudgetElapsed {
				t.Fatalf("budget kind = %v", budget.Kind)
			}
			if err != nil && strings.Contains(err.Error(), secret) {
				t.Fatalf("retry error rendered a cause: %q", err)
			}
		})
	}
}

func TestRetryReturnedBodyOutlivesExecutorAttempt(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("x"), 128<<10)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		writer.(http.Flusher).Flush()
		time.Sleep(150 * time.Millisecond)
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	middleware, err := NewRetryMiddleware(RetryOptions{Name: "retry", MaximumAttempts: 2, MaximumElapsed: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	client, err := New(Config{Middleware: []Middleware{middleware}})
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestClient(t, client)

	request, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("streamed request: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || len(body) != len(payload) {
		t.Fatalf("streamed body = %d bytes, err %v", len(body), err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	request, _ = http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	response, err = client.Do(request)
	if err != nil {
		t.Fatalf("cancelable request: %v", err)
	}
	time.AfterFunc(20*time.Millisecond, cancel)
	started := time.Now()
	_, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("caller cancellation did not stop the body read: %v after %v", err, time.Since(started))
	}
}

func TestRetryCancellationStopsStalledDrainAndWait(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	defer close(release)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/hint" {
			writer.Header().Set("Retry-After", "25")
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusServiceUnavailable)
		writer.(http.Flusher).Flush()
		select {
		case <-release:
		case <-request.Context().Done():
		}
	}))
	defer server.Close()
	for _, path := range []string{"/stalled", "/hint"} {
		t.Run(path, func(t *testing.T) {
			middleware, err := NewRetryMiddleware(RetryOptions{Name: "retry", MaximumAttempts: 3, MaximumElapsed: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				return server.Client().Transport.RoundTrip(request)
			})})
			if err != nil {
				t.Fatal(err)
			}
			defer closeTestClient(t, client)
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(50*time.Millisecond, cancel)
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
			started := time.Now()
			_, err = client.Do(request)
			if !errors.Is(err, context.Canceled) || calls != 1 || time.Since(started) > time.Second {
				t.Fatalf("cancellation = %v after %v with %d calls", err, time.Since(started), calls)
			}
			// A cancelled drain is a body-lifecycle stop that keeps the status;
			// a cancelled wait is a context stop.
			var status interface{ StatusCode() int }
			if path == "/stalled" && (!errors.As(err, new(*goretry.PermanentError)) || !errors.As(err, &status) || status.StatusCode() != http.StatusServiceUnavailable) ||
				path == "/hint" && !errors.As(err, new(*goretry.ContextError)) {
				t.Fatalf("%s cancellation error = %v", path, err)
			}
		})
	}
}

func TestRetryConcurrentRequestsKeepTheirOwnHints(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	recorded := map[int][]time.Duration{}
	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	clock.wait = func(ctx context.Context, delay time.Duration) error {
		mu.Lock()
		defer mu.Unlock()
		index, _ := ctx.Value(retryRequestKey{}).(int)
		recorded[index] = append(recorded[index], delay)
		return nil
	}
	middleware, err := NewRetryMiddleware(RetryOptions{Name: "retry", MaximumAttempts: 2, MaximumElapsed: time.Hour, BaseDelay: time.Millisecond, MaximumDelay: time.Millisecond, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	var attempts sync.Map
	client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(request *http.Request) (*http.Response, error) {
		index := request.Context().Value(retryRequestKey{}).(int)
		if _, loaded := attempts.LoadOrStore(index, true); loaded {
			return &http.Response{StatusCode: http.StatusNoContent}, nil
		}
		return retryResponse(http.StatusServiceUnavailable, strconv.Itoa(index+1), new(bool)), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestClient(t, client)
	var group sync.WaitGroup
	for index := range 8 {
		group.Go(func() {
			ctx := context.WithValue(context.Background(), retryRequestKey{}, index)
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.test", nil)
			if _, err := client.Do(request); err != nil {
				t.Errorf("request %d: %v", index, err)
			}
		})
	}
	group.Wait()
	for index := range 8 {
		if want := []time.Duration{time.Duration(index+1) * time.Second}; !slices.Equal(recorded[index], want) {
			t.Fatalf("request %d delays = %v, want %v", index, recorded[index], want)
		}
	}
}

func TestDefaultRetryPolicyEligibility(t *testing.T) {
	t.Parallel()

	called := false
	policy := RetryPolicyFunc(func(attempt RetryAttempt) bool {
		called = attempt.Attempt == 7

		return true
	})
	if !policy.ShouldRetry(RetryAttempt{Attempt: 7}) || !called {
		t.Fatal("RetryPolicyFunc did not delegate")
	}
	defaultPolicy := defaultRetryPolicy{}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	canceledRequest, _ := http.NewRequestWithContext(canceled, http.MethodGet, "https://api.example.test", nil)
	request, _ := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
	for _, attempt := range []RetryAttempt{
		{Request: canceledRequest, BodyReplayable: true, Failure: errors.New("failure")},
		{Request: request, BodyReplayable: true},
		{Request: request, BodyReplayable: true, Failure: context.DeadlineExceeded},
		{Request: request, Failure: errors.New("failure")},
		{BodyReplayable: true, Failure: errors.New("failure")},
	} {
		if defaultPolicy.ShouldRetry(attempt) {
			t.Fatalf("ineligible retry attempt accepted: %#v", attempt)
		}
	}
}

func TestSystemRetryClockWaits(t *testing.T) {
	t.Parallel()

	clock := systemRetryClock{}
	if clock.Now().IsZero() {
		t.Fatal("system clock returned zero time")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := clock.Wait(canceled, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled wait = %v", err)
	}
	if err := clock.Wait(context.Background(), time.Nanosecond); err != nil {
		t.Fatalf("completed wait = %v", err)
	}
	during, cancelDuring := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond, cancelDuring)
	if err := clock.Wait(during, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wait = %v", err)
	}
}

func TestRetryDrainUsesExactBound(t *testing.T) {
	t.Parallel()

	body := &retryCountingBody{reader: bytes.NewReader(make([]byte, 70_000))}
	if err := drainAndCloseRetryResponse(&http.Response{Body: body}); err != nil {
		t.Fatalf("drain retry response: %v", err)
	}
	if body.read != 65_537 || !body.closed {
		t.Fatalf("retry response drain = %d bytes, closed %t", body.read, body.closed)
	}
}

func TestRetryContainsPolicyPanicAndClosesResponse(t *testing.T) {
	t.Parallel()

	retry, err := NewRetryMiddleware(RetryOptions{
		Name: "retry", Layer: MiddlewareEndpoint, MaximumAttempts: 2,
		Policy: RetryPolicyFunc(func(RetryAttempt) bool { panic("policy secret") }),
	})
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}
	closed := false
	client, err := New(Config{
		Middleware: []Middleware{retry},
		Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       &retryCloseBody{Reader: strings.NewReader("body"), closed: &closed},
			}, nil
		}),
	})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	defer closeTestClient(t, client)
	request, _ := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
	_, err = client.Do(request)
	var panicError *MiddlewarePanicError
	if !errors.As(err, &panicError) || !closed {
		t.Fatalf("policy panic result = %#v, body closed = %v", err, closed)
	}
	if strings.Contains(err.Error(), "policy secret") {
		t.Fatalf("policy panic rendered value: %q", err)
	}
}

type retryTestClock struct {
	mu     sync.Mutex
	now    time.Time
	delays []time.Duration
	wait   func(context.Context, time.Duration) error
}

func (clock *retryTestClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()

	return clock.now
}

func (clock *retryTestClock) Wait(ctx context.Context, delay time.Duration) error {
	clock.mu.Lock()
	clock.delays = append(clock.delays, delay)
	clock.now = clock.now.Add(delay)
	wait := clock.wait
	clock.mu.Unlock()
	if wait != nil {
		return wait(ctx, delay)
	}

	return nil
}

func (clock *retryTestClock) Delays() []time.Duration {
	clock.mu.Lock()
	defer clock.mu.Unlock()

	return append([]time.Duration(nil), clock.delays...)
}

type retryTestRandom struct{}

func (*retryTestRandom) Int64n(int64) int64 { return 0 }

// maxRetryRandom selects the upper jitter bound so exponential delays are exact.
type maxRetryRandom struct{}

func (maxRetryRandom) Int64n(upper int64) int64 { return upper - 1 }

// halfRetryRandom selects the midpoint of the jitter range.
type halfRetryRandom struct{}

func (halfRetryRandom) Int64n(upper int64) int64 { return upper / 2 }

type retryRequestKey struct{}

// retryResponse builds a retryable response whose body records closure.
func retryResponse(status int, retryAfter string, closed *bool) *http.Response {
	header := make(http.Header)
	if retryAfter != "" {
		header.Set("Retry-After", retryAfter)
	}
	return &http.Response{StatusCode: status, Header: header, Body: &retryCloseBody{Reader: strings.NewReader("retry"), closed: closed}}
}

// runRetry executes one GET through the retry middleware and reports the
// physical exchange count.
func runRetry(t *testing.T, options RetryOptions, transport func(attempt int) (*http.Response, error)) (*http.Response, int, error) {
	t.Helper()
	if options.Name == "" {
		options.Name = "retry"
	}
	middleware, err := NewRetryMiddleware(options)
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}
	calls := 0
	client, err := New(Config{Middleware: []Middleware{middleware}, Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return transport(calls)
	})})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	t.Cleanup(func() { closeTestClient(t, client) })
	request, _ := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
	response, err := client.Do(request)

	return response, calls, err
}

type retryTestPolicy struct{}

func (*retryTestPolicy) ShouldRetry(RetryAttempt) bool { return false }

type retryOneShotReader struct{ io.Reader }

type retryCloseBody struct {
	io.Reader
	closed *bool
}

func (body *retryCloseBody) Close() error {
	*body.closed = true

	return nil
}

type retryFailingBody struct{ closeErr error }

func (retryFailingBody) Read([]byte) (int, error) { return 0, io.EOF }
func (body retryFailingBody) Close() error        { return body.closeErr }

type retryCountingBody struct {
	reader *bytes.Reader
	read   int
	closed bool
}

func (body *retryCountingBody) Read(buffer []byte) (int, error) {
	read, err := body.reader.Read(buffer)
	body.read += read
	return read, err
}

func (body *retryCountingBody) Close() error {
	body.closed = true
	return nil
}

func closeTestClient(t *testing.T, client *Client) {
	t.Helper()
	if err := client.Close(); err != nil {
		t.Errorf("close client: %v", err)
	}
}
