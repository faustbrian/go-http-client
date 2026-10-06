package httpclient

import (
	"fmt"

	"golang.org/x/oauth2"
)

// CredentialPolicy bounds package-owned credential processing. Zero fields use
// finite defaults: 64 KiB raw input, 256 KiB encoded output, and 256 collection
// items. Positive values may not exceed 1 MiB, 4 MiB, and 4096 respectively.
// Raw bytes are aggregated across each credential or token's string fields.
// Encoded bytes include credential syntax, not unrelated request data. Items
// count scopes, endpoint parameter names, and endpoint parameter values.
// Trusted callbacks, external acquisition/caches and opaque OAuth Extra data
// remain caller-owned; this is not a whole-request or transport policy.
type CredentialPolicy struct {
	MaximumInputBytes   int
	MaximumEncodedBytes int
	MaximumItems        int
}

func resolveCredentialPolicy(policy CredentialPolicy) (CredentialPolicy, error) {
	if policy.MaximumInputBytes == 0 {
		policy.MaximumInputBytes = 64 << 10
	}
	if policy.MaximumEncodedBytes == 0 {
		policy.MaximumEncodedBytes = 256 << 10
	}
	if policy.MaximumItems == 0 {
		policy.MaximumItems = 256
	}
	if policy.MaximumInputBytes < 1 || policy.MaximumInputBytes > 1<<20 ||
		policy.MaximumEncodedBytes < 1 || policy.MaximumEncodedBytes > 4<<20 ||
		policy.MaximumItems < 1 || policy.MaximumItems > 4096 {
		return CredentialPolicy{}, credentialLimitError()
	}
	return policy, nil
}

func credentialLimitError() error {
	return fmt.Errorf("%w: credential resource limit exceeded or invalid", ErrInvalidAuthentication)
}

// Subtraction avoids overflow even for lengths that have not been admitted.
func withinCredentialBudget(maximum int, lengths ...int) bool {
	for _, length := range lengths {
		if length < 0 || length > maximum {
			return false
		}
		maximum -= length
	}
	return true
}

func (policy CredentialPolicy) input(lengths ...int) error {
	if !withinCredentialBudget(policy.MaximumInputBytes, lengths...) {
		return credentialLimitError()
	}
	return nil
}

func (policy CredentialPolicy) encoded(lengths ...int) error {
	if !withinCredentialBudget(policy.MaximumEncodedBytes, lengths...) {
		return credentialLimitError()
	}
	return nil
}

// The input is already admitted before this scan. url.QueryEscape would
// allocate; measuring its exact byte expansion lets us reject before encoding.
func credentialQueryBytes(value string) int {
	size := 0
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' ||
			character == '.' || character == '~' || character == ' ' {
			size++
		} else {
			size += 3
		}
	}
	return size
}

func basicCredentialBytes(usernameBytes, passwordBytes int) int {
	return 6 + ((usernameBytes+passwordBytes+1+2)/3)*4
}

func (policy CredentialPolicy) token(token *oauth2.Token) bool {
	if token == nil || policy.input(len(token.AccessToken), len(token.TokenType), len(token.RefreshToken)) != nil {
		return false
	}
	// Type may scan TokenType, so call it only after raw admission.
	return policy.encoded(len(token.Type()), 1, len(token.AccessToken)) == nil
}

// Admit every count and aggregate raw size before scope validation or copying.
func (policy CredentialPolicy) clientCredentials(options ClientCredentialsOptions) error {
	remaining := policy.MaximumInputBytes
	if !withinCredentialBudget(remaining, len(options.TokenURL), len(options.ClientID), len(options.ClientSecret)) {
		return credentialLimitError()
	}
	remaining -= len(options.TokenURL) + len(options.ClientID) + len(options.ClientSecret)
	items := policy.MaximumItems
	if len(options.Scopes) > items || len(options.EndpointParams) > items-len(options.Scopes) {
		return credentialLimitError()
	}
	items -= len(options.Scopes) + len(options.EndpointParams)
	for _, values := range options.EndpointParams {
		if len(values) > items {
			return credentialLimitError()
		}
		items -= len(values)
	}
	for _, scope := range options.Scopes {
		if len(scope) > remaining {
			return credentialLimitError()
		}
		remaining -= len(scope)
	}
	for name, values := range options.EndpointParams {
		if len(name) > remaining {
			return credentialLimitError()
		}
		remaining -= len(name)
		for _, value := range values {
			if len(value) > remaining {
				return credentialLimitError()
			}
			remaining -= len(value)
		}
	}
	// The endpoint request body and client Authorization value each have a
	// separate encoded-output budget. URL length belongs to the raw budget.
	// grant_type=client_credentials is always sent.
	remainingEncoded := policy.MaximumEncodedBytes
	consume := func(lengths ...int) bool {
		if !withinCredentialBudget(remainingEncoded, lengths...) {
			return false
		}
		for _, length := range lengths {
			remainingEncoded -= length
		}
		return true
	}
	if !consume(len("grant_type=client_credentials")) {
		return credentialLimitError()
	}
	if options.AuthStyle == oauth2.AuthStyleInParams {
		if !consume(len("&client_id=&client_secret="), credentialQueryBytes(options.ClientID), credentialQueryBytes(options.ClientSecret)) {
			return credentialLimitError()
		}
	} else if err := policy.encoded(basicCredentialBytes(credentialQueryBytes(options.ClientID), credentialQueryBytes(options.ClientSecret))); err != nil {
		return err
	}
	if len(options.Scopes) > 0 {
		if !consume(len("&scope="), len(options.Scopes)-1) {
			return credentialLimitError()
		}
		for _, scope := range options.Scopes {
			if !consume(credentialQueryBytes(scope)) {
				return credentialLimitError()
			}
		}
	}
	for name, values := range options.EndpointParams {
		for _, value := range values {
			if !consume(2, credentialQueryBytes(name), credentialQueryBytes(value)) {
				return credentialLimitError()
			}
		}
	}
	return nil
}
