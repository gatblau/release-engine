package scheduler

import (
	"errors"
	"fmt"
)

var (
	ErrLeaseAcquireConflict = errors.New("lease already held")
	ErrFencedConflict       = errors.New("lease lost or fenced out")
)

type LeaseError struct {
	Err    error
	Code   string
	Detail map[string]string
}

func (e *LeaseError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *LeaseError) Unwrap() error {
	return e.Err
}
