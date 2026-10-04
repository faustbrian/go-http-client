package httpclient

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestCredentialAdmissionBasicBoundary(t *testing.T) {
	policy := CredentialPolicy{MaximumInputBytes: 3, MaximumEncodedBytes: 14}
	if _, err := NewBasicAuthWithPolicy("a", "bb", policy); err != nil {
		t.Fatal("inclusive valid credential was rejected")
	}
	if _, err := NewBasicAuthWithPolicy("a", "bbb", policy); !errors.Is(err, ErrInvalidAuthentication) {
		t.Fatal("one-byte-over valid credential was not rejected")
	}
}

func TestCredentialAdmissionStaticEncodedBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		limit  int
		new    func(CredentialPolicy) (RequestEditor, error)
		verify func(*http.Request) bool
	}{
		{"basic", 14, func(p CredentialPolicy) (RequestEditor, error) { return NewBasicAuthWithPolicy("a", "bb", p) }, func(r *http.Request) bool { u, p, ok := r.BasicAuth(); return ok && u == "a" && p == "bb" }},
		{"bearer", 9, func(p CredentialPolicy) (RequestEditor, error) { return NewBearerAuthWithPolicy("ab", p) }, func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer ab" }},
		{"header", 5, func(p CredentialPolicy) (RequestEditor, error) { return NewAPIKeyHeaderWithPolicy("X", "ab", p) }, func(r *http.Request) bool { return r.Header.Get("X") == "ab" }},
		{"query", 8, func(p CredentialPolicy) (RequestEditor, error) { return NewAPIKeyQueryWithPolicy("k", "é", p) }, func(r *http.Request) bool { return r.URL.Query().Get("k") == "é" && r.URL.Query().Get("keep") == "1" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := CredentialPolicy{MaximumInputBytes: 3, MaximumEncodedBytes: test.limit}
			editor, err := test.new(policy)
			if err != nil {
				t.Fatal("inclusive encoded credential was rejected")
			}
			for range 2 {
				request, err := http.NewRequest(http.MethodGet, "https://example.test/?keep=1", nil)
				if err != nil {
					t.Fatal("request setup failed")
				}
				if err := editor.EditRequest(request); err != nil || !test.verify(request) {
					t.Fatal("accepted editor did not preserve credential semantics")
				}
			}
			policy.MaximumEncodedBytes--
			if _, err := test.new(policy); !errors.Is(err, ErrInvalidAuthentication) {
				t.Fatal("one-byte-over encoded credential was accepted")
			}
			policy.MaximumEncodedBytes++
			policy.MaximumInputBytes--
			if test.name != "bearer" {
				if _, err := test.new(policy); !errors.Is(err, ErrInvalidAuthentication) {
					t.Fatal("one-byte-over raw aggregate was accepted")
				}
			}
		})
	}
	if _, err := NewBearerAuthWithPolicy("abc", CredentialPolicy{MaximumInputBytes: 2}); !errors.Is(err, ErrInvalidAuthentication) {
		t.Fatal("over-limit bearer input was accepted")
	}
}

func TestCredentialAdmissionRejectsInvalidPolicies(t *testing.T) {
	for _, policy := range []CredentialPolicy{
		{MaximumInputBytes: -1}, {MaximumInputBytes: 1<<20 + 1},
		{MaximumEncodedBytes: -1}, {MaximumEncodedBytes: 4<<20 + 1},
		{MaximumItems: -1}, {MaximumItems: 4097},
	} {
		constructors := []func() error{
			func() error { _, e := NewBasicAuthWithPolicy("a", "b", policy); return e },
			func() error { _, e := NewBearerAuthWithPolicy("a", policy); return e },
			func() error { _, e := NewAPIKeyHeaderWithPolicy("X", "a", policy); return e },
			func() error { _, e := NewAPIKeyQueryWithPolicy("k", "a", policy); return e },
			func() error { _, e := NewHMACAuth(HMACOptions{CredentialPolicy: policy}); return e },
			func() error {
				_, e := NewContextOAuth2AuthWithPolicy(ContextTokenSourceFunc(func(context.Context) (*oauth2.Token, error) { return nil, nil }), policy)
				return e
			},
			func() error {
				_, e := NewOAuth2AuthWithPolicy(credentialAdmissionSource(func() (*oauth2.Token, error) { return nil, nil }), policy)
				return e
			},
			func() error {
				_, e := NewClientCredentialsTokenSource(ClientCredentialsOptions{CredentialPolicy: policy})
				return e
			},
			func() error { _, e := NewCachedTokenSource(TokenCacheOptions{CredentialPolicy: policy}); return e },
		}
		for _, constructor := range constructors {
			if !errors.Is(constructor(), ErrInvalidAuthentication) {
				t.Fatal("invalid finite policy was accepted")
			}
		}
	}
}

