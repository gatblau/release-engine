package http

import (
	"errors"
	"fmt"
)

var (
	ErrHTTPBindFailed      = errors.New("http bind failed")
	ErrHTTPShutdownTimeout = errors.New("shutdown timed out")
)

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
