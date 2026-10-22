package httpclient

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	retry "github.com/faustbrian/go-retry/v3"
	retryhttp "github.com/faustbrian/go-retry/v3/adapters/http"
)

// ErrInvalidRetryPolicy indicates invalid retry configuration or behavior.
var ErrInvalidRetryPolicy = errors.New("invalid HTTP retry policy")

const (
	defaultRetryBaseDelay      = 100 * time.Millisecond
	defaultRetryMaximumDelay   = 2 * time.Second
	defaultRetryMaximumElapsed = 10 * time.Second
	defaultMaximumRetryAfter   = 30 * time.Second
	maximumRetryAttempts       = 100
	retryDrainBytes            = 64 << 10
	retryMiddlewarePriority    = -500
)

// RetryClock supplies deterministic time and context-aware waits.
// Implementations must be safe for concurrent use.
type RetryClock interface {
	// Now returns monotonic-compatible observation time; callbacks may call it while holding a mutex.
	Now() time.Time
	// Wait waits for a duration or cancellation; it must return when context is canceled.
	Wait(context.Context, time.Duration) error
}

// RetryAttempt describes one completed physical exchange to custom policy.
// Response and Failure are mutually exclusive.
type RetryAttempt struct {
	// Request is a metadata snapshot without a usable Body or GetBody.
	Request *http.Request
	// Response is a metadata snapshot without a usable body.
	Response *http.Response
	// Failure is the attempt error.
	Failure error
	// Attempt is the one-based attempt count.
	Attempt int
	// BodyReplayable reports whether another independent body can be opened.
	BodyReplayable bool
	// HasIdempotency reports package-applied key proof, not merely a header's presence.
	HasIdempotency bool
}

// RetryPolicy classifies completed physical exchanges. It must be safe for
// concurrent use and must not consume or close response bodies.
type RetryPolicy interface {
	// ShouldRetry selects retry eligibility; custom policy does not bypass body replay or
	// attempt-count checks.
	ShouldRetry(RetryAttempt) bool
}

// RetryPolicyFunc adapts a function to RetryPolicy.
type RetryPolicyFunc func(RetryAttempt) bool

// ShouldRetry implements RetryPolicy.
func (function RetryPolicyFunc) ShouldRetry(attempt RetryAttempt) bool {
	return function(attempt)
}

// RetryOptions configures bounded operation retry middleware. Scheduling,
// budgets and terminal errors are owned by github.com/faustbrian/go-retry/v3;
// this middleware owns HTTP eligibility, request replay and response cleanup.
type RetryOptions struct {
	// Name names the middleware registration; use a stable nonsecret lowercase identifier.
	Name string
	// Layer selects registration precedence; higher layers replace the same name, scope and stage.
	Layer MiddlewareLayer
	// Priority orders the registration within its stage; zero selects the helper default where
	// provided.
	Priority int
	// MaximumAttempts is required in [2,100], including the initial attempt.
	MaximumAttempts int
	// MaximumElapsed is checked between exchanges and delays, not a deadline for Next; zero
	// selects ten seconds.
	MaximumElapsed time.Duration
	// Delays selects one explicit delay after each failed attempt. When set,
	// it must contain MaximumAttempts minus one positive entries no larger than
	// MaximumRetryAfter and cannot be combined with BaseDelay or MaximumDelay.
	// Explicit delays are not jittered.
	Delays []time.Duration
	// BaseDelay starts exponential backoff; zero selects 100 milliseconds.
	BaseDelay time.Duration
	// MaximumDelay caps exponential delay before jitter; zero selects two
	// seconds. It must not exceed MaximumRetryAfter.
	MaximumDelay time.Duration
	// MaximumRetryAfter caps every selected delay, including a server
	// Retry-After hint; zero selects 30 seconds.
	MaximumRetryAfter time.Duration
	// RetryUnsafeWithIdempotency opts unsafe methods into default policy only with
	// package-applied key proof.
	RetryUnsafeWithIdempotency bool
	// Clock is borrowed; Now must be nonblocking and Wait must honor cancellation.
	Clock RetryClock
	// Random is borrowed, shared by every request through the middleware and
	// must be safe for concurrent use. It jitters exponential delays; nil
	// selects a per-middleware source seeded from crypto/rand.
	Random retry.Random
	// Policy is borrowed; nil uses the package's method, status and error policy.
	Policy RetryPolicy
}

