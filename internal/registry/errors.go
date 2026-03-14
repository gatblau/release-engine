// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package registry

import (
	"errors"
	"fmt"
)

var (
	ErrModuleDuplicate    = errors.New("duplicate module registration")
	ErrModuleNotFound     = errors.New("module not found")
	ErrConnectorDuplicate = errors.New("duplicate connector registration")
	ErrConnectorNotFound  = errors.New("connector not found")
)

type RegistryError struct {
	Err    error
	Code   string
	Detail map[string]string
}

func (e *RegistryError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *RegistryError) Unwrap() error {
	return e.Err
}
