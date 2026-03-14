// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package logger

import (
	"errors"
	"fmt"
)

var (
	ErrLogLevelInvalid  = errors.New("unsupported log level")
	ErrLoggerInitFailed = errors.New("logger initialisation failed")
)

type LoggerError struct {
	Err    error
	Code   string
	Detail map[string]string
}

func (e *LoggerError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *LoggerError) Unwrap() error {
	return e.Err
}
