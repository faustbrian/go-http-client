package httpclient

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrInvalidPool indicates malformed pool policy, source, or metadata.
	ErrInvalidPool = errors.New("invalid HTTP request pool")
	// ErrPoolLimit indicates that a finite pool-wide budget was reached.
	ErrPoolLimit = errors.New("HTTP request pool limit reached")
)

const (
	defaultPoolConcurrency     = 4
	defaultPoolMaximumRequests = 10_000
	defaultPoolMaximumElapsed  = 5 * time.Minute
	defaultPoolMaximumBytes    = 64 << 20
	maximumPoolConcurrency     = 1 << 10
	maximumPoolPending         = 1 << 20
	unknownPoolRequestCount    = -1
)

// PoolResultOrder controls result presentation independently from execution.
type PoolResultOrder uint8

const (
	// PoolInputOrder returns results in stable source order.
	PoolInputOrder PoolResultOrder = iota
	// PoolCompletionOrder returns results as workers complete.
	PoolCompletionOrder
)

// PoolFailureMode controls request-error cancellation.
type PoolFailureMode uint8

const (
	// PoolCollectAll retains every per-request result and error.
	PoolCollectAll PoolFailureMode = iota
	// PoolFailFast cancels pending work after the first request error.
	PoolFailFast
)

// PoolValue is one successful typed value and its explicit budget accounting.
type PoolValue[Output any] struct {
	// Value is the executor output; referenced storage remains caller-owned.
	Value Output
	// ResponseBytes is caller-reported response-byte accounting, not a measurement of allocations.
	ResponseBytes int64
	// MemoryBytes is caller-reported retained-output accounting, not measured heap use.
	MemoryBytes int64
}

// PoolExecutor executes one input through caller-owned HTTP policy.
type PoolExecutor[Input any, Output any] func(
	context.Context,
	Input,
) (PoolValue[Output], error)

// PoolKey returns a stable low-cardinality result key.
type PoolKey[Input any] func(Input) (string, error)

// PoolGenerator yields one input at a time and must honor context cancellation.
type PoolGenerator[Input any] func(context.Context) (Input, bool, error)

// PoolWorkload describes source shape for bounded concurrency selection.
type PoolWorkload struct {
	// KnownRequests is the known count or -1 for a lazy/unknown source.
	KnownRequests int
}

// PoolConcurrencySelector chooses one run-wide worker count within configured
// minimum and maximum bounds.
type PoolConcurrencySelector func(PoolWorkload) int

// PoolLimits are finite run-wide budgets. Zero fields select safe defaults.
type PoolLimits struct {
	// MaximumRequests bounds scheduled requests; zero selects 10,000.
	MaximumRequests int
	// MaximumElapsed cancels the run after the clock wait; noncooperating callbacks can delay
	// return; zero selects five minutes.
	MaximumElapsed time.Duration
	// MaximumResponseBytes bounds accepted caller-reported bytes; zero selects 64 MiB.
	MaximumResponseBytes int64
	// MaximumMemoryBytes bounds accepted caller-reported memory; zero selects 64 MiB.
	MaximumMemoryBytes int64
}

// PoolOptions configures one immutable typed execution pool.
type PoolOptions[Input any, Output any] struct {
	// Concurrency selects fixed worker count; zero selects four.
	Concurrency int
	// MinimumConcurrency bounds adaptive selection from below; zero selects one.
	MinimumConcurrency int
	// MaximumConcurrency bounds adaptive selection from above; zero selects 32, at most 1,024.
	MaximumConcurrency int
	// SelectConcurrency is a borrowed once-per-run selector; it runs before the run timeout is
	// installed.
	SelectConcurrency PoolConcurrencySelector
	// Pending bounds queued inputs; zero selects fixed concurrency, at most 1,048,576.
	Pending int
	// Order selects input order or completion order.
	Order PoolResultOrder
	// Failure selects collect-all or fail-fast cancellation.
	Failure PoolFailureMode
	// Limits selects per-run accounting bounds; zero fields use the helper defaults.
	Limits PoolLimits
	// Clock is borrowed; Now must be nonblocking and Wait must honor cancellation.
	Clock RetryClock
	// Key is borrowed and supplies result metadata; it has no context argument and must return promptly.
	Key PoolKey[Input]
	// Execute is borrowed; it owns response bodies and must honor context.
	Execute PoolExecutor[Input, Output]
}

