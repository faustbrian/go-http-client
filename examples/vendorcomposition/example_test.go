package vendorcomposition_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	httpclient "github.com/faustbrian/go-http-client"
)

// Example demonstrates the supported vendor boundary: the application owns
// DTOs and endpoint identity while http-client owns retry, rate, breaker,
// cache, telemetry, and transport policy. The bulkhead is an application
// adapter at the public middleware seam.
func Example_vendorComposition() {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=60")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "vendor-ok")
	}))
	defer server.Close()

	cache, err := httpclient.NewMemoryCache(httpclient.MemoryCacheOptions{MaximumEntries: 8, MaximumBytes: 1 << 20})
	if err != nil {
		panic(err)
	}
	cacheMiddleware, err := httpclient.NewCacheMiddleware(httpclient.CacheOptions{
		Name: "vendor-cache", Store: cache, Methods: []string{http.MethodGet},
		Statuses: []int{http.StatusOK}, VariantKey: []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		panic(err)
	}
	rateMiddleware, err := httpclient.NewRateLimitMiddleware(httpclient.RateLimitOptions{Name: "vendor-rate", Limiter: &limiter{}})
	if err != nil {
		panic(err)
	}
	breakerMiddleware, err := httpclient.NewCircuitBreakerMiddleware(httpclient.CircuitBreakerOptions{Name: "vendor-breaker", Breaker: breaker{}})
	if err != nil {
		panic(err)
	}
	retryMiddleware, err := httpclient.NewRetryMiddleware(httpclient.RetryOptions{Name: "vendor-retry", MaximumAttempts: 2, MaximumElapsed: time.Second})
	if err != nil {
		panic(err)
	}
	bulkheadMiddleware, err := httpclient.NewTransportMiddleware(httpclient.MiddlewareOptions{Name: "vendor-bulkhead", Scope: httpclient.ScopeAttempt, Layer: httpclient.MiddlewareEndpoint}, (&bulkhead{slots: make(chan struct{}, 2)}).around)
	if err != nil {
		panic(err)
	}
	client, err := httpclient.New(httpclient.Config{Transport: server.Client().Transport, Telemetry: &httpclient.TelemetryOptions{Observer: &observer{}}, Middleware: append(append(rateMiddleware, breakerMiddleware, retryMiddleware, bulkheadMiddleware), cacheMiddleware)})
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Close() }()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		panic(err)
	}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(body))
	// Output: vendor-ok
}

type limiter struct{}

func (*limiter) Acquire(context.Context, time.Duration) (time.Duration, error) { return 0, nil }
func (*limiter) DeferUntil(time.Time)                                          {}
func (*limiter) Now() time.Time                                                { return time.Now() }

type breaker struct{}

func (breaker) Execute(ctx context.Context, operation func(context.Context) (*http.Response, error)) (*http.Response, error) {
	return operation(ctx)
}

type bulkhead struct{ slots chan struct{} }

func (b *bulkhead) around(request *http.Request, next httpclient.Next) (*http.Response, error) {
	select {
	case b.slots <- struct{}{}:
		defer func() { <-b.slots }()
		return next(request)
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}
}

type observer struct{ sync.Mutex }

func (*observer) Start(ctx context.Context, _ httpclient.TelemetryEvent) context.Context { return ctx }
func (*observer) Finish(context.Context, httpclient.TelemetryEvent)                      {}
