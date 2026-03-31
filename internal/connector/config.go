// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FamilyConfig holds connector family bindings configuration.
type FamilyConfig struct {
	Families map[string]string `yaml:"families"` // family name → connector key
}

// allowedConfigRoot defines the base directory from which config files may be read.
// Override at initialisation time if your deployment uses a different root.
var allowedConfigRoot = func() string {
	root, err := filepath.Abs(".")
	if err != nil {
		return "."
	}
	return root
}()

// safeReadFile resolves `path` to an absolute, cleaned path and verifies it
// remains within `allowedConfigRoot` before reading, preventing path traversal.
func safeReadFile(path string) ([]byte, error) {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Ensure the resolved path is inside the allowed root directory.
	rootWithSep := allowedConfigRoot + string(os.PathSeparator)
	if absPath != allowedConfigRoot && !strings.HasPrefix(absPath, rootWithSep) {
		return nil, fmt.Errorf("config path %q is outside the allowed directory %q", absPath, allowedConfigRoot)
	}

	// #nosec G304 -- path has been validated against allowedConfigRoot above.
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// LoadFamilyConfig loads family bindings from a YAML file.
func LoadFamilyConfig(path string) (*FamilyConfig, error) {
	if path == "" {
		path = "connectors.yaml"
	}

	data, err := safeReadFile(path)
	if err != nil {
		// Return empty config if file doesn't exist.
		if os.IsNotExist(err) {
			return &FamilyConfig{Families: make(map[string]string)}, nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config struct {
		Connectors struct {
			Families map[string]string `yaml:"families"`
		} `yaml:"connectors"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config %s: %w", path, err)
	}

	return &FamilyConfig{Families: config.Connectors.Families}, nil
}

// LoadFamilyConfigFromEnv loads family bindings from environment variables.
// Format: CONNECTOR_FAMILY_<FAMILY_NAME>=<CONNECTOR_KEY>
func LoadFamilyConfigFromEnv() (*FamilyConfig, error) {
	config := &FamilyConfig{Families: make(map[string]string)}

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]
		if strings.HasPrefix(key, "CONNECTOR_FAMILY_") {
			familyName := strings.TrimPrefix(key, "CONNECTOR_FAMILY_")
			familyName = strings.ToLower(familyName)
			config.Families[familyName] = value
		}
	}

	return config, nil
}

// NOTE: ApplyBindings is deprecated. The new design uses per-module resolution
// via FamilyRegistry.Resolve(family, implKey) at module assembly time.
// This function is kept for backward compatibility with any legacy code.
func (c *FamilyConfig) ApplyBindings(registry FamilyRegistry) error {
	// In the new design, bindings are resolved per-module, not globally.
	// This function is a no-op in the new architecture.
	return nil
}

// DefaultFamilies returns the default family contracts with their valid members.
func DefaultFamilies() []ConnectorFamily {
	return []ConnectorFamily{
		{
			Name:    "git",
			Members: []string{"git-github", "git-gitea", "git-file"},
			Operations: map[string]OperationContract{
				"commit_files": {
					RequiredInputFields: []string{
						"repo",
						"branch",
						"path_prefix",
						"files",
						"message",
						"idempotency_key",
					},
					RequiredOutputFields: []string{
						"commit_sha",
						"changed",
					},
				},
				"read_file": {
					RequiredInputFields: []string{
						"repo",
						"branch",
						"path",
					},
					RequiredOutputFields: []string{
						"content",
						"sha",
						"status",
					},
				},
			},
		},
		{
			Name:    "policy",
			Members: []string{"policy-embedded", "policy-pmock", "policy-opa"},
			Operations: map[string]OperationContract{
				"evaluate": {
					RequiredInputFields: []string{
						"policy_bundle",
						"resource",
					},
					RequiredOutputFields: []string{
						"allowed",
						"violations",
					},
				},
			},
		},
		{
			Name:    "webhook",
			Members: []string{"webhook-webhook", "webhook-wmock", "webhook-http"},
			Operations: map[string]OperationContract{
				"post_callback": {
					RequiredInputFields: []string{
						"url",
						"headers",
						"body",
						"idempotency_key",
					},
					RequiredOutputFields: []string{
						"status_code",
						"response_body",
					},
				},
			},
		},
		{
			Name:    "infra",
			Members: []string{"infra-crossplane", "infra-crossplaneMock"},
			Operations: map[string]OperationContract{
				"apply": {
					RequiredInputFields: []string{
						"manifest",
					},
					RequiredOutputFields: []string{
						"applied",
						"status",
					},
				},
			},
		},
	}
}
