package scheduler

import (
	"errors"
	"fmt"
)

var (
	ErrClaimQueryFailed  = errors.New("claim query failed")
	ErrDispatchSaturated = errors.New("dispatch queue saturated")
)

type SchedulerError struct {
	Err    error
	Code   string
	Detail map[string]string
}

func (e *SchedulerError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *SchedulerError) Unwrap() error {
	return e.Err
}