func TestCredentialAdmissionHMACCopiesAdmittedSecret(t *testing.T) {
	secret := []byte("ab")
	expected := hmac.New(sha256.New, secret)
	_, _ = expected.Write([]byte("m"))
	options := HMACOptions{
		CredentialPolicy: CredentialPolicy{MaximumInputBytes: 2},
		Secret:           secret, NewHash: sha256.New,
		Canonicalize: func(*http.Request) ([]byte, error) { return []byte("m"), nil },
		ApplySignature: func(_ *http.Request, signature []byte) error {
			if !bytes.Equal(signature, expected.Sum(nil)) {
				t.Error("HMAC secret copy or signature changed")
			}
			return nil
		},
	}
	editor, err := NewHMACAuth(options)
	if err != nil {
		t.Fatal("inclusive HMAC secret was rejected")
	}
	secret[0] = 'c'
	request, _ := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err := editor.EditRequest(request); err != nil {
		t.Fatal("HMAC editor failed")
	}
	options.Secret = []byte("abc")
	if _, err := NewHMACAuth(options); !errors.Is(err, ErrInvalidAuthentication) {
		t.Fatal("over-limit HMAC secret was accepted")
	}
}

func TestCredentialAdmissionOAuthEditorBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		token  oauth2.Token
		policy CredentialPolicy
	}{
		{"raw access", oauth2.Token{AccessToken: "ab", TokenType: "Bearer"}, CredentialPolicy{MaximumInputBytes: 8, MaximumEncodedBytes: 9}},
		{"raw refresh", oauth2.Token{AccessToken: "a", RefreshToken: "bc"}, CredentialPolicy{MaximumInputBytes: 3, MaximumEncodedBytes: 8}},
		{"encoded", oauth2.Token{AccessToken: "ab"}, CredentialPolicy{MaximumInputBytes: 2, MaximumEncodedBytes: 9}},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := ContextTokenSourceFunc(func(context.Context) (*oauth2.Token, error) { copy := test.token; return &copy, nil })
			editor, err := NewContextOAuth2AuthWithPolicy(source, test.policy)
			if err != nil {
				t.Fatal("editor setup failed")
			}
			request, _ := http.NewRequest(http.MethodGet, "https://example.test", nil)
			if err := editor.EditRequest(request); err != nil {
				t.Fatal("inclusive token was rejected")
			}
			if request.Header.Get("Authorization") != test.token.Type()+" "+test.token.AccessToken {
				t.Fatal("token application changed")
			}
			if test.name == "encoded" {
				test.policy.MaximumEncodedBytes--
			} else {
				test.policy.MaximumInputBytes--
			}
			editor, err = NewContextOAuth2AuthWithPolicy(source, test.policy)
			if err != nil {
				t.Fatal("editor setup failed")
			}
			request.Header.Set("Authorization", "original")
			if !errors.Is(editor.EditRequest(request), ErrInvalidOAuth2Token) || request.Header.Get("Authorization") != "original" {
				t.Fatal("over-limit token rejection modified original request")
			}
		})
	}
}

type credentialAdmissionSource func() (*oauth2.Token, error)

func (source credentialAdmissionSource) Token() (*oauth2.Token, error) { return source() }