type resolvedRetryOptions struct {
	policy *retry.Policy
	retry  RetryPolicy
}

// NewRetryMiddleware creates operation-scoped transport middleware. The
// middleware retries only replayable requests accepted by endpoint policy.
func NewRetryMiddleware(options RetryOptions) (Middleware, error) {
	resolved, err := resolveRetryOptions(options)
	if err != nil {
		return Middleware{}, err
	}

	return NewTransportMiddleware(MiddlewareOptions{
		Name: options.Name, Scope: ScopeOperation, Layer: options.Layer,
		Priority: retryMiddlewarePriority + options.Priority,
	}, func(request *http.Request, next Next) (*http.Response, error) {
		return executeRetry(request, next, resolved)
	})
}

func resolveRetryOptions(options RetryOptions) (resolvedRetryOptions, error) {
	if options.MaximumAttempts < 2 || options.MaximumAttempts > maximumRetryAttempts {
		return resolvedRetryOptions{}, fmt.Errorf("%w: maximum attempts must be between 2 and 100", ErrInvalidRetryPolicy)
	}
	if options.MaximumElapsed < 0 || options.MaximumRetryAfter < 0 {
		return resolvedRetryOptions{}, fmt.Errorf("%w: retry duration bounds are invalid", ErrInvalidRetryPolicy)
	}
	maximumElapsed := options.MaximumElapsed
	if maximumElapsed == 0 {
		maximumElapsed = defaultRetryMaximumElapsed
	}
	maximumRetryAfter := options.MaximumRetryAfter
	if maximumRetryAfter == 0 {
		maximumRetryAfter = defaultMaximumRetryAfter
	}
	backoff, err := resolveRetryBackoff(options, maximumRetryAfter)
	if err != nil {
		return resolvedRetryOptions{}, err
	}
	clock := options.Clock
	if clock == nil {
		clock = systemRetryClock{}
	} else if nilLike(clock) {
		return resolvedRetryOptions{}, fmt.Errorf("%w: clock is nil", ErrInvalidRetryPolicy)
	}
	random := options.Random
	if random == nil {
		random, err = newRetryRandom()
		if err != nil {
			return resolvedRetryOptions{}, err
		}
	} else if nilLike(random) {
		return resolvedRetryOptions{}, fmt.Errorf("%w: random is nil", ErrInvalidRetryPolicy)
	}
	policy := options.Policy
	if policy == nil {
		policy = defaultRetryPolicy{unsafeIdempotency: options.RetryUnsafeWithIdempotency}
	} else if nilLike(policy) {
		return resolvedRetryOptions{}, fmt.Errorf("%w: policy is nil", ErrInvalidRetryPolicy)
	}
	scheduler := retryPolicyClock{clock: clock}
	executor, err := retry.NewPolicy(retry.Config{
		Backoff: backoff, MaxAttempts: uint(options.MaximumAttempts),
		MaxElapsed: maximumElapsed, MaxDelay: maximumRetryAfter,
		Clock: scheduler, Sleeper: scheduler, Random: random,
		Classifier: retry.ClassifyFunc(classifyRetryOutcome),
		// Budget and cancellation stops carry only their context cause; keeping
		// the last attempt lets callers read the final HTTP status from history.
		HistoryLimit: 1,
	})
	if err != nil {
		return resolvedRetryOptions{}, fmt.Errorf("%w: %w", ErrInvalidRetryPolicy, err)
	}

	return resolvedRetryOptions{policy: executor, retry: policy}, nil
}

