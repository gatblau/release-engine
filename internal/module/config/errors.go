// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package config

import (
	"errors"
	"fmt"
)

var (
	// ErrConfigFileNotFound indicates the configuration file does not exist
	ErrConfigFileNotFound = errors.New("configuration file not found")
	// ErrConfigInvalidYAML indicates the YAML content is malformed
	ErrConfigInvalidYAML = errors.New("invalid YAML content")
	// ErrConfigUnsupportedAPIVersion indicates an unsupported API version
	ErrConfigUnsupportedAPIVersion = errors.New("unsupported API version")
	// ErrConfigModuleMismatch indicates the module name in the file doesn't match requested module
	ErrConfigModuleMismatch = errors.New("module name mismatch")
	// ErrConfigMissingField indicates a required field is missing
	ErrConfigMissingField = errors.New("missing required field")
	// ErrConfigMissingConnectorFamilies indicates the connectors.families block is missing
	ErrConfigMissingConnectorFamilies = errors.New("missing connector families mapping")
	// ErrConfigInvalidModuleName indicates the module name contains invalid characters or path traversal
	ErrConfigInvalidModuleName = errors.New("invalid module name")
	// ErrConfigUnknownTopLevelField indicates an unknown top-level field in the YAML
	ErrConfigUnknownTopLevelField = errors.New("unknown top-level field")
)

// ConfigError wraps configuration errors with additional context
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

// NewConfigError creates a new ConfigError with the given details
func NewConfigError(err error, code string, detail map[string]string) *ConfigError {
	return &ConfigError{
		Err:    err,
		Code:   code,
		Detail: detail,
	}
}
