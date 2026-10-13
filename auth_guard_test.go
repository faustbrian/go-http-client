package httpclient

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// authenticationGuardContext overrides only an existing paired scope. It does
// not fabricate a key: lookup still goes through the originating operation.
type authenticationGuardContext struct {
	context.Context
	allowInsecure bool
}

func (guard authenticationGuardContext) Value(key any) any {
	value := guard.Context.Value(key)
	if scope, ok := value.(authenticationScope); ok {
		scope.origins = map[string]struct{}{"http://api.example.test": {}}
		scope.allowInsecure = guard.allowInsecure
		return scope
	}
	return value
}

// TestAuthenticationAttemptHTTPSGuard uses the real operation/attempt pair to
// exercise the defensive TLS check if retained scope policy becomes inconsistent.
func TestAuthenticationAttemptHTTPSGuard(t *testing.T) {
	for _, allowInsecure := range []bool{false, true} {
		edited, transported := false, false
		middleware, err := NewAuthenticationMiddleware(AuthenticationOptions{Name: "guard"}, RequestEditorFunc(func(request *http.Request) error {
			edited = true
			request.Header.Set("Authorization", "guard-test")
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		pipeline, err := NewPipeline(middleware[1])
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequest(http.MethodGet, "https://api.example.test", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := middleware[0].around(request, func(scoped *http.Request) (*http.Response, error) {
			attempt := scoped.Clone(authenticationGuardContext{Context: scoped.Context(), allowInsecure: allowInsecure})
			attempt.URL.Scheme = "http"
			return pipeline.Execute(attempt, TransportFunc(func(sent *http.Request) (*http.Response, error) {
				transported = true
				if sent.Header.Get("Authorization") != "guard-test" {
					t.Fatal("admitted attempt did not apply credentials")
				}
				return &http.Response{StatusCode: http.StatusNoContent}, nil
			}))
		})
		if !allowInsecure {
			if response != nil || !errors.Is(err, ErrInvalidAuthentication) ||
				!strings.Contains(err.Error(), "credential origin must use HTTPS") {
				t.Fatal("paired cleartext scope did not fail specifically at the HTTPS guard")
			}
			if edited || transported {
				t.Fatal("refused cleartext attempt edited credentials or reached transport")
			}
			continue
		}
		if err != nil || response == nil || !edited || !transported || response.StatusCode != http.StatusNoContent {
			t.Fatal("explicit insecure scope did not reach credentialed transport")
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
