package httpclient_test

import (
	"errors"
	"net/http"
	"testing"

	httpclient "github.com/faustbrian/go-http-client/v2"
)

func TestClientCredentialsAggregateInputAdmission(t *testing.T) {
	endpointCalls := 0
	client, err := httpclient.New(httpclient.Config{
		Transport: httpclient.TransportFunc(func(*http.Request) (*http.Response, error) {
			endpointCalls++
			return nil, errors.New("unexpected token endpoint call")
		}),
	})
	if err != nil {
		t.Fatal("construct client")
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatal("close client")
		}
	})
	// Fourteen URL bytes and one byte for each credential: no collection or
	// encoded-size limit obscures this independent aggregate input boundary.
	options := httpclient.ClientCredentialsOptions{
		Client: client, TokenURL: "https://t.test", ClientID: "a", ClientSecret: "b",
		CredentialPolicy: httpclient.CredentialPolicy{MaximumInputBytes: 16},
	}
	source, err := httpclient.NewClientCredentialsTokenSource(options)
	if err != nil || source == nil {
		t.Fatal("exact aggregate input budget must admit ordinary valid options")
	}
	options.CredentialPolicy.MaximumInputBytes = 15
	source, err = httpclient.NewClientCredentialsTokenSource(options)
	if source != nil || !errors.Is(err, httpclient.ErrInvalidAuthentication) {
		t.Fatal("one-byte-over aggregate must return no source and ErrInvalidAuthentication")
	}
	if endpointCalls != 0 {
		t.Fatal("constructor admission must not call the token endpoint")
	}
}
