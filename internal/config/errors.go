package config

import (
	"errors"
	"fmt"
)

var (
	ErrConfigMissing = errors.New("missing required configuration")
	ErrConfigInvalid = errors.New("invalid configuration value")
)

type ConfigError struct {
	Err    error
	Code   string
	Detail map[string]string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}