// PoolResult preserves one source item, typed value, metadata, and independent
// request failure.
type PoolResult[Input any, Output any] struct {
	// Index is the zero-based source position.
	Index int
	// Key is caller-supplied metadata, not automatically redacted.
	Key string
	// Input is a shallow copy of the source input.
	Input Input
	// Value is a shallow copy of the executor result.
	Value Output
	// ResponseBytes is caller-reported response-byte accounting, not a measurement of allocations.
	ResponseBytes int64
	// MemoryBytes is reported retained-output accounting.
	MemoryBytes int64
	// Error is the per-request error; collect-all does not promote it to the run error.
	Error error
}

// PoolError reports source, cancellation, fail-fast, or budget termination
// without rendering input, key, response, or underlying error text.
type PoolError struct {
	// Completed counts results retained when the run stopped.
	Completed int
	// Cause retains the underlying error for unwrapping; it may contain sensitive provider data.
	Cause error
}

// Error implements error without rendering potentially sensitive causes.
func (*PoolError) Error() string { return "HTTP request pool failed" }

// Unwrap returns the termination cause.
func (err *PoolError) Unwrap() error { return err.Cause }

// PoolPanicError reports a contained selector, source, key, or executor panic.
// Value remains available programmatically but is never rendered.
type PoolPanicError struct {
	// Stage names source, key, execute or selector invocation.
	Stage string
	// Value retains the recovered payload without rendering it; it may be sensitive.
	Value any
}

// Error implements error without rendering the panic value.
func (err *PoolPanicError) Error() string {
	return fmt.Sprintf("HTTP request pool %s callback panicked", err.Stage)
}

// Pool is an immutable reusable worker policy.
type Pool[Input any, Output any] struct {
	concurrency int
	minimum     int
	maximum     int
	selector    PoolConcurrencySelector
	pending     int
	order       PoolResultOrder
	failure     PoolFailureMode
	limits      PoolLimits
	clock       RetryClock
	key         PoolKey[Input]
	execute     PoolExecutor[Input, Output]
}

// NewPool validates and constructs a reusable pool without starting workers.
func NewPool[Input any, Output any](
	options PoolOptions[Input, Output],
) (*Pool[Input, Output], error) {
	if nilLike(options.Execute) {
		return nil, fmt.Errorf("%w: executor is nil", ErrInvalidPool)
	}
	if options.Order > PoolCompletionOrder || options.Failure > PoolFailFast {
		return nil, fmt.Errorf("%w: enum policy is unknown", ErrInvalidPool)
	}
	minimum := options.MinimumConcurrency
	if minimum == 0 {
		minimum = 1
	}
	maximum := options.MaximumConcurrency
	if maximum == 0 {
		maximum = 32
	}
	concurrency := options.Concurrency
	if concurrency == 0 {
		concurrency = defaultPoolConcurrency
	}
	switch {
	case minimum < 1:
		return nil, fmt.Errorf("%w: concurrency bounds are invalid", ErrInvalidPool)
	case maximum < minimum:
		return nil, fmt.Errorf("%w: concurrency bounds are invalid", ErrInvalidPool)
	case maximum > maximumPoolConcurrency:
		return nil, fmt.Errorf("%w: concurrency bounds are invalid", ErrInvalidPool)
	case concurrency < minimum:
		return nil, fmt.Errorf("%w: concurrency bounds are invalid", ErrInvalidPool)
	case concurrency > maximum:
		return nil, fmt.Errorf("%w: concurrency bounds are invalid", ErrInvalidPool)
	}
	pending := options.Pending
	if pending == 0 {
		pending = concurrency
	}
	if pending < 1 {
		return nil, fmt.Errorf("%w: pending bound is invalid", ErrInvalidPool)
	}
	if pending > maximumPoolPending {
		return nil, fmt.Errorf("%w: pending bound is invalid", ErrInvalidPool)
	}
	limits, err := resolvePoolLimits(options.Limits)
	if err != nil {
		return nil, err
	}
	clock := options.Clock
	if clock == nil {
		clock = systemRetryClock{}
	} else if nilLike(clock) {
		return nil, fmt.Errorf("%w: clock is nil", ErrInvalidPool)
	}
	return &Pool[Input, Output]{
		concurrency: concurrency, minimum: minimum, maximum: maximum,
		selector: options.SelectConcurrency, pending: pending,
		order: options.Order, failure: options.Failure, limits: limits,
		clock: clock, key: options.Key, execute: options.Execute,
	}, nil
}

