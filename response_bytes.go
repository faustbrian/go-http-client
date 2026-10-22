package httpclient

import (
	"errors"
	"io"
	"net/http"
)

// ReadOptions configures bounded raw response consumption, without codec policy.
type ReadOptions struct {
	// MaximumBodyBytes bounds consumed payload bytes; zero selects 8 MiB, at most
	// 1 GiB. Buffer growth can allocate more than the payload limit.
	MaximumBodyBytes int64
}

// ReadResponseBytes consumes and closes a nonnil response body exactly once,
// including after invalid options, read failure or overflow. It returns raw
// bytes without interpreting status, media type, declared length or document
// framing. Empty bodies succeed. Read and limit failures discard partial bytes;
// a close-only failure may accompany a complete body. Error text omits causes.
//
// Reads follow the body's cancellation/deadline behavior; use a response from
// a bounded request. This byte limit cannot interrupt a blocking custom body.
func ReadResponseBytes(response *http.Response, options ReadOptions) (body []byte, resultErr error) {
	if response == nil || nilLike(response.Body) {
		return nil, ErrInvalidResponsePolicy
	}
	source := response.Body
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, &ResponseBodyError{Operation: "close", Cause: closeErr})
		}
	}()
	maximum, err := resolveMaximumBodyBytes(options.MaximumBodyBytes)
	if err != nil {
		return nil, err
	}
	reader := &boundedResponseReader{
		reader: source, remaining: maximum, limit: maximum, expected: -1,
	}
	body, err = io.ReadAll(reader)
	if err != nil {
		return nil, wrapResponseDecodeError(err)
	}
	return body, nil
}
