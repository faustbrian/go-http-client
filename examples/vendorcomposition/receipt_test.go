package vendorcomposition_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	httpclient "github.com/faustbrian/go-http-client"
)

func TestVendorCompositionRetriesThenReusesCache(t *testing.T) {
	t.Parallel()

	origin := &receiptOrigin{}
	admission := &receiptLimiter{origin: origin}
	dependency := &receiptBreaker{}
	isolation := &receiptBulkhead{slots: make(chan struct{}, 1)}
	telemetry := &receiptObserver{}
	client := newReceiptClient(t, origin, admission, dependency, isolation, telemetry)

	first := executeReceiptRequest(t, client, http.MethodGet)
	if string(first.body) != "vendor-ok" || first.cache != httpclient.CacheMiss {
		t.Fatalf("first response = %q, cache %v", first.body, first.cache)
	}
	second := executeReceiptRequest(t, client, http.MethodGet)
	if string(second.body) != "vendor-ok" || second.cache != httpclient.CacheHit {
		t.Fatalf("second response = %q, cache %v", second.body, second.cache)
	}

	if got := origin.calls(); got != 2 {
		t.Fatalf("origin calls = %d, want two retry attempts and no cache-hit call", got)
	}
	if got := origin.closedBodies(); got != 2 {
		t.Fatalf("closed origin bodies = %d, want both retry responses closed", got)
	}
	if got := admission.acquisitions(); got != 3 {
		t.Fatalf("rate admissions = %d, want two operations plus one retry", got)
	}
	if got := admission.checkpoints(); !equalInts(got, []int{0, 1, 2}) {
		t.Fatalf("rate admission origin checkpoints = %v, want [0 1 2]", got)
	}
	if got := dependency.executions(); got != 1 {
		t.Fatalf("breaker executions = %d, want one cache-miss operation", got)
	}
	if entries, maximum := isolation.snapshot(); entries != 2 || maximum != 1 {
		t.Fatalf("bulkhead entries/max active = %d/%d, want 2/1", entries, maximum)
	}

	wantTelemetry := []receiptTelemetry{
		{phase: httpclient.TelemetryStart, scope: httpclient.TelemetryOperation, cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryStart, scope: httpclient.TelemetryAttempt, attempt: 1, cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryFinish, scope: httpclient.TelemetryAttempt, attempt: 1, outcome: httpclient.TelemetryOutcomeHTTPError, status: "5xx", cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryStart, scope: httpclient.TelemetryAttempt, attempt: 2, cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryFinish, scope: httpclient.TelemetryAttempt, attempt: 2, outcome: httpclient.TelemetryOutcomeSuccess, status: "2xx", cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryFinish, scope: httpclient.TelemetryOperation, outcome: httpclient.TelemetryOutcomeSuccess, status: "2xx", cache: httpclient.TelemetryCacheMiss},
		{phase: httpclient.TelemetryStart, scope: httpclient.TelemetryOperation, cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryFinish, scope: httpclient.TelemetryOperation, outcome: httpclient.TelemetryOutcomeSuccess, status: "2xx", cache: httpclient.TelemetryCacheHit},
	}
	if got := telemetry.snapshot(); !equalReceiptTelemetry(got, wantTelemetry) {
		t.Fatalf("telemetry =\n%v\nwant\n%v", got, wantTelemetry)
	}
}

