// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// isValidModuleName ensures the module name contains only safe characters,
// preventing path traversal attacks when used in file path construction.
func isValidModuleName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// isPathSafe checks if a path is safe by ensuring it doesn't contain path traversal
// sequences. For environment variables (user-controlled input), we should validate.
// For defaultBasePath (application-controlled), we can be more permissive.
func isPathSafe(path string) bool {
	if path == "" {
		return false
	}

	// Clean the path to resolve any ".." or "." components
	cleanPath := filepath.Clean(path)

	// Check for any ".." component in the path
	// We split by path separator and check each component
	components := strings.Split(filepath.ToSlash(cleanPath), "/")
	for _, comp := range components {
		if comp == ".." {
			return false
		}
	}

	return true
}

// Loader defines the interface for loading module configuration files.
type Loader interface {
	// Load loads the configuration for the specified module.
	// It follows the path resolution order:
	// 1. CFG_PATH_<MODULE_UPPER>
	// 2. ${CFG_ROOT}/cfg_<module>.yaml if CFG_ROOT is set
	// 3. <defaultBasePath>/cfg_<module>.yaml
	Load(ctx context.Context, moduleName string) (*ModuleConfigFile, error)
}

type loader struct {
	defaultBasePath string
}

// NewLoader creates a new Loader with the given default base path.
func NewLoader(defaultBasePath string) Loader {
	return &loader{defaultBasePath: defaultBasePath}
}

// Load implements the Loader interface.
func (l *loader) Load(ctx context.Context, moduleName string) (*ModuleConfigFile, error) {
	// Find the config file path
	configPath, fromEnvVar, err := l.resolveConfigPath(moduleName)
	if err != nil {
		return nil, err
	}

	// Sanitize and confine the resolved path before any file operation.
	// This breaks the taint chain from user-controlled input (os.Getenv)
	// and satisfies G304 (CWE-22).
	configPath, err = l.sanitizeAndConfine(configPath, fromEnvVar)
	if err != nil {
		return nil, err
	}

	// Read the file
	data, err := os.ReadFile(configPath) // #nosec G304 -- path is sanitized and confined above
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewConfigError(ErrConfigFileNotFound, "CONFIG_FILE_NOT_FOUND", map[string]string{
				"path": configPath,
			})
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse YAML
	var rawConfig ModuleConfigFile
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return nil, NewConfigError(ErrConfigInvalidYAML, "CONFIG_INVALID_YAML", map[string]string{
			"path":  configPath,
			"error": err.Error(),
		})
	}

	// Validate framework-level concerns
	if err := l.validateFrameworkConcerns(&rawConfig, moduleName); err != nil {
		return nil, err
	}

	return &rawConfig, nil
}

// sanitizeAndConfine cleans the path and ensures it is confined to one of the
// permitted root directories, breaking the taint chain from user-controlled input.
// The fromEnvVar parameter indicates whether this path came from CFG_PATH_<MODULE>
// environment variable. If true, the path is allowed even if not within permitted
// roots, as it represents an explicit operator configuration.
func (l *loader) sanitizeAndConfine(path string, fromEnvVar bool) (string, error) {
	cleanPath := filepath.Clean(path)

	// If the path came from CFG_PATH_<MODULE> environment variable, we allow it
	// (after basic safety checks) because the operator explicitly configured it.
	if fromEnvVar {
		if !isPathSafe(cleanPath) {
			return "", NewConfigError(ErrConfigInvalidModuleName, "CONFIG_INVALID_PATH", map[string]string{
				"path":   path,
				"reason": "path contains traversal sequences",
			})
		}
		return cleanPath, nil
	}

	// Build the list of permitted root directories.
	// Only application-controlled or already-validated roots are included.
	permittedRoots := []string{}

	if cfgRoot := os.Getenv("CFG_ROOT"); cfgRoot != "" {
		permittedRoots = append(permittedRoots, filepath.Clean(cfgRoot))
	}

	if l.defaultBasePath != "" {
		permittedRoots = append(permittedRoots, filepath.Clean(l.defaultBasePath))
	}

	// If no permitted roots are defined, fall back to basic safety check
	if len(permittedRoots) == 0 {
		if !isPathSafe(cleanPath) {
			return "", NewConfigError(ErrConfigInvalidModuleName, "CONFIG_INVALID_PATH", map[string]string{
				"path":   path,
				"reason": "path contains traversal sequences",
			})
		}
		return cleanPath, nil
	}

	// Verify the clean path is under at least one permitted root.
	for _, root := range permittedRoots {
		prefix := root + string(filepath.Separator)
		if strings.HasPrefix(cleanPath, prefix) || cleanPath == root {
			return cleanPath, nil
		}
	}

	return "", NewConfigError(ErrConfigInvalidModuleName, "CONFIG_INVALID_PATH", map[string]string{
		"path":   path,
		"reason": "path is not within any permitted config root directory",
	})
}