func TestCredentialAdmissionOAuthReuseRejectsBeforeRetention(t *testing.T) {
	calls := 0
	source := credentialAdmissionSource(func() (*oauth2.Token, error) {
		calls++
		if calls == 1 {
			return &oauth2.Token{AccessToken: "abc"}, nil
		}
		return &oauth2.Token{AccessToken: "ab"}, nil
	})
	editor, err := NewOAuth2AuthWithPolicy(source, CredentialPolicy{MaximumInputBytes: 2, MaximumEncodedBytes: 9})
	if err != nil {
		t.Fatal("reuse editor setup failed")
	}
	request, _ := http.NewRequest(http.MethodGet, "https://example.test", nil)
	request.Header.Set("Authorization", "original")
	if !errors.Is(editor.EditRequest(request), ErrInvalidOAuth2Token) || request.Header.Get("Authorization") != "original" {
		t.Fatal("over-limit reuse token was applied")
	}
	for range 2 {
		if err := editor.EditRequest(request); err != nil {
			t.Fatal("valid replacement token was not reused")
		}
	}
	if calls != 2 {
		t.Fatal("rejected token was retained or valid token not reused")
	}
}

func TestCredentialAdmissionCacheRejectsBeforeRetention(t *testing.T) {
	client, err := New(Config{})
	if err != nil {
		t.Fatal("client setup failed")
	}
	defer client.Close()
	calls := 0
	extra := map[string]interface{}{"note": "x"}
	source, err := NewCachedTokenSource(TokenCacheOptions{
		Client: client, CredentialPolicy: CredentialPolicy{MaximumInputBytes: 3, MaximumEncodedBytes: 8},
		Source: ContextTokenSourceFunc(func(context.Context) (*oauth2.Token, error) {
			calls++
			if calls == 1 {
				return &oauth2.Token{AccessToken: "a", RefreshToken: "bcd"}, nil
			}
			return (&oauth2.Token{AccessToken: "a", RefreshToken: "bc"}).WithExtra(extra), nil
		}),
	})
	if err != nil {
		t.Fatal("cache setup failed")
	}
	if token, err := source.Token(context.Background()); token != nil || !errors.Is(err, ErrInvalidOAuth2Token) {
		t.Fatal("over-limit token was returned or retained")
	}
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatal("inclusive cache token was rejected")
	}
	token.AccessToken = "changed"
	extra["note"] = "y"
	token, err = source.Token(context.Background())
	if err != nil || calls != 2 || token.AccessToken != "a" || token.Extra("note") != "y" {
		t.Fatal("credential struct copy or existing opaque metadata semantics changed")
	}
	if source.Invalidate("bb") || !source.Invalidate("a") || source.Invalidate("a") {
		t.Fatal("exact invalidation semantics changed")
	}
}

func TestCredentialAdmissionCacheEncodedBoundary(t *testing.T) {
	client, err := New(Config{})
	if err != nil {
		t.Fatal("client setup failed")
	}
	defer client.Close()
	for _, maximum := range []int{8, 7} {
		source, err := NewCachedTokenSource(TokenCacheOptions{
			Client: client, CredentialPolicy: CredentialPolicy{MaximumInputBytes: 1, MaximumEncodedBytes: maximum},
			Source: ContextTokenSourceFunc(func(context.Context) (*oauth2.Token, error) { return &oauth2.Token{AccessToken: "a"}, nil }),
		})
		if err != nil {
			t.Fatal("cache setup failed")
		}
		token, err := source.Token(context.Background())
		if maximum == 8 {
			if err != nil || token.AccessToken != "a" {
				t.Fatal("inclusive encoded cache token was rejected")
			}
		} else if token != nil || !errors.Is(err, ErrInvalidOAuth2Token) {
			t.Fatal("one-byte-over encoded cache token was admitted")
		}
	}
}