func TestVendorCompositionPreservesUnknownPOSTOutcomeWithoutRetry(t *testing.T) {
	t.Parallel()

	disconnect := errors.New("connection lost after dispatch")
	origin := &receiptOrigin{failure: disconnect}
	admission := &receiptLimiter{}
	dependency := &receiptBreaker{}
	isolation := &receiptBulkhead{slots: make(chan struct{}, 1)}
	telemetry := &receiptObserver{}
	client := newReceiptClient(t, origin, admission, dependency, isolation, telemetry)

	vendor := vendorClient{http: client}
	err := vendor.createOrder(context.Background())
	var unknown *vendorUnknownOutcomeError
	if !errors.As(err, &unknown) {
		t.Fatalf("vendor error = %v, want typed unknown outcome", err)
	}
	if !errors.Is(err, httpclient.ErrRetryExhausted) || !errors.Is(err, disconnect) {
		t.Fatalf("unknown-outcome error = %v", err)
	}
	var exhausted *httpclient.RetryExhaustedError
	if !errors.As(err, &exhausted) || exhausted.Attempts != 1 {
		t.Fatalf("retry exhaustion = %#v, want one attempt", exhausted)
	}

	if got := origin.calls(); got != 1 {
		t.Fatalf("unsafe POST origin calls = %d, want one", got)
	}
	if got := admission.acquisitions(); got != 1 {
		t.Fatalf("unsafe POST rate admissions = %d, want one", got)
	}
	if got := dependency.executions(); got != 1 {
		t.Fatalf("unsafe POST breaker executions = %d, want one", got)
	}
	if entries, maximum := isolation.snapshot(); entries != 1 || maximum != 1 {
		t.Fatalf("unsafe POST bulkhead entries/max active = %d/%d, want 1/1", entries, maximum)
	}

	wantTelemetry := []receiptTelemetry{
		{phase: httpclient.TelemetryStart, scope: httpclient.TelemetryOperation, cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryStart, scope: httpclient.TelemetryAttempt, attempt: 1, cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryFinish, scope: httpclient.TelemetryAttempt, attempt: 1, outcome: httpclient.TelemetryOutcomeFailure, cache: httpclient.TelemetryCacheNone},
		{phase: httpclient.TelemetryFinish, scope: httpclient.TelemetryOperation, outcome: httpclient.TelemetryOutcomeRetryFailure, cache: httpclient.TelemetryCacheNone},
	}
	if got := telemetry.snapshot(); !equalReceiptTelemetry(got, wantTelemetry) {
		t.Fatalf("unknown-outcome telemetry =\n%v\nwant\n%v", got, wantTelemetry)
	}
}

func TestVendorCompositionKeepsSuccessfulResponseCallerOwned(t *testing.T) {
	t.Parallel()

	origin := &receiptOrigin{noStore: true}
	client := newReceiptClient(t, origin, &receiptLimiter{}, &receiptBreaker{}, &receiptBulkhead{slots: make(chan struct{}, 1)}, &receiptObserver{})
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://vendor.example.test/resource", nil)
	if err != nil {
		t.Fatalf("construct request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	if got := origin.closedBodies(); got != 0 {
		t.Fatalf("origin body closed before caller release = %d", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "vendor-ok" {
		t.Fatalf("read response = %q, %v", body, err)
	}
	if got := origin.closedBodies(); got != 0 {
		t.Fatalf("origin body closed by read = %d", got)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}
	if got := origin.closedBodies(); got != 1 {
		t.Fatalf("origin body closes after caller release = %d, want one", got)
	}
}

func TestVendorCompositionBulkheadSerializesConcurrentAttempts(t *testing.T) {
	t.Parallel()

	origin := &receiptBlockingOrigin{
		started: make(chan struct{}, 2), release: make(chan struct{}),
	}
	isolation := &receiptBulkhead{
		slots: make(chan struct{}, 1), waiting: make(chan struct{}),
	}
	client := newReceiptClient(t, origin, &receiptLimiter{}, &receiptBreaker{}, isolation, &receiptObserver{})
	firstDone := make(chan error, 1)
	go func() { firstDone <- executeBypassingCache(client) }()
	<-origin.started
	secondDone := make(chan error, 1)
	go func() { secondDone <- executeBypassingCache(client) }()
	<-isolation.waiting
	if got := origin.calls(); got != 1 {
		t.Fatalf("origin calls while second attempt waits = %d, want one", got)
	}
	close(origin.release)
	for index, done := range []<-chan error{firstDone, secondDone} {
		if err := <-done; err != nil {
			t.Fatalf("concurrent request %d: %v", index+1, err)
		}
	}
	if got := origin.calls(); got != 2 {
		t.Fatalf("origin calls after release = %d, want two", got)
	}
	if entries, maximum := isolation.snapshot(); entries != 2 || maximum != 1 {
		t.Fatalf("bulkhead entries/max active = %d/%d, want 2/1", entries, maximum)
	}
}

func TestVendorCompositionStopsBeforeTransportOnAdmissionFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		limiterFailure error
		breakerFailure error
		want           error
		wantOutcome    httpclient.TelemetryOutcome
		wantAdmissions int
		wantExecutions int
	}{
		{
			name: "rate capacity", limiterFailure: httpclient.ErrRateLimitCapacity,
			want: httpclient.ErrRateLimitCapacity, wantOutcome: httpclient.TelemetryOutcomeRateLimited,
			wantAdmissions: 1,
		},
		{
			name: "open circuit", breakerFailure: httpclient.ErrCircuitRejected,
			want: httpclient.ErrCircuitRejected, wantOutcome: httpclient.TelemetryOutcomeCircuitOpen,
			wantAdmissions: 1, wantExecutions: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			origin := &receiptOrigin{}
			admission := &receiptLimiter{failure: test.limiterFailure}
			dependency := &receiptBreaker{failure: test.breakerFailure}
			isolation := &receiptBulkhead{slots: make(chan struct{}, 1)}
			telemetry := &receiptObserver{}
			client := newReceiptClient(t, origin, admission, dependency, isolation, telemetry)

			vendor := vendorClient{http: client}
			err := vendor.createOrder(context.Background())
			if !errors.Is(err, test.want) {
				t.Fatalf("admission error = %v, want %v", err, test.want)
			}
			var unknown *vendorUnknownOutcomeError
			if errors.As(err, &unknown) {
				t.Fatalf("local admission error classified as unknown outcome: %v", err)
			}
			if got := origin.calls(); got != 0 {
				t.Fatalf("origin calls = %d, want zero", got)
			}
			if got := admission.acquisitions(); got != test.wantAdmissions {
				t.Fatalf("rate admissions = %d, want %d", got, test.wantAdmissions)
			}
			if got := dependency.executions(); got != test.wantExecutions {
				t.Fatalf("breaker executions = %d, want %d", got, test.wantExecutions)
			}
			if entries, maximum := isolation.snapshot(); entries != 0 || maximum != 0 {
				t.Fatalf("bulkhead entries/max active = %d/%d, want 0/0", entries, maximum)
			}
			wantTelemetry := []receiptTelemetry{
				{phase: httpclient.TelemetryStart, scope: httpclient.TelemetryOperation, cache: httpclient.TelemetryCacheNone},
				{phase: httpclient.TelemetryFinish, scope: httpclient.TelemetryOperation, outcome: test.wantOutcome, cache: httpclient.TelemetryCacheNone},
			}
			if got := telemetry.snapshot(); !equalReceiptTelemetry(got, wantTelemetry) {
				t.Fatalf("admission telemetry =\n%v\nwant\n%v", got, wantTelemetry)
			}
		})
	}
}

