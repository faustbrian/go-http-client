package httpclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(main *testing.M) {
	http.DefaultTransport = TransportFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("unexpected use of process-global HTTP transport")
	})
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("unexpected use of process-global DNS resolver")
		},
	}
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		_ = os.Setenv(name, "http://127.0.0.1:1")
	}
	for _, name := range []string{"NO_PROXY", "no_proxy"} {
		_ = os.Setenv(name, "127.0.0.1,localhost,::1")
	}
	goleak.VerifyTestMain(main)
}

func receiveTestValue[Value any](t *testing.T, values <-chan Value) Value {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for test coordination")
		var zero Value
		return zero
	}
}

func closeTestSignal(signal chan struct{}) {
	select {
	case <-signal:
	default:
		close(signal)
	}
}
