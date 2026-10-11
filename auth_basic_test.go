package httpclient

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestBasicAuthRejectsASCIIControls(t *testing.T) {
	t.Parallel()

	constructors := []struct {
		name string
		new  func(string, string) (RequestEditor, error)
	}{
		{"default", NewBasicAuth},
		{"finite policy", func(username, password string) (RequestEditor, error) {
			return NewBasicAuthWithPolicy(username, password, CredentialPolicy{MaximumInputBytes: 128, MaximumEncodedBytes: 256})
		}},
	}
	for _, constructor := range constructors {
		t.Run(constructor.name, func(t *testing.T) {
			for control := byte(0); control <= 0x7f; control++ {
				if control >= 0x20 && control != 0x7f {
					continue
				}
				for _, field := range []string{"username", "password"} {
					t.Run(fmt.Sprintf("%s/%02x", field, control), func(t *testing.T) {
						username, password := "test-user", "test-password"
						credential := "unrendered-credential-" + string(control)
						if field == "username" {
							username = credential
						} else {
							password = credential
						}
						editor, err := constructor.new(username, password)
						if editor != nil || !errors.Is(err, ErrInvalidAuthentication) {
							t.Fatal("Basic authentication admitted an ASCII control character")
						}
						if strings.Contains(err.Error(), credential) {
							t.Fatal("authentication error rendered the rejected credential")
						}
					})
				}
			}
		})
	}
}

func TestBasicAuthPreservesValidCredentialWireValues(t *testing.T) {
	t.Parallel()

	for _, credential := range []struct {
		name, username, password string
	}{
		{"ASCII boundary", " !~", " !~:password"},
		{"UTF-8", "käyttäjä", "密碼"},
		{"non-ASCII control scope", "user\u0085", "password\u009f"},
		{"empty password", "user", ""},
	} {
		t.Run(credential.name, func(t *testing.T) {
			for _, policy := range []CredentialPolicy{{}, {MaximumInputBytes: 128, MaximumEncodedBytes: 256}} {
				editor, err := NewBasicAuthWithPolicy(credential.username, credential.password, policy)
				if err != nil {
					t.Fatal("valid Basic credentials were rejected")
				}
				request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := editor.EditRequest(request); err != nil {
					t.Fatal("valid Basic editor failed to apply")
				}
				username, password, ok := request.BasicAuth()
				if !ok || username != credential.username || password != credential.password {
					t.Fatal("Basic editor changed accepted credential wire values")
				}
			}
		})
	}
}