type receiptResult struct {
	body  []byte
	cache httpclient.CacheProvenance
}

func executeReceiptRequest(t *testing.T, client *httpclient.Client, method string) receiptResult {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), method, "https://vendor.example.test/resource", nil)
	if err != nil {
		t.Fatalf("construct request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}
	metadata, ok := httpclient.CacheMetadataFromResponse(response)
	if !ok {
		t.Fatal("response has no cache provenance")
	}

	return receiptResult{body: body, cache: metadata.Provenance}
}

func newReceiptClient(
	t *testing.T,
	origin http.RoundTripper,
	admission *receiptLimiter,
	dependency *receiptBreaker,
	isolation *receiptBulkhead,
	telemetry *receiptObserver,
) *httpclient.Client {
	t.Helper()
	cache, err := httpclient.NewMemoryCache(httpclient.MemoryCacheOptions{MaximumEntries: 8, MaximumBytes: 1 << 20})
	if err != nil {
		t.Fatalf("construct cache: %v", err)
	}
	cacheMiddleware, err := httpclient.NewCacheMiddleware(httpclient.CacheOptions{
		Name: "vendor-cache", Store: cache, Methods: []string{http.MethodGet},
		Statuses: []int{http.StatusOK}, VariantKey: []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("construct cache middleware: %v", err)
	}
	rateMiddleware, err := httpclient.NewRateLimitMiddleware(httpclient.RateLimitOptions{Name: "vendor-rate", Limiter: admission})
	if err != nil {
		t.Fatalf("construct rate middleware: %v", err)
	}
	breakerMiddleware, err := httpclient.NewCircuitBreakerMiddleware(httpclient.CircuitBreakerOptions{Name: "vendor-breaker", Breaker: dependency})
	if err != nil {
		t.Fatalf("construct breaker middleware: %v", err)
	}
	retryMiddleware, err := httpclient.NewRetryMiddleware(httpclient.RetryOptions{
		Name: "vendor-retry", MaximumAttempts: 2, MaximumElapsed: time.Second,
		Delays: []time.Duration{time.Nanosecond}, Clock: receiptClock{},
	})
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}
	bulkheadMiddleware, err := httpclient.NewTransportMiddleware(httpclient.MiddlewareOptions{
		Name: "vendor-bulkhead", Scope: httpclient.ScopeAttempt, Layer: httpclient.MiddlewareEndpoint,
	}, isolation.around)
	if err != nil {
		t.Fatalf("construct bulkhead middleware: %v", err)
	}
	middleware := append(rateMiddleware, breakerMiddleware, retryMiddleware, bulkheadMiddleware)
	middleware = append(middleware, cacheMiddleware)
	client, err := httpclient.New(httpclient.Config{
		Transport: origin, Telemetry: &httpclient.TelemetryOptions{Observer: telemetry}, Middleware: middleware,
	})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	})

	return client
}

