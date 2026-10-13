package httpclient

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRequestSpecRejectsFragments(t *testing.T) {
	for _, input := range []struct{ base, reference string }{
		{"https://api.example.test/#secret", "child"},
		{"https://api.example.test/", "child#secret"},
		{"https://api.example.test/", "#secret"},
		{"https://api.example.test/#", "child"},
		{"https://api.example.test/", "child#"},
	} {
		if _, err := NewRequestSpec(input.base, input.reference); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("fragment-bearing input was admitted: %v", err)
		}
	}
	// Escaped path/query octets are data, not a URI fragment.
	spec, err := NewRequestSpec("https://api.example.test/", "child%23data?q=%23data")
	if err != nil {
		t.Fatal(err)
	}
	request, err := spec.Build(context.Background(), http.MethodGet)
	if err != nil || request.URL.Fragment != "" || request.URL.EscapedPath() != "/child%23data" {
		t.Fatalf("escaped hash behavior changed: request=%v err=%v", request, err)
	}
}

func TestCompressionRejectsNonfiniteRatios(t *testing.T) {
	for _, ratio := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := NewCompressionMiddleware(CompressionOptions{Name: "compression", MaximumExpansionRatio: ratio}); !errors.Is(err, ErrInvalidCompression) {
			t.Errorf("nonfinite ratio was admitted: %v", err)
		}
	}
	for _, ratio := range []float64{0, 1, maximumExpansionRatio} {
		if _, err := NewCompressionMiddleware(CompressionOptions{Name: "compression", MaximumExpansionRatio: ratio}); err != nil {
			t.Errorf("valid ratio %v rejected: %v", ratio, err)
		}
	}
}

func TestFileTransferChmodFailureClosesResponse(t *testing.T) {
	modeFailure, bodyFailure := errors.New("mode failure"), errors.New("body close failure")
	for _, closeErr := range []error{nil, bodyFailure} {
		var closed atomic.Int64
		body := &responseTestBody{Reader: strings.NewReader("body"), closed: &closed, closeErr: closeErr}
		file := &fakeTransferFile{chmodErr: modeFailure}
		fs := &fakeFileTransferFS{file: file}
		response := &http.Response{StatusCode: http.StatusOK, Body: body, ContentLength: -1}
		_, err := copyResponseToFile(context.Background(), response, "destination", FileTransferOptions{}, fs)
		if !errors.Is(err, modeFailure) || closed.Load() != 1 || file.closeCalls != 1 || fs.removeCalls != 1 {
			t.Errorf("mode failure cleanup: error=%v body closes=%d file closes=%d removes=%d", err, closed.Load(), file.closeCalls, fs.removeCalls)
		}
		if closeErr != nil && !errors.Is(err, closeErr) {
			t.Errorf("body close failure was lost: %v", err)
		}
	}
}

func TestAuthenticationPairsOwnIndependentOperationState(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		var all []Middleware
		for order := range 2 {
			index := order
			if reverse {
				index = 1 - index
			}
			name, origin, header := "alpha", "https://alpha.example.test", "X-Alpha"
			if index == 1 {
				name, origin, header = "beta", "https://beta.example.test", "X-Beta"
			}
			editor, err := NewAPIKeyHeader(header, "credential")
			if err != nil {
				t.Fatal(err)
			}
			pair, err := NewAuthenticationMiddleware(AuthenticationOptions{Name: name, Priority: order, AllowedOrigins: []string{origin}, SensitiveHeaders: []string{header}}, editor)
			if err != nil {
				t.Fatal(err)
			}
			all = append(all, pair...)
		}
		pipeline, err := NewPipeline(all...)
		if err != nil {
			t.Fatal(err)
		}
		for _, origin := range []string{"alpha", "beta", "neither"} {
			request, err := http.NewRequest(http.MethodGet, "https://"+origin+".example.test/resource", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("X-Alpha", "stale")
			request.Header.Set("X-Beta", "stale")
			response, err := pipeline.Execute(request, TransportFunc(func(attempt *http.Request) (*http.Response, error) {
				for name, trusted := range map[string]bool{"X-Alpha": origin == "alpha", "X-Beta": origin == "beta"} {
					want := ""
					if trusted {
						want = "credential"
					}
					if got := attempt.Header.Get(name); got != want {
						t.Errorf("%s at %s origin=%q, want %q", name, origin, got, want)
					}
				}
				return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// Link pagination preserves RFC Link continuations for its borrowed fetcher;
// RequestSpec admission is a separate, intentionally stricter public contract.
func TestLinkPaginatorPreservesFragmentContinuations(t *testing.T) {
	for _, initial := range []string{"https://api.example.test/start", "https://api.example.test/start#section"} {
		var fetched []string
		paginator, err := NewLinkPaginator(LinkPaginationOptions[string]{
			InitialURL: initial,
			Fetch: func(_ context.Context, reference string) (LinkPaginationPage[string], error) {
				fetched = append(fetched, reference)
				if len(fetched) == 1 {
					return LinkPaginationPage[string]{Items: []string{"first"}, Link: `<next#section>; rel="next"`}, nil
				}
				return LinkPaginationPage[string]{Items: []string{"second"}}, nil
			},
		})
		if err != nil {
			t.Fatalf("admitted fragment continuation failed construction: %v", err)
		}
		items := collectPaginator(t, paginator)
		if !reflect.DeepEqual(items, []string{"first", "second"}) || !reflect.DeepEqual(fetched, []string{initial, "https://api.example.test/next#section"}) {
			t.Fatalf("fragment continuation changed: items=%v fetched=%v", items, fetched)
		}
	}
}
