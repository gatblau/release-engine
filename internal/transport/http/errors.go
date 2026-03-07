package http

import (
	"errors"
	"fmt"
)

var (
	ErrHTTPBindFailed      = errors.New("http bind failed")
	ErrHTTPShutdownTimeout = errors.New("shutdown timed out")
)

// HTTPError represents an HTTP-specific error.
type HTTPError struct {
	Err    error
	Code   string
	Detail map[string]string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}