type receiptOrigin struct {
	mu       sync.Mutex
	requests int
	closed   int
	failure  error
	noStore  bool
}

func (origin *receiptOrigin) RoundTrip(request *http.Request) (*http.Response, error) {
	origin.mu.Lock()
	origin.requests++
	attempt := origin.requests
	failure := origin.failure
	noStore := origin.noStore
	origin.mu.Unlock()
	if failure != nil {
		return nil, failure
	}
	status := http.StatusOK
	body := "vendor-ok"
	if attempt == 1 && !noStore {
		status = http.StatusServiceUnavailable
		body = "retry"
	}
	header := http.Header{"Content-Type": {"text/plain"}, "Cache-Control": {"max-age=60"}}
	if noStore {
		header.Set("Cache-Control", "no-store")
	}

	return &http.Response{
		StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: header, Body: &receiptBody{Buffer: bytes.NewBufferString(body), closed: origin.closeBody},
		ContentLength: int64(len(body)), Request: request,
	}, nil
}

func (origin *receiptOrigin) closeBody() {
	origin.mu.Lock()
	origin.closed++
	origin.mu.Unlock()
}

func (origin *receiptOrigin) calls() int {
	origin.mu.Lock()
	defer origin.mu.Unlock()

	return origin.requests
}

func (origin *receiptOrigin) closedBodies() int {
	origin.mu.Lock()
	defer origin.mu.Unlock()

	return origin.closed
}

type receiptBody struct {
	*bytes.Buffer
	closed func()
	once   sync.Once
}

func (body *receiptBody) Close() error {
	body.once.Do(body.closed)

	return nil
}

type receiptLimiter struct {
	mu        sync.Mutex
	acquire   int
	failure   error
	origin    *receiptOrigin
	acquireAt []int
}

func (limiter *receiptLimiter) Acquire(context.Context, time.Duration) (time.Duration, error) {
	originCalls := -1
	if limiter.origin != nil {
		originCalls = limiter.origin.calls()
	}
	limiter.mu.Lock()
	limiter.acquire++
	limiter.acquireAt = append(limiter.acquireAt, originCalls)
	failure := limiter.failure
	limiter.mu.Unlock()

	return 0, failure
}

func (limiter *receiptLimiter) checkpoints() []int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	return append([]int(nil), limiter.acquireAt...)
}

func (*receiptLimiter) DeferUntil(time.Time) {}
func (*receiptLimiter) Now() time.Time       { return time.Unix(1_700_000_000, 0) }

func (limiter *receiptLimiter) acquisitions() int {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	return limiter.acquire
}

type receiptBreaker struct {
	mu      sync.Mutex
	calls   int
	failure error
}

func (breaker *receiptBreaker) Execute(ctx context.Context, operation func(context.Context) (*http.Response, error)) (*http.Response, error) {
	breaker.mu.Lock()
	breaker.calls++
	failure := breaker.failure
	breaker.mu.Unlock()
	if failure != nil {
		return nil, failure
	}

	return operation(ctx)
}

func (breaker *receiptBreaker) executions() int {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	return breaker.calls
}

type receiptBulkhead struct {
	slots    chan struct{}
	waiting  chan struct{}
	waitOnce sync.Once
	mu       sync.Mutex
	entry    int
	active   int
	maximum  int
}

func (bulkhead *receiptBulkhead) around(request *http.Request, next httpclient.Next) (*http.Response, error) {
	select {
	case bulkhead.slots <- struct{}{}:
		return bulkhead.execute(request, next)
	default:
		if bulkhead.waiting != nil {
			bulkhead.waitOnce.Do(func() { close(bulkhead.waiting) })
		}
	}
	select {
	case bulkhead.slots <- struct{}{}:
		return bulkhead.execute(request, next)
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}
}

