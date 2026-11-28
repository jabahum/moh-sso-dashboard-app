package client

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors. Every error returned by a Client call matches one of them
// with errors.Is (or ErrToken when no token could be obtained).
var (
	// ErrInvalid: the request was rejected as malformed (400), or failed the
	// client's own checks before it was sent. Don't retry unchanged.
	ErrInvalid = errors.New("platform client: invalid request")
	// ErrUnauthorized: the token is missing, expired, inactive, or has the
	// wrong issuer or audience (401).
	ErrUnauthorized = errors.New("platform client: unauthorized")
	// ErrForbidden: not bound, binding disabled, scope not effective,
	// application not active and enabled, or the user has no access (403).
	ErrForbidden = errors.New("platform client: forbidden")
	// ErrNotFound: unknown resource, or one owned by another application (404).
	ErrNotFound = errors.New("platform client: not found")
	// ErrConflict: an idempotency key was reused with a different body (409).
	ErrConflict = errors.New("platform client: conflict")
	// ErrRateLimited: a quota was exceeded (429); see Error.RetryAfter.
	ErrRateLimited = errors.New("platform client: rate limited")
	// ErrUnavailable: the portal or a dependency is unavailable (5xx), it
	// could not be reached, did not answer in time, or answered with a body
	// the client could not read. Fail closed for authorization decisions.
	ErrUnavailable = errors.New("platform client: portal unavailable")
	// ErrUnexpected: any other answer, including redirects.
	ErrUnexpected = errors.New("platform client: unexpected response")
)

// Error describes a failed platform API call. It never contains tokens or
// secrets.
type Error struct {
	// Status is the HTTP status, or 0 when no response was received.
	Status int
	// Code is the portal's error code (for example APPLICATION_ACCESS_DENIED),
	// or NETWORK_ERROR, INVALID_REQUEST, INVALID_RESPONSE or HTTP_<status>.
	Code    string
	Message string
	// RequestID is the portal's request ID; quote it when reporting problems.
	RequestID string
	// RetryAfter is the portal's Retry-After hint, if any.
	RetryAfter time.Duration

	sentinel error
	cause    error
}

func (e *Error) Error() string {
	text := "platform API"
	if e.Status != 0 {
		text += fmt.Sprintf(" HTTP %d", e.Status)
	}
	text += ": " + e.Code
	if e.Message != "" {
		text += ": " + e.Message
	}
	if e.RequestID != "" {
		text += " (request " + e.RequestID + ")"
	}
	return text
}

// Is matches the sentinel for the error's status.
func (e *Error) Is(target error) bool { return target == e.sentinel }

// Unwrap returns the underlying network error, if any.
func (e *Error) Unwrap() error { return e.cause }

func sentinelFor(status int) error {
	switch {
	case status == 400:
		return ErrInvalid
	case status == 401:
		return ErrUnauthorized
	case status == 403:
		return ErrForbidden
	case status == 404:
		return ErrNotFound
	case status == 409:
		return ErrConflict
	case status == 429:
		return ErrRateLimited
	case status >= 500:
		return ErrUnavailable
	}
	return ErrUnexpected
}

func invalid(message string) *Error {
	return &Error{Code: "INVALID_REQUEST", Message: message, sentinel: ErrInvalid}
}
