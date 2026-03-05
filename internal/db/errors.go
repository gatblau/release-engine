package db

import (
	"errors"
	"fmt"
)

var (
	ErrDBUnavailable    = errors.New("database unavailable")
	ErrInvalidIsolation = errors.New("invalid isolation level")
)

type DBError struct {
	Err    error
	Code   string
	Detail map[string]string
}

func (e *DBError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *DBError) Unwrap() error {
	return e.Err
}