func resolveRetryBackoff(options RetryOptions, maximumRetryAfter time.Duration) (retry.Backoff, error) {
	if len(options.Delays) > 0 {
		if len(options.Delays) != options.MaximumAttempts-1 || options.BaseDelay != 0 || options.MaximumDelay != 0 {
			return nil, fmt.Errorf("%w: explicit delays must match attempts and cannot use exponential bounds", ErrInvalidRetryPolicy)
		}
		delays := append(explicitRetryBackoff(nil), options.Delays...)
		for _, delay := range delays {
			if delay <= 0 || delay > maximumRetryAfter {
				return nil, fmt.Errorf("%w: explicit delays must be positive and within the maximum retry delay", ErrInvalidRetryPolicy)
			}
		}

		return delays, nil
	}
	baseDelay := options.BaseDelay
	if baseDelay == 0 {
		baseDelay = defaultRetryBaseDelay
	}
	maximumDelay := options.MaximumDelay
	if maximumDelay == 0 {
		maximumDelay = defaultRetryMaximumDelay
	}
	if baseDelay < 0 || maximumDelay < baseDelay || maximumDelay > maximumRetryAfter {
		return nil, fmt.Errorf("%w: retry duration bounds are invalid", ErrInvalidRetryPolicy)
	}

	return retry.FullJitter(cappedRetryBackoff{backoff: retry.Exponential(baseDelay, 2), maximum: maximumDelay}), nil
}

func executeRetry(request *http.Request, next Next, options resolvedRetryOptions) (*http.Response, error) {
	bodyReplayable := retryBodyReplayable(request)
	hasIdempotency := idempotencyPolicyApplied(request.Context())
	attempt := 0
	var passthrough error
	// lastStatus keeps the previous response's status so a stop before the
	// next exchange, such as a failed body replay, still reports it.
	var lastStatus error
	// Exchanges keep the caller's request context. The executor's attempt
	// context ends when Do returns, which would cancel a returned response body
	// mid-stream; Policy.OperationTimeout owns the exchange deadline instead.
	operation := func(context.Context) (retry.AttemptResult[*http.Response], error) {
		current := request
		if attempt > 0 {
			replayed, err := replayRequest(request)
			if err != nil {
				return retry.AttemptResult[*http.Response]{Outcome: retry.OutcomeKnown}, retry.Permanent(errors.Join(lastStatus, err))
			}
			current = replayed
		}
		attempt++
		response, failure := next(current)
		pending := response
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = drainAndCloseRetryResponse(pending)
				panic(recovered)
			}
		}()
		eligible := options.retry.ShouldRetry(RetryAttempt{
			Request: snapshotRequest(current), Response: response, Failure: failure,
			Attempt: attempt, BodyReplayable: bodyReplayable, HasIdempotency: hasIdempotency,
		})
		if !eligible {
			if failure != nil {
				// A failed exchange the policy does not repeat may have reached the
				// server, so its outcome is unknown; the caller receives the
				// original failure exactly as an unwrapped client would.
				passthrough = failure
				return retry.AttemptResult[*http.Response]{Outcome: retry.OutcomeUnknown}, failure
			}
			return retry.AttemptResult[*http.Response]{Value: response, Outcome: retry.OutcomeKnown}, nil
		}
		closeErr := drainAndCloseRetryResponse(response)
		pending = nil
		cause := retryCause(response, failure)
		if cause == nil {
			// Only a broken inner middleware returns neither a response nor an
			// error; whether it sent the request is unknowable, so never replay it.
			return retry.AttemptResult[*http.Response]{Outcome: retry.OutcomeKnown}, retry.Permanent(ErrInvalidMiddlewareResult)
		}
		lastStatus = nil
		if response != nil {
			lastStatus = cause
		}
		if closeErr != nil {
			return retry.AttemptResult[*http.Response]{Outcome: retry.OutcomeKnown}, retry.Permanent(errors.Join(cause, closeErr))
		}
		if !bodyReplayable {
			return retry.AttemptResult[*http.Response]{Outcome: retry.OutcomeKnown}, retry.Permanent(cause)
		}

		return retry.AttemptResult[*http.Response]{Outcome: retry.OutcomeKnown}, retry.Retryable(cause)
	}
	result, err := retry.Do(request.Context(), options.policy, operation)
	if err != nil {
		if passthrough != nil && errors.Is(err, retry.ErrOutcomeUnknown) {
			return nil, passthrough
		}

		return nil, err
	}

	return result.Value, nil
}

