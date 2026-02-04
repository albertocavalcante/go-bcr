package bcr

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a module or version does not exist.
// Use [errors.Is] to check for this error, or [errors.As] with
// [*NotFoundError] to get detailed information.
var ErrNotFound = errors.New("bcr: not found")

// NotFoundError provides details about what was not found.
type NotFoundError struct {
	// Module is the module name that was queried.
	Module string

	// Version is the version that was queried, or empty if the
	// module itself was not found.
	Version string

	// StatusCode is the HTTP status code, if available.
	StatusCode int
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	if e.Version != "" {
		return fmt.Sprintf("bcr: module %q version %q not found", e.Module, e.Version)
	}
	return fmt.Sprintf("bcr: module %q not found", e.Module)
}

// Is reports whether this error matches the target.
// Returns true for [ErrNotFound].
func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// Unwrap returns nil (NotFoundError is a leaf error).
func (e *NotFoundError) Unwrap() error {
	return nil
}

// RequestError indicates an error making an HTTP request.
type RequestError struct {
	// URL is the URL that was requested.
	URL string

	// StatusCode is the HTTP status code, or 0 if the request failed
	// before receiving a response.
	StatusCode int

	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *RequestError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("bcr: request to %s failed with status %d", e.URL, e.StatusCode)
	}
	return fmt.Sprintf("bcr: request to %s failed: %v", e.URL, e.Err)
}

// Unwrap returns the underlying error.
func (e *RequestError) Unwrap() error {
	return e.Err
}
