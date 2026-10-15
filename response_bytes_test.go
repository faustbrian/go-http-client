package httpclient

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReadResponseBytesPreservesRawPayloadAndCloses(t *testing.T) {
	t.Parallel()
	payload := []byte{0, 0xff, '\n', '{', '}'}
	for _, variant := range []string{"ordinary", "empty", "missing-media", "invalid-media", "declared-length", "semantic-empty"} {
		t.Run(variant, func(t *testing.T) {
			var closed atomic.Int64
			want := payload
			if variant == "empty" {
				want = nil
			}
			response := &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": {"text/plain"}},
				Body:       &responseTestBody{Reader: bytes.NewReader(want), closed: &closed},
			}
			switch variant {
			case "missing-media":
				response.Header.Del("Content-Type")
			case "invalid-media":
				response.Header.Set("Content-Type", "not a media type")
			case "declared-length":
				response.ContentLength = 1
				response.Header.Set("Content-Length", "1")
			case "semantic-empty":
				response.StatusCode = http.StatusNoContent
				response.Request = &http.Request{Method: http.MethodHead}
			}
			body, err := ReadResponseBytes(response, ReadOptions{MaximumBodyBytes: 8})
			if err != nil || !bytes.Equal(body, want) {
				t.Fatalf("raw response = %x, %v; want %x", body, err, want)
			}
			if closed.Load() != 1 {
				t.Fatalf("body closed %d times, want once", closed.Load())
			}
		})
	}
}

func TestReadResponseBytesEnforcesInclusiveBoundAndOneByteProbe(t *testing.T) {
	t.Parallel()
	for _, size := range []int{7, 8, 9, 32} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			var closed atomic.Int64
			reader := &rawByteCounter{Reader: strings.NewReader(strings.Repeat("x", size))}
			response := &http.Response{Body: &responseTestBody{Reader: reader, closed: &closed}}
			body, err := ReadResponseBytes(response, ReadOptions{MaximumBodyBytes: 8})
			if size <= 8 {
				if err != nil || len(body) != size {
					t.Fatalf("size %d: bytes=%d error=%v", size, len(body), err)
				}
			} else {
				var limit *ResponseLimitError
				if !errors.Is(err, ErrResponseBodyLimit) || !errors.As(err, &limit) || limit.Limit != 8 || body != nil {
					t.Fatalf("size %d: partial data or wrong limit error: bytes=%d error=%v", size, len(body), err)
				}
			}
			if reader.read != min(size, 9) || closed.Load() != 1 {
				t.Fatalf("size %d: consumed=%d closes=%d", size, reader.read, closed.Load())
			}
		})
	}
}

func TestReadResponseBytesSeparatesReadAndClosePolicy(t *testing.T) {
	t.Parallel()
	readFailure := errors.New("private-reader-canary")
	closeFailure := errors.New("private-close-canary")
	for _, test := range []struct {
		name        string
		readFailure bool
		ignoreClose bool
	}{
		{name: "close-reported"},
		{name: "close-ignored", ignoreClose: true},
		{name: "both-reported", readFailure: true},
		{name: "read-never-ignored", readFailure: true, ignoreClose: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var closed atomic.Int64
			var reader io.Reader = strings.NewReader("partial")
			if test.readFailure {
				reader = io.MultiReader(reader, &responseErrorReader{err: readFailure})
			}
			response := &http.Response{Body: &responseTestBody{Reader: reader, closed: &closed, closeErr: closeFailure}}
			body, err := ReadResponseBytes(response, ReadOptions{MaximumBodyBytes: 64, IgnoreCloseError: test.ignoreClose})
			if errors.Is(err, readFailure) != test.readFailure || errors.Is(err, closeFailure) != !test.ignoreClose {
				t.Fatalf("wrong retained error causes: %v", err)
			}
			if test.readFailure && body != nil || !test.readFailure && string(body) != "partial" {
				t.Fatalf("partial output policy: %q", body)
			}
			if err != nil && strings.Contains(err.Error(), "private-") || closed.Load() != 1 {
				t.Fatalf("rendered private cause or wrong close count: %v, %d", err, closed.Load())
			}
		})
	}
}

