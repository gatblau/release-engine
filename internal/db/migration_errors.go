package db

import (
	"errors"
	"fmt"
)

var (
	ErrMigrationOutdated = errors.New("database schema out of date")
	ErrMigrationMetadata = errors.New("migration metadata missing")
)

type MigrationError struct {
	Err    error
	Code   string
	Detail map[string]string
}

func (e *MigrationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *MigrationError) Unwrap() error {
	return e.Err
}