func resolvePoolLimits(limits PoolLimits) (PoolLimits, error) {
	switch {
	case limits.MaximumRequests < 0:
		return PoolLimits{}, fmt.Errorf("%w: limit is negative", ErrInvalidPool)
	case limits.MaximumElapsed < 0:
		return PoolLimits{}, fmt.Errorf("%w: limit is negative", ErrInvalidPool)
	case limits.MaximumResponseBytes < 0:
		return PoolLimits{}, fmt.Errorf("%w: limit is negative", ErrInvalidPool)
	case limits.MaximumMemoryBytes < 0:
		return PoolLimits{}, fmt.Errorf("%w: limit is negative", ErrInvalidPool)
	}
	if limits.MaximumRequests == 0 {
		limits.MaximumRequests = defaultPoolMaximumRequests
	}
	if limits.MaximumElapsed == 0 {
		limits.MaximumElapsed = defaultPoolMaximumElapsed
	}
	if limits.MaximumResponseBytes == 0 {
		limits.MaximumResponseBytes = defaultPoolMaximumBytes
	}
	if limits.MaximumMemoryBytes == 0 {
		limits.MaximumMemoryBytes = defaultPoolMaximumBytes
	}
	return limits, nil
}

// RunSlice shallow-copies inputs before applying the request-count bound.
// Callers must bound input allocation and synchronize referenced values.
func (pool *Pool[Input, Output]) RunSlice(
	ctx context.Context,
	inputs []Input,
) ([]PoolResult[Input, Output], error) {
	snapshot := append([]Input(nil), inputs...)
	index := 0

	return pool.run(ctx, len(snapshot), func(context.Context) (Input, bool, error) {
		value := snapshot[index]
		index++

		return value, true, nil
	})
}

// RunGenerator executes a lazy caller-owned source with bounded backpressure.
func (pool *Pool[Input, Output]) RunGenerator(
	ctx context.Context,
	generator PoolGenerator[Input],
) ([]PoolResult[Input, Output], error) {
	if nilLike(generator) {
		return nil, fmt.Errorf("%w: generator is nil", ErrInvalidPool)
	}

	return pool.run(ctx, unknownPoolRequestCount, generator)
}

// RunChannel executes inputs until channel closure or cancellation.
func (pool *Pool[Input, Output]) RunChannel(
	ctx context.Context,
	input <-chan Input,
) ([]PoolResult[Input, Output], error) {
	if nilLike(input) {
		return nil, fmt.Errorf("%w: input channel is nil", ErrInvalidPool)
	}

	return pool.run(ctx, unknownPoolRequestCount, func(ctx context.Context) (Input, bool, error) {
		return receivePoolInput(ctx, input)
	})
}

func receivePoolInput[Input any](ctx context.Context, input <-chan Input) (Input, bool, error) {
	var zero Input
	select {
	case <-ctx.Done():
		return zero, false, ctx.Err()
	case value, ok := <-input:
		return value, ok, nil
	}
}

type poolJob[Input any] struct {
	index int
	key   string
	input Input
}

func (pool *Pool[Input, Output]) run(
	ctx context.Context,
	knownRequests int,
	source PoolGenerator[Input],
) ([]PoolResult[Input, Output], error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: context is nil", ErrInvalidPool)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if knownRequests > pool.limits.MaximumRequests {
		return nil, &PoolError{Cause: ErrPoolLimit}
	}
	concurrency, err := pool.selectConcurrency(PoolWorkload{KnownRequests: knownRequests})
	if err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan poolJob[Input], pool.pending)
	completed := make(chan PoolResult[Input, Output], concurrency)
	sourceFailure := make(chan error, 1)
	deadlineDone := make(chan struct{})
	var elapsed atomic.Bool
	go func() {
		defer close(deadlineDone)
		if err := pool.clock.Wait(runCtx, pool.limits.MaximumElapsed); err != nil {
			return
		}
		elapsed.Store(true)
		cancel()
	}()

	var workers sync.WaitGroup
	workers.Add(concurrency)
	for range concurrency {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-runCtx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					value, executeErr := invokePoolExecutor(pool.execute, runCtx, job.input)
					result := PoolResult[Input, Output]{
						Index: job.index, Key: job.key, Input: job.input,
						Value: value.Value, ResponseBytes: value.ResponseBytes,
						MemoryBytes: value.MemoryBytes, Error: executeErr,
					}
					completed <- result
				}
			}
		}()
	}
	go pool.schedule(runCtx, cancel, knownRequests, source, jobs, sourceFailure)
	go func() {
		workers.Wait()
		close(completed)
	}()

	results := make([]PoolResult[Input, Output], 0, max(knownRequests, 0))
	accounting := poolBudgetState{}
	var runFailure error
	for result := range completed {
		resultFailure := pool.accountResult(&accounting, result, elapsed.Load())
		if resultFailure != nil {
			if runFailure == nil {
				runFailure = resultFailure
				cancel()
			}
		} else {
			results = append(results, result)
			if pool.failure == PoolFailFast && result.Error != nil && runFailure == nil {
				runFailure = result.Error
				cancel()
			}
		}
	}
	cancel()
	<-deadlineDone
	scheduleFailure := <-sourceFailure
	if runFailure == nil {
		runFailure = scheduleFailure
	}
	if runFailure == nil {
		if elapsed.Load() {
			runFailure = ErrPoolLimit
		}
	}
	orderPoolResults(results, pool.order)
	if runFailure != nil {
		return results, &PoolError{Completed: len(results), Cause: runFailure}
	}
	if err := ctx.Err(); err != nil {
		return results, &PoolError{Completed: len(results), Cause: err}
	}

	return results, nil
}