func TestReadResponseBytesValidatesLimitsAndNilBodies(t *testing.T) {
	t.Parallel()
	var typedNil *responseTestBody
	for _, response := range []*http.Response{nil, {}, {Body: typedNil}} {
		if body, err := ReadResponseBytes(response, ReadOptions{}); !errors.Is(err, ErrInvalidResponsePolicy) || body != nil {
			t.Fatalf("nil input: bytes=%d error=%v", len(body), err)
		}
	}
	for _, maximum := range []int64{-1, (1 << 30) + 1} {
		var closed atomic.Int64
		reader := &rawByteCounter{Reader: strings.NewReader("never read")}
		response := &http.Response{Body: &responseTestBody{Reader: reader, closed: &closed}}
		if _, err := ReadResponseBytes(response, ReadOptions{MaximumBodyBytes: maximum}); !errors.Is(err, ErrInvalidResponsePolicy) || reader.read != 0 || closed.Load() != 1 {
			t.Fatalf("invalid bound %d: consumed=%d closes=%d error=%v", maximum, reader.read, closed.Load(), err)
		}
	}
	response := &http.Response{Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (8<<20)+1)))}
	if body, err := ReadResponseBytes(response, ReadOptions{}); !errors.Is(err, ErrResponseBodyLimit) || body != nil {
		t.Fatalf("default bound: bytes=%d error=%v", len(body), err)
	}
}

func TestReadResponseBytesNeverRendersReaderWrappedLimit(t *testing.T) {
	t.Parallel()
	cause := errors.Join(errors.New("private-reader-canary"), ErrResponseBodyLimit)
	var closed atomic.Int64
	response := &http.Response{Body: &responseTestBody{
		Reader: &responseErrorReader{err: cause}, closed: &closed,
	}}
	body, err := ReadResponseBytes(response, ReadOptions{})
	if !errors.Is(err, cause) || !errors.Is(err, ErrResponseBodyLimit) || body != nil || closed.Load() != 1 {
		t.Fatalf("reader error lost identity, partial bytes or ownership: bytes=%d closes=%d error=%v", len(body), closed.Load(), err)
	}
	if strings.Contains(err.Error(), "private-reader-canary") {
		t.Fatal("raw reader limit cause was rendered")
	}
}

func TestDecodeResponseNeverRendersReaderWrappedLimit(t *testing.T) {
	t.Parallel()
	for name, decode := range map[string]func(*http.Response) error{
		"custom": func(response *http.Response) error {
			_, err := DecodeResponse(response, DecodeOptions{ExpectedMediaTypes: []string{"application/json"}}, io.ReadAll)
			return err
		},
		"json": func(response *http.Response) error {
			_, err := DecodeJSONResponse[any](response, DecodeOptions{})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			cause := errors.Join(errors.New("private-reader-canary"), ErrResponseBodyLimit)
			var closed atomic.Int64
			response := &http.Response{
				Header: http.Header{"Content-Type": {"application/json"}},
				Body:   &responseTestBody{Reader: &responseErrorReader{err: cause}, closed: &closed},
			}
			err := decode(response)
			if !errors.Is(err, cause) || !errors.Is(err, ErrResponseBodyLimit) || closed.Load() != 1 {
				t.Fatalf("decoder lost reader identity or ownership: closes=%d error=%v", closed.Load(), err)
			}
			if strings.Contains(err.Error(), "private-reader-canary") {
				t.Fatal("decoder reader limit cause was rendered")
			}
		})
	}
}

func TestReadResponseBytesRetainsAdmittedBodyOwnership(t *testing.T) {
	t.Parallel()
	var originalClosed, replacementClosed atomic.Int64
	response := &http.Response{}
	replacement := &responseTestBody{Reader: strings.NewReader("replacement"), closed: &replacementClosed}
	t.Cleanup(func() { _ = replacement.Close() })
	reader := strings.NewReader("original")
	response.Body = &responseTestBody{closed: &originalClosed, Reader: rawReadFunc(func(buffer []byte) (int, error) {
		response.Body = replacement
		return reader.Read(buffer)
	})}
	body, err := ReadResponseBytes(response, ReadOptions{})
	if err != nil || string(body) != "original" || originalClosed.Load() != 1 || replacementClosed.Load() != 0 {
		t.Fatalf("body ownership changed: bytes=%q error=%v original=%d replacement=%d", body, err, originalClosed.Load(), replacementClosed.Load())
	}
}

type rawReadFunc func([]byte) (int, error)

func (read rawReadFunc) Read(buffer []byte) (int, error) { return read(buffer) }

type rawByteCounter struct {
	io.Reader
	read int
}

func (reader *rawByteCounter) Read(buffer []byte) (int, error) {
	n, err := reader.Reader.Read(buffer)
	reader.read += n
	return n, err
}
