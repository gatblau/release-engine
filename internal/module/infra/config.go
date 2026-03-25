// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package infra

import (
	"fmt"
	"time"

	"github.com/gatblau/release-engine/internal/module/config"
)

// Config represents the typed configuration for the infra module.
type Config struct {
	Vars       Vars
	Connectors ConnectorSelection
}

// Vars contains module-specific configuration variables.
type Vars struct {
	HealthTimeout time.Duration
	PollInterval  time.Duration
	LogLevel      string
}

// ConnectorSelection specifies which connector implementations to use.
type ConnectorSelection struct {
	Git        string
	Crossplane string
	Policy     string
	Webhook    string
}

// ParseConfig converts a raw ModuleConfigFile into typed infra Config.
// It applies defaults, validates var types and ranges, and ensures
// required connector families are present.
func ParseConfig(raw *config.ModuleConfigFile) (*Config, error) {
	if raw == nil {
		return nil, fmt.Errorf("raw config is nil")
	}

	// Parse vars
	vars, err := parseVars(raw.Vars)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vars: %w", err)
	}

	// Parse connector selections
	connectors, err := parseConnectors(raw.Connectors)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connectors: %w", err)
	}

	return &Config{
		Vars:       *vars,
		Connectors: *connectors,
	}, nil
}

// RequiredConnectorFamilies returns the list of connector families
// that the infra module requires to be configured.
func RequiredConnectorFamilies() []string {
	return []string{"git", "crossplane", "policy", "webhook"}
}

// parseVars extracts and validates module-specific variables.
func parseVars(vars map[string]any) (*Vars, error) {
	result := &Vars{
		// Default values
		HealthTimeout: 30 * time.Second,
		PollInterval:  500 * time.Millisecond,
		LogLevel:      "info",
	}

	// Parse health_timeout if provided
	if val, ok := vars["health_timeout"]; ok {
		if str, ok := val.(string); ok {
			duration, err := time.ParseDuration(str)
			if err != nil {
				return nil, fmt.Errorf("invalid health_timeout duration %q: %w", str, err)
			}
			if duration <= 0 {
				return nil, fmt.Errorf("health_timeout must be positive, got %v", duration)
			}
			result.HealthTimeout = duration
		} else {
			return nil, fmt.Errorf("health_timeout must be a string duration, got %T", val)
		}
	}

	// Parse poll_interval if provided
	if val, ok := vars["poll_interval"]; ok {
		if str, ok := val.(string); ok {
			duration, err := time.ParseDuration(str)
			if err != nil {
				return nil, fmt.Errorf("invalid poll_interval duration %q: %w", str, err)
			}
			if duration <= 0 {
				return nil, fmt.Errorf("poll_interval must be positive, got %v", duration)
			}
			result.PollInterval = duration
		} else {
			return nil, fmt.Errorf("poll_interval must be a string duration, got %T", val)
		}
	}

	// Parse log_level if provided
	if val, ok := vars["log_level"]; ok {
		if str, ok := val.(string); ok {
			// Validate log level (will be validated by logger factory later)
			validLevels := map[string]bool{
				"debug": true, "info": true, "warn": true, "warning": true,
				"error": true, "dpanic": true, "panic": true, "fatal": true,
			}
			if !validLevels[str] {
				return nil, fmt.Errorf("invalid log_level %q, must be one of: debug, info, warn, error, dpanic, panic, fatal", str)
			}
			result.LogLevel = str
		} else {
			return nil, fmt.Errorf("log_level must be a string, got %T", val)
		}
	}

	// Validate that poll_interval is less than health_timeout
	if result.PollInterval >= result.HealthTimeout {
		return nil, fmt.Errorf("poll_interval (%v) must be less than health_timeout (%v)",
			result.PollInterval, result.HealthTimeout)
	}

	return result, nil
}

// parseConnectors extracts connector family to implementation mappings.
func parseConnectors(connectors config.ConnectorsConfig) (*ConnectorSelection, error) {
	result := &ConnectorSelection{}

	// Check required families
	required := RequiredConnectorFamilies()
	for _, family := range required {
		if impl, ok := connectors.Families[family]; !ok || impl == "" {
			return nil, fmt.Errorf("missing required connector family %q in connectors.families", family)
		}
	}

	// Extract implementations
	result.Git = connectors.Families["git"]
	result.Crossplane = connectors.Families["crossplane"]
	result.Policy = connectors.Families["policy"]
	result.Webhook = connectors.Families["webhook"]

	// Validate that implementations are non-empty strings
	if result.Git == "" {
		return nil, fmt.Errorf("git connector implementation cannot be empty")
	}
	if result.Crossplane == "" {
		return nil, fmt.Errorf("crossplane connector implementation cannot be empty")
	}
	if result.Policy == "" {
		return nil, fmt.Errorf("policy connector implementation cannot be empty")
	}
	if result.Webhook == "" {
		return nil, fmt.Errorf("webhook connector implementation cannot be empty")
	}

	return result, nil
}