// classifyRetryOutcome retries only the operation's own top-level retryable
// wrapper. A retryable error nested in a failure chain, such as one returned by
// an inner go-retry user, must not override this middleware's permanent stops.
func classifyRetryOutcome(_ context.Context, err error) (retry.Classification, error) {
	if _, ok := err.(*retry.RetryableError); ok {
		return retry.ClassificationRetryable, nil
	}

	return retry.ClassificationPermanent, nil
}

// retryCause carries the response status and its first Retry-After value as a
// retry delay hint. A Retry-After value beyond the parser's raw byte bound is
// treated as absent, so the exchange falls back to the configured backoff.
func retryCause(response *http.Response, failure error) error {
	if response == nil {
		return failure
	}
	cause, err := retryhttp.NewError(response.StatusCode, response.Header, nil)
	if err == nil {
		return cause
	}
	cause, err = retryhttp.NewError(response.StatusCode, nil, nil)
	if err == nil {
		return cause
	}

	return err
}

type defaultRetryPolicy struct{ unsafeIdempotency bool }

func (policy defaultRetryPolicy) ShouldRetry(attempt RetryAttempt) bool {
	if !attempt.BodyReplayable || attempt.Request == nil || attempt.Request.Context().Err() != nil {
		return false
	}
	if !retrySafeMethod(attempt.Request.Method) && (!policy.unsafeIdempotency || !attempt.HasIdempotency) {
		return false
	}
	if attempt.Failure != nil {
		return !errors.Is(attempt.Failure, context.Canceled) && !errors.Is(attempt.Failure, context.DeadlineExceeded)
	}
	if attempt.Response == nil {
		return false
	}
	switch attempt.Response.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retrySafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace,
		http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func retryBodyReplayable(request *http.Request) bool {
	return request != nil && (request.Body == nil || request.Body == http.NoBody || request.GetBody != nil)
}

func replayRequest(original *http.Request) (*http.Request, error) {
	replayed := original.Clone(original.Context())
	if original.Body == nil || original.Body == http.NoBody {
		replayed.Body = original.Body

		return replayed, nil
	}
	body, err := original.GetBody()
	if err != nil {
		return nil, &BodyOpenError{Cause: err}
	}
	if body == nil {
		return nil, &BodyOpenError{Cause: ErrInvalidBody}
	}
	replayed.Body = body

	return replayed, nil
}

func drainAndCloseRetryResponse(response *http.Response) error {
	if response == nil || response.Body == nil {
		return nil
	}
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, retryDrainBytes+1))
	closeErr := response.Body.Close()

	return errors.Join(readErr, closeErr)
}

// cappedRetryBackoff bounds exponential growth before jitter so MaximumDelay
// limits local backoff while MaximumRetryAfter separately bounds server hints.
type cappedRetryBackoff struct {
	backoff retry.Backoff
	maximum time.Duration
}

func (backoff cappedRetryBackoff) Delay(attempt uint, previous time.Duration, random retry.Random) time.Duration {
	return min(backoff.backoff.Delay(attempt, previous, random), backoff.maximum)
}

type explicitRetryBackoff []time.Duration

func (delays explicitRetryBackoff) Delay(attempt uint, _ time.Duration, _ retry.Random) time.Duration {
	if attempt == 0 || attempt > uint(len(delays)) {
		return delays[len(delays)-1]
	}

	return delays[attempt-1]
}

type retryPolicyClock struct{ clock RetryClock }

func (clock retryPolicyClock) Now() time.Time { return clock.clock.Now() }

func (clock retryPolicyClock) Sleep(ctx context.Context, delay time.Duration) error {
	return clock.clock.Wait(ctx, delay)
}

// WithTimeout deliberately adds no per-attempt deadline. MaximumElapsed is
// checked between exchanges; a go-retry attempt deadline would also end the
// attempt early and drop it from the retry history, while exchange deadlines
// belong to Policy.OperationTimeout.
func (retryPolicyClock) WithTimeout(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
	return parent, func() {}
}

func newRetryRandom() (retry.Random, error) {
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, fmt.Errorf("%w: random seed unavailable", ErrInvalidRetryPolicy)
	}

	return retry.NewRandom(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:])), nil
}

type systemRetryClock struct{}

func (systemRetryClock) Now() time.Time { return time.Now() }

func (systemRetryClock) Wait(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
