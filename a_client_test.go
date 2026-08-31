package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestNewClientProvidesFiniteSafeDefaults(t *testing.T) {
	// Keep this construction invariant serial and ahead of request tests. A
	// broken default transport can otherwise leave parallel requests blocked.
	client, err := New(Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	httpClient := client.HTTPClient()
	if defaultConnectTimeout != 10*time.Second || defaultKeepAlive != 30*time.Second {
		t.Fatalf("dial defaults = connect:%s keep-alive:%s", defaultConnectTimeout, defaultKeepAlive)
	}
	if httpClient.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %v, want 30s", httpClient.Timeout)
	}

	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", httpClient.Transport)
	}

	if transport.DialContext == nil {
		t.Fatal("DialContext is nil")
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig is nil")
	}
	if transport.TLSClientConfig.MinVersion == 0 {
		t.Fatal("TLS minimum version is not explicit")
	}
	if transport.TLSHandshakeTimeout <= 0 {
		t.Fatalf("TLSHandshakeTimeout = %v, want a finite timeout", transport.TLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout <= 0 {
		t.Fatalf("ResponseHeaderTimeout = %v, want a finite timeout", transport.ResponseHeaderTimeout)
	}
	if transport.IdleConnTimeout <= 0 {
		t.Fatalf("IdleConnTimeout = %v, want a finite timeout", transport.IdleConnTimeout)
	}
	if transport.MaxResponseHeaderBytes <= 0 {
		t.Fatalf("MaxResponseHeaderBytes = %d, want a finite limit", transport.MaxResponseHeaderBytes)
	}
	if !transport.ForceAttemptHTTP2 {
		t.Fatal("ForceAttemptHTTP2 = false, want true")
	}
}

func TestRetryStopsAtMaximumAttempts(t *testing.T) {
	clock := &retryTestClock{now: time.Unix(1_700_000_000, 0)}
	retry, err := NewRetryMiddleware(RetryOptions{
		Name: "bounded-retry", MaximumAttempts: 2,
		BaseDelay: time.Millisecond, MaximumDelay: time.Millisecond,
		Clock:  clock,
		Policy: RetryPolicyFunc(func(RetryAttempt) bool { return true }),
	})
	if err != nil {
		t.Fatalf("construct retry middleware: %v", err)
	}

	attempts := 0
	client, err := New(Config{
		Middleware: []Middleware{retry},
		Transport: TransportFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			if attempts > 2 {
				t.Fatalf("transport attempt %d exceeded configured maximum", attempts)
			}

			return nil, errors.New("temporary failure")
		}),
	})
	if err != nil {
		t.Fatalf("construct client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	request, err := http.NewRequest(http.MethodGet, "https://api.example.test/items", nil)
	if err != nil {
		t.Fatalf("construct request: %v", err)
	}
	_, err = client.Do(request)
	var exhausted *RetryExhaustedError
	if !errors.As(err, &exhausted) {
		t.Fatalf("retry error = %#v", err)
	}
	if exhausted.Attempts != 2 || attempts != 2 {
		t.Fatalf("attempts = error:%d transport:%d, want 2", exhausted.Attempts, attempts)
	}
	if delays := clock.Delays(); len(delays) != 1 {
		t.Fatalf("retry delays = %v, want one delay", delays)
	}
}

func TestCopyResponseReadsWithPositiveBufferAndMakesProgress(t *testing.T) {
	payload := []byte("bounded transfer")
	body := &positiveReadCloser{Reader: bytes.NewReader(payload)}
	response := &http.Response{Body: body, ContentLength: int64(len(payload))}
	var destination bytes.Buffer

	result, err := CopyResponse(context.Background(), response, &destination, TransferOptions{})
	if err != nil {
		t.Fatalf("copy response: %v", err)
	}
	if result.Bytes != int64(len(payload)) || !bytes.Equal(destination.Bytes(), payload) {
		t.Fatalf("copy result = bytes:%d payload:%q", result.Bytes, destination.Bytes())
	}
	if body.emptyRead {
		t.Fatal("response body received an empty read buffer")
	}
}

type positiveReadCloser struct {
	io.Reader
	emptyRead bool
}

func (reader *positiveReadCloser) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		reader.emptyRead = true

		return 0, errors.New("empty read buffer")
	}

	return reader.Reader.Read(buffer)
}

func (*positiveReadCloser) Close() error { return nil }
