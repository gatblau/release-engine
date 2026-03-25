// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package infra

import (
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/module/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequiredConnectorFamilies(t *testing.T) {
	families := RequiredConnectorFamilies()
	expected := []string{"git", "crossplane", "policy", "webhook"}
	assert.Equal(t, expected, families)
}

func TestParseConfig_ValidConfig(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "10s",
			"poll_interval":  "200ms",
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check vars
	assert.Equal(t, 10*time.Second, cfg.Vars.HealthTimeout)
	assert.Equal(t, 200*time.Millisecond, cfg.Vars.PollInterval)

	// Check connectors
	assert.Equal(t, "git-file", cfg.Connectors.Git)
	assert.Equal(t, "crossplane-mock", cfg.Connectors.Crossplane)
	assert.Equal(t, "policy-mock", cfg.Connectors.Policy)
	assert.Equal(t, "webhook-mock", cfg.Connectors.Webhook)
}

func TestParseConfig_Defaults(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars:       map[string]any{}, // Empty vars
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Check default values
	assert.Equal(t, 30*time.Second, cfg.Vars.HealthTimeout)
	assert.Equal(t, 500*time.Millisecond, cfg.Vars.PollInterval)
}

func TestParseConfig_MissingRequiredConnectorFamily(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "10s",
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				// Missing webhook
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "missing required connector family")
	assert.Contains(t, err.Error(), "webhook")
}

func TestParseConfig_EmptyConnectorImplementation(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "10s",
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "", // Empty
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "missing required connector family")
	assert.Contains(t, err.Error(), "git")
}

func TestParseConfig_InvalidHealthTimeout(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "invalid-duration",
			"poll_interval":  "100ms",
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid health_timeout duration")
}

func TestParseConfig_InvalidPollInterval(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "10s",
			"poll_interval":  "not-a-duration",
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid poll_interval duration")
}

func TestParseConfig_NilRawConfig(t *testing.T) {
	cfg, err := ParseConfig(nil)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "raw config is nil")
}

func TestParseConfig_PollIntervalGreaterThanHealthTimeout(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "1s",
			"poll_interval":  "2s", // Greater than health_timeout
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "poll_interval (2s) must be less than health_timeout (1s)")
}

func TestParseConfig_ZeroHealthTimeout(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "0s",
			"poll_interval":  "100ms",
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "health_timeout must be positive")
}

func TestParseConfig_NegativePollInterval(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "10s",
			"poll_interval":  "-100ms",
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "poll_interval must be positive")
}

func TestParseConfig_NonStringDuration(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": 123, // Not a string
			"poll_interval":  "100ms",
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "health_timeout must be a string duration")
}

func TestParseConfig_PollIntervalEqualToHealthTimeout(t *testing.T) {
	raw := &config.ModuleConfigFile{
		APIVersion: "module.config/v1",
		Module:     "infra",
		Vars: map[string]any{
			"health_timeout": "1s",
			"poll_interval":  "1s", // Equal to health_timeout
		},
		Connectors: config.ConnectorsConfig{
			Families: map[string]string{
				"git":        "git-file",
				"crossplane": "crossplane-mock",
				"policy":     "policy-mock",
				"webhook":    "webhook-mock",
			},
		},
	}

	cfg, err := ParseConfig(raw)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "poll_interval (1s) must be less than health_timeout (1s)")
}
