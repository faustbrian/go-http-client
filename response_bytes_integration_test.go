package httpclient_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	httpclient "github.com/faustbrian/go-http-client/v3"
)

func TestReadResponseBytesJoinsCallerCancellationAndClosesManagedBody(t *testing.T) {
	upstreamDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, "partial")
		writer.(http.Flusher).Flush()
		<-request.Context().Done()
		close(upstreamDone)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)
	timeout := 3 * time.Second
	client, err := httpclient.New(httpclient.Config{Policy: httpclient.PolicyOverrides{OperationTimeout: &timeout}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	observed := &rawObservedBody{ReadCloser: response.Body, started: make(chan struct{})}
	response.Body = observed
	type result struct {
		body []byte
		err  error
	}
	finished := make(chan result, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		body, readErr := httpclient.ReadResponseBytes(response, httpclient.ReadOptions{MaximumBodyBytes: 64})
		finished <- result{body: body, err: readErr}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-joined:
		case <-time.After(2 * time.Second):
			t.Error("raw reader did not join after cancellation")
		}
	})
	select {
	case <-observed.started:
	case <-ctx.Done():
		t.Fatal("raw reader never entered body consumption")
	}
	cancel()
	select {
	case got := <-finished:
		if !errors.Is(got.err, context.Canceled) || got.body != nil || observed.closed.Load() != 1 {
			t.Fatalf("canceled body: bytes=%d error=%v closes=%d", len(got.body), got.err, observed.closed.Load())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled raw body remained active")
	}
	select {
	case <-upstreamDone:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled upstream remained active")
	}
}

type rawObservedBody struct {
	io.ReadCloser
	started chan struct{}
	once    sync.Once
	closed  atomic.Int64
}

func (body *rawObservedBody) Read(buffer []byte) (int, error) {
	body.once.Do(func() { close(body.started) })
	return body.ReadCloser.Read(buffer)
}

func (body *rawObservedBody) Close() error {
	body.closed.Add(1)
	return body.ReadCloser.Close()
}