func TestCredentialAdmissionClientCredentialsCollections(t *testing.T) {
	client, err := New(Config{})
	if err != nil {
		t.Fatal("client setup failed")
	}
	defer client.Close()
	base := ClientCredentialsOptions{
		Client: client, TokenURL: "https://t.test", ClientID: "a", ClientSecret: "b",
		Scopes: []string{"r"}, EndpointParams: url.Values{"p": {"v"}},
		CredentialPolicy: CredentialPolicy{MaximumInputBytes: len("https://t.test") + 5, MaximumEncodedBytes: 41, MaximumItems: 3},
	}
	if _, err := NewClientCredentialsTokenSource(base); err != nil {
		t.Fatal("inclusive client credentials collections were rejected")
	}
	for _, test := range []struct {
		name   string
		change func(*ClientCredentialsOptions)
	}{
		{"raw", func(o *ClientCredentialsOptions) { o.ClientSecret += "c" }},
		{"counts", func(o *ClientCredentialsOptions) { o.CredentialPolicy.MaximumItems-- }},
		{"scope count", func(o *ClientCredentialsOptions) { o.Scopes = []string{"r", "s"} }},
		{"parameter count", func(o *ClientCredentialsOptions) { o.EndpointParams = url.Values{"p": {"v"}, "q": {}} }},
		{"value count", func(o *ClientCredentialsOptions) { o.EndpointParams = url.Values{"p": {"v", "w"}} }},
		{"encoded", func(o *ClientCredentialsOptions) { o.CredentialPolicy.MaximumEncodedBytes = 40 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := base
			test.change(&options)
			if _, err := NewClientCredentialsTokenSource(options); !errors.Is(err, ErrInvalidAuthentication) {
				t.Fatal("over-limit client credential policy was accepted")
			}
			if _, err := NewClientCredentialsTokenSource(options); strings.Contains(err.Error(), options.TokenURL) {
				t.Fatal("rejection exposed endpoint data")
			}
		})
	}
	// Escaping and Basic encoding are checked independently of the form body.
	base.Scopes, base.EndpointParams = nil, nil
	base.ClientID, base.ClientSecret = "a+", "b"
	base.CredentialPolicy = CredentialPolicy{MaximumInputBytes: len(base.TokenURL) + 3, MaximumEncodedBytes: 29}
	if _, err := NewClientCredentialsTokenSource(base); err != nil {
		t.Fatal("inclusive header client authentication was rejected")
	}
	base.AuthStyle = oauth2.AuthStyleInParams
	base.CredentialPolicy.MaximumEncodedBytes = 60
	if _, err := NewClientCredentialsTokenSource(base); err != nil {
		t.Fatal("inclusive parameter client authentication was rejected")
	}
	base.CredentialPolicy.MaximumEncodedBytes--
	if _, err := NewClientCredentialsTokenSource(base); !errors.Is(err, ErrInvalidAuthentication) {
		t.Fatal("over-limit encoded client parameter body was accepted")
	}
	base.AuthStyle = oauth2.AuthStyleInHeader
	base.ClientID, base.ClientSecret = "+++++", "b"
	base.CredentialPolicy = CredentialPolicy{MaximumInputBytes: len(base.TokenURL) + 6, MaximumEncodedBytes: 30}
	if _, err := NewClientCredentialsTokenSource(base); err != nil {
		t.Fatal("inclusive encoded client header was rejected")
	}
	base.CredentialPolicy.MaximumEncodedBytes--
	if _, err := NewClientCredentialsTokenSource(base); !errors.Is(err, ErrInvalidAuthentication) {
		t.Fatal("over-limit encoded client header was accepted")
	}
}

func TestCredentialAdmissionClientCredentialsReturnedToken(t *testing.T) {
	for _, boundary := range []string{"raw", "encoded"} {
		t.Run(boundary, func(t *testing.T) {
			calls := 0
			client, err := New(Config{Transport: TransportFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.ContentLength != 29 {
					t.Fatal("bounded token request body changed")
				}
				response := `{"access_token":"a","token_type":"Bearer","refresh_token":"b"}`
				if calls == 1 {
					if boundary == "raw" {
						response = `{"access_token":"a","token_type":"Bearer","refresh_token":"` + strings.Repeat("b", 26) + `"}`
					} else {
						response = `{"access_token":"` + strings.Repeat("a", 23) + `","token_type":"Bearer"}`
					}
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(response)), Request: request}, nil
			})})
			if err != nil {
				t.Fatal("in-memory client setup failed")
			}
			defer client.Close()
			source, err := NewClientCredentialsTokenSource(ClientCredentialsOptions{
				Client: client, TokenURL: "https://t.test", ClientID: "a", ClientSecret: "b",
				CredentialPolicy: CredentialPolicy{MaximumInputBytes: 32, MaximumEncodedBytes: 29},
			})
			if err != nil {
				t.Fatal("client credential source setup failed")
			}
			if token, err := source.Token(context.Background()); token != nil || !errors.Is(err, ErrInvalidOAuth2Token) {
				t.Fatal("over-limit endpoint token was admitted")
			}
			for range 2 {
				token, err := source.Token(context.Background())
				if err != nil || token.AccessToken != "a" {
					t.Fatal("valid replacement endpoint token was not cached")
				}
			}
			if calls != 2 {
				t.Fatal("rejected endpoint token was retained or valid replacement not cached")
			}
		})
	}
}

