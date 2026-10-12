package httpclient

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	// ErrInvalidStatusPolicy indicates invalid response classification policy.
	ErrInvalidStatusPolicy = errors.New("invalid HTTP status policy")
	// ErrHTTPStatus indicates a response status rejected by caller policy.
	ErrHTTPStatus = errors.New("HTTP response status rejected")
)

const (
	defaultMaximumStatusDrainBytes = 64 << 10
	maximumStatusExcerptBytes      = 1 << 20
	maximumStatusDrainBytes        = 16 << 20
)

// StatusSnapshot supplies raw cloned headers and request ID, plus an optional
// bounded redacted body excerpt. It is not a fully sanitized response snapshot.
type StatusSnapshot struct {
	// StatusCode is the rejected response status.
	StatusCode int
	// Header is a clone of raw response headers, not a redacted or independently bounded collection.
	Header http.Header
	// Excerpt is the optional bounded redacted body prefix.
	Excerpt []byte
	// RequestID is the first selected raw field value, not sanitized by the package.
	RequestID string
}

// ExcerptRedactor returns an independent safe excerpt. It must not retain its
// input, which can contain sensitive response data.
type ExcerptRedactor func([]byte) ([]byte, error)

// VendorErrorMapper maps response state to a stable vendor code. Only Excerpt
// has passed the redactor; raw headers and request ID require caller policy. The
// returned cause remains available without being rendered.
type VendorErrorMapper func(StatusSnapshot) (string, error)

// StatusOptions configures independent status classification.
type StatusOptions struct {
	// Accept is borrowed; nil accepts 2xx without consuming the body.
	Accept func(int) bool
	// MaximumExcerptBytes opts into capture only with a redactor; at most 1 MiB.
	MaximumExcerptBytes int64
	// MaximumDrainBytes bounds additional discarded bytes; zero selects 64 KiB, at most 16 MiB.
	MaximumDrainBytes int64
	// RedactExcerpt is borrowed and must return sanitized bytes within the excerpt bound.
	RedactExcerpt ExcerptRedactor
	// Retryable is borrowed and classifies status with cloned raw headers.
	Retryable func(int, http.Header) bool
	// MapVendorError is borrowed; it receives the excerpt plus raw headers and request ID.
	MapVendorError VendorErrorMapper
	// RequestIDHeaders is copied in priority order; empty selects X-Request-ID and X-Correlation-ID.
	RequestIDHeaders []string
}

// HTTPStatusError preserves structured response state, including raw headers
// and request ID. Callers must classify exported values before use. Error text never
// renders headers, excerpts, vendor messages, request IDs, or causes.
type HTTPStatusError struct {
	// StatusCode is the rejected status.
	StatusCode int
	// Header contains cloned raw headers; callers must redact before logging or exporting.
	Header http.Header
	// VendorCode is mapper-defined metadata, not independently sanitized.
	VendorCode string
	// Excerpt contains only the opted-in bounded redactor output.
	Excerpt []byte
	// RequestID is the selected raw identifier, not independently sanitized.
	RequestID string
	// Retryable is the configured classifier result.
	Retryable bool
	// Cause retains the underlying error for unwrapping; it may contain sensitive provider data.
	Cause error
}

// Error implements error.
func (*HTTPStatusError) Error() string { return "HTTP response status rejected" }

// Unwrap preserves the stable category and optional vendor cause.
func (err *HTTPStatusError) Unwrap() []error {
	causes := []error{ErrHTTPStatus}
	if err.Cause != nil {
		causes = append(causes, err.Cause)
	}
	return causes
}

// ClassifyResponse returns nil for an accepted response without touching its
// body. A rejected response is boundedly consumed and always closed.
func ClassifyResponse(response *http.Response, options StatusOptions) (resultErr error) {
	if response == nil || response.Body == nil || response.Header == nil {
		return fmt.Errorf("%w: response state is invalid", ErrInvalidStatusPolicy)
	}
	accept := options.Accept
	if accept == nil {
		accept = func(status int) bool { return status >= 200 && status < 300 }
	}
	if accept(response.StatusCode) {
		return nil
	}
	body := response.Body
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, &ResponseBodyError{Operation: "close", Cause: closeErr})
		}
	}()
	if options.MaximumExcerptBytes < 0 || options.MaximumExcerptBytes > maximumStatusExcerptBytes ||
		options.MaximumDrainBytes < 0 || options.MaximumDrainBytes > maximumStatusDrainBytes ||
		options.MaximumExcerptBytes > 0 && options.RedactExcerpt == nil {
		return fmt.Errorf("%w: body bounds or redactor are invalid", ErrInvalidStatusPolicy)
	}
	drain := options.MaximumDrainBytes
	if drain == 0 {
		drain = defaultMaximumStatusDrainBytes
	}
	var excerpt []byte
	if options.MaximumExcerptBytes > 0 {
		content, readErr := io.ReadAll(io.LimitReader(body, options.MaximumExcerptBytes))
		excerpt = append([]byte(nil), content...)
		if readErr != nil {
			resultErr = errors.Join(resultErr, &ResponseBodyError{Operation: "excerpt read", Cause: readErr})
		}
		redacted, redactErr := options.RedactExcerpt(append([]byte(nil), excerpt...))
		if redactErr != nil {
			resultErr = errors.Join(resultErr, &ResponseBodyError{Operation: "excerpt redaction", Cause: redactErr})
			excerpt = nil
		} else if int64(len(redacted)) > options.MaximumExcerptBytes {
			resultErr = errors.Join(resultErr, &ResponseBodyError{Operation: "excerpt redaction", Cause: ErrInvalidStatusPolicy})
			excerpt = nil
		} else {
			excerpt = append([]byte(nil), redacted...)
		}
	}
	if _, drainErr := io.Copy(io.Discard, io.LimitReader(body, drain+1)); drainErr != nil {
		resultErr = errors.Join(resultErr, &ResponseBodyError{Operation: "drain", Cause: drainErr})
	}
	requestIDHeaders := options.RequestIDHeaders
	if len(requestIDHeaders) == 0 {
		requestIDHeaders = []string{"X-Request-ID", "X-Correlation-ID"}
	}
	requestID := ""
	for _, name := range requestIDHeaders {
		if value := response.Header.Get(name); value != "" {
			requestID = value
			break
		}
	}
	snapshot := StatusSnapshot{
		StatusCode: response.StatusCode, Header: response.Header.Clone(),
		Excerpt: append([]byte(nil), excerpt...), RequestID: requestID,
	}
	statusError := &HTTPStatusError{
		StatusCode: response.StatusCode, Header: response.Header.Clone(),
		Excerpt: append([]byte(nil), excerpt...), RequestID: requestID,
	}
	if options.Retryable != nil {
		statusError.Retryable = options.Retryable(response.StatusCode, response.Header.Clone())
	}
	if options.MapVendorError != nil {
		statusError.VendorCode, statusError.Cause = options.MapVendorError(snapshot)
	}

	return errors.Join(statusError, resultErr)
}
