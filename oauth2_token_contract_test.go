package httpclient

import "testing"

func TestContextOAuth2TokenValidatorRejectsNil(t *testing.T) {
	// This is the helper's standalone defensive contract. Public editors
	// reject nil tokens earlier through CredentialPolicy.token.
	if validContextOAuth2Token(nil, nil) {
		t.Fatal("standalone token validator must reject a nil token")
	}
}