// resolveConfigPath resolves the configuration file path using the precedence rules.
// Returns the resolved path and a boolean indicating if the path came from
// CFG_PATH_<MODULE> environment variable (true) or from CFG_ROOT/default (false).
func (l *loader) resolveConfigPath(moduleName string) (string, bool, error) {
	// Validate module name to prevent path traversal
	if !isValidModuleName(moduleName) {
		return "", false, NewConfigError(ErrConfigInvalidModuleName, "CONFIG_INVALID_MODULE_NAME", map[string]string{
			"module": moduleName,
			"reason": "module name contains invalid characters",
		})
	}

	// 1. Check CFG_PATH_<MODULE_UPPER>
	envVarName := fmt.Sprintf("CFG_PATH_%s", strings.ToUpper(moduleName))
	if path := os.Getenv(envVarName); path != "" {
		// Validate that the path from environment variable is safe
		if !isPathSafe(path) {
			return "", false, NewConfigError(ErrConfigInvalidModuleName, "CONFIG_INVALID_MODULE_NAME", map[string]string{
				"module": moduleName,
				"reason": "CFG_PATH contains path traversal sequences",
			})
		}
		// Use the cleaned path to avoid taint propagation
		return filepath.Clean(path), true, nil
	}

	// 2. Check CFG_ROOT/cfg_<module>.yaml
	if cfgRoot := os.Getenv("CFG_ROOT"); cfgRoot != "" {
		// Validate CFG_ROOT path is safe
		if !isPathSafe(cfgRoot) {
			return "", false, NewConfigError(ErrConfigInvalidModuleName, "CONFIG_INVALID_MODULE_NAME", map[string]string{
				"module": moduleName,
				"reason": "CFG_ROOT contains path traversal sequences",
			})
		}

		cleanRoot := filepath.Clean(cfgRoot)
		cleanPath := filepath.Clean(filepath.Join(cleanRoot, fmt.Sprintf("cfg_%s.yaml", moduleName)))

		// Ensure the resolved path is within the expected directory
		if !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) && cleanPath != cleanRoot {
			return "", false, NewConfigError(ErrConfigInvalidModuleName, "CONFIG_INVALID_MODULE_NAME", map[string]string{
				"module": moduleName,
				"reason": "resolved path escapes config root directory",
			})
		}

		// Use cleanPath (sanitized) for os.Stat to avoid taint analysis warning
		if _, err := os.Stat(cleanPath); err == nil {
			return cleanPath, false, nil
		}
		// Continue to default path if file doesn't exist in CFG_ROOT
	}

	// 3. Use defaultBasePath/cfg_<module>.yaml
	if l.defaultBasePath == "" {
		return "", false, NewConfigError(ErrConfigFileNotFound, "CONFIG_PATH_RESOLUTION_FAILED", map[string]string{
			"module": moduleName,
			"reason": "no config path found and defaultBasePath is empty",
		})
	}

	cleanBasePath := filepath.Clean(l.defaultBasePath)
	cleanPath := filepath.Clean(filepath.Join(cleanBasePath, fmt.Sprintf("cfg_%s.yaml", moduleName)))

	// Ensure the resolved path is within the expected directory
	if !strings.HasPrefix(cleanPath, cleanBasePath+string(filepath.Separator)) && cleanPath != cleanBasePath {
		return "", false, NewConfigError(ErrConfigInvalidModuleName, "CONFIG_INVALID_MODULE_NAME", map[string]string{
			"module": moduleName,
			"reason": "resolved path escapes base directory",
		})
	}

	return cleanPath, false, nil
}

// validateFrameworkConcerns validates framework-level requirements.
func (l *loader) validateFrameworkConcerns(config *ModuleConfigFile, requestedModule string) error {
	// Validate API version
	if config.APIVersion == "" {
		return NewConfigError(ErrConfigMissingField, "CONFIG_MISSING_FIELD", map[string]string{
			"field": "apiVersion",
		})
	}
	if config.APIVersion != APIVersionV1 {
		return NewConfigError(ErrConfigUnsupportedAPIVersion, "CONFIG_UNSUPPORTED_API_VERSION", map[string]string{
			"expected": APIVersionV1,
			"actual":   config.APIVersion,
		})
	}

	// Validate module name
	if config.Module == "" {
		return NewConfigError(ErrConfigMissingField, "CONFIG_MISSING_FIELD", map[string]string{
			"field": "module",
		})
	}
	if config.Module != requestedModule {
		return NewConfigError(ErrConfigModuleMismatch, "CONFIG_MODULE_MISMATCH", map[string]string{
			"expected": requestedModule,
			"actual":   config.Module,
		})
	}

	// Validate connectors.families exists
	if config.Connectors.Families == nil {
		return NewConfigError(ErrConfigMissingConnectorFamilies, "CONFIG_MISSING_CONNECTOR_FAMILIES", nil)
	}

	// Note: We don't validate unknown top-level fields here because yaml.v3's
	// strict decoder would be needed for that. We'll leave that for a future enhancement
	// when we implement strict YAML decoding.

	return nil
}