type poolBudgetState struct {
	responseBytes int64
	memoryBytes   int64
}

func (pool *Pool[Input, Output]) accountResult(
	state *poolBudgetState,
	result PoolResult[Input, Output],
	elapsed bool,
) error {
	if elapsed {
		return ErrPoolLimit
	}
	if result.ResponseBytes < 0 {
		return ErrInvalidPool
	}
	if result.MemoryBytes < 0 {
		return ErrInvalidPool
	}
	if result.ResponseBytes > pool.limits.MaximumResponseBytes-state.responseBytes {
		return ErrPoolLimit
	}
	if result.MemoryBytes > pool.limits.MaximumMemoryBytes-state.memoryBytes {
		return ErrPoolLimit
	}
	state.responseBytes += result.ResponseBytes
	state.memoryBytes += result.MemoryBytes

	return nil
}

func orderPoolResults[Input any, Output any](
	results []PoolResult[Input, Output],
	order PoolResultOrder,
) {
	if order == PoolInputOrder {
		sort.SliceStable(results, func(left int, right int) bool {
			return cmp.Compare(results[left].Index, results[right].Index) == -1
		})
	}
}

func (pool *Pool[Input, Output]) selectConcurrency(workload PoolWorkload) (int, error) {
	selected := pool.concurrency
	if pool.selector != nil {
		var panicValue any
		func() {
			defer func() { panicValue = recover() }()
			selected = pool.selector(workload)
		}()
		if panicValue != nil {
			return 0, &PoolPanicError{Stage: "concurrency", Value: panicValue}
		}
	}
	if selected < pool.minimum {
		return 0, fmt.Errorf("%w: selected concurrency is out of bounds", ErrInvalidPool)
	}
	if selected > pool.maximum {
		return 0, fmt.Errorf("%w: selected concurrency is out of bounds", ErrInvalidPool)
	}

	return selected, nil
}

func (pool *Pool[Input, Output]) schedule(
	ctx context.Context,
	cancel context.CancelFunc,
	knownRequests int,
	source PoolGenerator[Input],
	jobs chan<- poolJob[Input],
	failure chan<- error,
) {
	defer close(jobs)
	requestCount := pool.limits.MaximumRequests
	knownSource := knownRequests != unknownPoolRequestCount
	if knownSource {
		requestCount = knownRequests
	}
	for index := range requestCount {
		if err := ctx.Err(); err != nil {
			failure <- nil
			return
		}
		input, ok, sourceErr := invokePoolGenerator(source, ctx)
		if sourceErr != nil {
			failure <- sourceErr
			cancel()
			return
		}
		if !ok {
			failure <- nil
			return
		}
		key := fmt.Sprintf("%d", index)
		if pool.key != nil {
			var keyErr error
			key, keyErr = invokePoolKey(pool.key, input)
			if keyErr != nil {
				failure <- keyErr
				cancel()
				return
			}
		}
		job := poolJob[Input]{index: index, key: key, input: input}
		select {
		case jobs <- job:
		case <-ctx.Done():
			failure <- nil
			return
		}
	}
	if knownSource {
		failure <- nil
		return
	}
	failure <- ErrPoolLimit
}

func invokePoolExecutor[Input any, Output any](
	executor PoolExecutor[Input, Output],
	ctx context.Context,
	input Input,
) (value PoolValue[Output], failure error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			value = PoolValue[Output]{}
			failure = &PoolPanicError{Stage: "executor", Value: recovered}
		}
	}()

	return executor(ctx, input)
}

func invokePoolGenerator[Input any](
	generator PoolGenerator[Input],
	ctx context.Context,
) (input Input, ok bool, failure error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			failure = &PoolPanicError{Stage: "generator", Value: recovered}
		}
	}()

	return generator(ctx)
}

func invokePoolKey[Input any](key PoolKey[Input], input Input) (value string, failure error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			failure = &PoolPanicError{Stage: "key", Value: recovered}
		}
	}()

	value, failure = key(input)
	return value, failure
}