func TestCredentialAdmissionClientCredentialsIndependentBudgets(t *testing.T) {
	client, err := New(Config{})
	if err != nil {
		t.Fatal("client setup failed")
	}
	defer client.Close()
	base := ClientCredentialsOptions{
		Client: client, TokenURL: "https://t.test", ClientID: "a", ClientSecret: "b",
	}
	baseBytes := len(base.TokenURL) + len(base.ClientID) + len(base.ClientSecret)
	for _, test := range []struct {
		name       string
		scopes     []string
		parameters url.Values
		accepted   CredentialPolicy
		rejected   CredentialPolicy
	}{
		{
			name: "aggregate scope bytes", scopes: []string{"r", "s"},
			accepted: CredentialPolicy{MaximumInputBytes: baseBytes + 2},
			rejected: CredentialPolicy{MaximumInputBytes: baseBytes + 1},
		},
		{
			name: "parameter name bytes", parameters: url.Values{"pq": {}},
			accepted: CredentialPolicy{MaximumInputBytes: baseBytes + 2},
			rejected: CredentialPolicy{MaximumInputBytes: baseBytes + 1},
		},
		{
			name: "parameter value bytes", parameters: url.Values{"p": {"vw"}},
			accepted: CredentialPolicy{MaximumInputBytes: baseBytes + 3},
			rejected: CredentialPolicy{MaximumInputBytes: baseBytes + 2},
		},
		{
			name: "scope count", scopes: []string{"r", "s"},
			accepted: CredentialPolicy{MaximumItems: 2},
			rejected: CredentialPolicy{MaximumItems: 1},
		},
		{
			name: "parameter name count", parameters: url.Values{"p": {}, "q": {}},
			accepted: CredentialPolicy{MaximumItems: 2},
			rejected: CredentialPolicy{MaximumItems: 1},
		},
		{
			name: "aggregate name and scope count", scopes: []string{"r"}, parameters: url.Values{"p": {}},
			accepted: CredentialPolicy{MaximumItems: 2},
			rejected: CredentialPolicy{MaximumItems: 1},
		},
		{
			name:     "required grant syntax",
			accepted: CredentialPolicy{MaximumEncodedBytes: 29},
			rejected: CredentialPolicy{MaximumEncodedBytes: 28},
		},
		{
			name: "required scope syntax", scopes: []string{"r"},
			accepted: CredentialPolicy{MaximumEncodedBytes: 37},
			rejected: CredentialPolicy{MaximumEncodedBytes: 35},
		},
		{
			name: "scope value bytes", scopes: []string{"r"},
			accepted: CredentialPolicy{MaximumEncodedBytes: 37},
			rejected: CredentialPolicy{MaximumEncodedBytes: 36},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := base
			options.Scopes, options.EndpointParams = test.scopes, test.parameters
			options.CredentialPolicy = test.accepted
			if source, err := NewClientCredentialsTokenSource(options); err != nil || source == nil {
				t.Fatal("valid credential configuration within isolated budget was rejected")
			}
			options.CredentialPolicy = test.rejected
			source, err := NewClientCredentialsTokenSource(options)
			if source != nil || !errors.Is(err, ErrInvalidAuthentication) {
				t.Fatal("valid credential configuration exceeding isolated budget was admitted")
			}
			if strings.Contains(err.Error(), options.TokenURL) {
				t.Fatal("admission rejection exposed endpoint data")
			}
		})
	}
}