func (bulkhead *receiptBulkhead) execute(request *http.Request, next httpclient.Next) (*http.Response, error) {
	bulkhead.mu.Lock()
	bulkhead.entry++
	bulkhead.active++
	bulkhead.maximum = max(bulkhead.maximum, bulkhead.active)
	bulkhead.mu.Unlock()
	defer func() {
		bulkhead.mu.Lock()
		bulkhead.active--
		bulkhead.mu.Unlock()
		<-bulkhead.slots
	}()

	return next(request)
}

func (bulkhead *receiptBulkhead) snapshot() (int, int) {
	bulkhead.mu.Lock()
	defer bulkhead.mu.Unlock()

	return bulkhead.entry, bulkhead.maximum
}

type receiptClock struct{}

func (receiptClock) Now() time.Time                            { return time.Unix(1_700_000_000, 0) }
func (receiptClock) Wait(context.Context, time.Duration) error { return nil }

type receiptTelemetry struct {
	phase   httpclient.TelemetryPhase
	scope   httpclient.TelemetryScope
	attempt int
	outcome httpclient.TelemetryOutcome
	status  string
	cache   httpclient.TelemetryCacheOutcome
}

type receiptObserver struct {
	mu     sync.Mutex
	events []receiptTelemetry
}

func (observer *receiptObserver) Start(ctx context.Context, event httpclient.TelemetryEvent) context.Context {
	observer.record(event)

	return ctx
}

func (observer *receiptObserver) Finish(_ context.Context, event httpclient.TelemetryEvent) {
	observer.record(event)
}

func (observer *receiptObserver) record(event httpclient.TelemetryEvent) {
	observer.mu.Lock()
	observer.events = append(observer.events, receiptTelemetry{
		phase: event.Phase, scope: event.Scope, attempt: event.Attempt,
		outcome: event.Outcome, status: event.StatusClass, cache: event.Cache,
	})
	observer.mu.Unlock()
}

func (observer *receiptObserver) snapshot() []receiptTelemetry {
	observer.mu.Lock()
	defer observer.mu.Unlock()

	return append([]receiptTelemetry(nil), observer.events...)
}

func equalReceiptTelemetry(left, right []receiptTelemetry) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

func equalInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

type vendorClient struct{ http *httpclient.Client }

type vendorUnknownOutcomeError struct{ cause error }

func (*vendorUnknownOutcomeError) Error() string     { return "vendor operation outcome unknown" }
func (err *vendorUnknownOutcomeError) Unwrap() error { return err.cause }

func (vendor vendorClient) createOrder(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://vendor.example.test/orders", bytes.NewReader([]byte(`{"order":"123"}`)))
	if err != nil {
		return err
	}
	response, err := vendor.http.Do(request)
	if err != nil {
		var transport *httpclient.TransportError
		if errors.As(err, &transport) {
			return &vendorUnknownOutcomeError{cause: err}
		}

		return err
	}
	return response.Body.Close()
}

type receiptBlockingOrigin struct {
	mu       sync.Mutex
	requests int
	started  chan struct{}
	release  chan struct{}
}

func (origin *receiptBlockingOrigin) RoundTrip(request *http.Request) (*http.Response, error) {
	origin.mu.Lock()
	origin.requests++
	origin.mu.Unlock()
	origin.started <- struct{}{}
	select {
	case <-origin.release:
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}
	body := "vendor-ok"

	return &http.Response{
		StatusCode: http.StatusOK, Status: "200 OK", Proto: "HTTP/1.1",
		ProtoMajor: 1, ProtoMinor: 1,
		Header: http.Header{"Cache-Control": {"no-store"}},
		Body:   io.NopCloser(bytes.NewBufferString(body)), ContentLength: int64(len(body)),
		Request: request,
	}, nil
}

func (origin *receiptBlockingOrigin) calls() int {
	origin.mu.Lock()
	defer origin.mu.Unlock()

	return origin.requests
}

func executeBypassingCache(client *httpclient.Client) error {
	ctx, err := httpclient.WithCacheMode(context.Background(), httpclient.CacheModeBypass)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://vendor.example.test/resource", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	_, readErr := io.ReadAll(response.Body)

	return errors.Join(readErr, response.Body.Close())
}
