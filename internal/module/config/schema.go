// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package config

// ModuleConfigFile represents the raw framework-level parse representation
// of a module configuration file, not the final runtime config injected into a module.
type ModuleConfigFile struct {
	APIVersion string           `yaml:"apiVersion"`
	Module     string           `yaml:"module"`
	Vars       map[string]any   `yaml:"vars,omitempty"`
	Connectors ConnectorsConfig `yaml:"connectors"`
}

// ConnectorsConfig defines the connector family to implementation mappings.
type ConnectorsConfig struct {
	Families map[string]string `yaml:"families"`
}

// APIVersion constants
const (
	APIVersionV1 = "module.config/v1"
)
