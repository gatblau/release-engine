package catalog

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed definitions/*.yaml
var catalogFS embed.FS

// TemplateCatalog defines a provisioning template with constraints.
type TemplateCatalog struct {
	Name                  string             `yaml:"name"`
	Description           string             `yaml:"description"`
	Version               string             `yaml:"version"`
	RequiredCapabilities  []string           `yaml:"required_capabilities"`
	OptionalCapabilities  []string           `yaml:"optional_capabilities"`
	ForbiddenCapabilities []string           `yaml:"forbidden_capabilities"`
	Constraints           CatalogConstraints `yaml:"constraints"`
	Defaults              map[string]any     `yaml:"defaults"`
}

// CatalogConstraints define guardrails for a template.
type CatalogConstraints struct {
	AllowedEnvironments     []string `yaml:"allowed_environments"`
	AllowedWorkloadProfiles []string `yaml:"allowed_workload_profiles"`
	AllowedAvailabilities   []string `yaml:"allowed_availabilities"`
	AllowedResidencies      []string `yaml:"allowed_residencies"`
	AllowedProviders        []string `yaml:"allowed_providers"`
	RequiresApproval        bool     `yaml:"requires_approval"`
	MaxCostMonthly          float64  `yaml:"max_cost_monthly"`
}

// LoadAll loads all catalogue definitions from embedded YAML files.
func LoadAll() (map[string]*TemplateCatalog, error) {
	entries, err := catalogFS.ReadDir("definitions")
	if err != nil {
		return nil, fmt.Errorf("read catalog dir: %w", err)
	}

	catalogs := make(map[string]*TemplateCatalog, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := catalogFS.ReadFile("definitions/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		var cat TemplateCatalog
		if err := yaml.Unmarshal(data, &cat); err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		if cat.Name == "" {
			return nil, fmt.Errorf("parse %s: missing name", entry.Name())
		}
		catalogs[cat.Name] = &cat
	}

	return catalogs, nil
}

// ValidateCatalogs validates all catalogues against a supported providers map.
func ValidateCatalogs(catalogs map[string]*TemplateCatalog, supportedProviders map[string][]string) error {
	for name, cat := range catalogs {
		// Every required/optional capability must exist in SupportedProviders
		allCaps := append(cat.RequiredCapabilities, cat.OptionalCapabilities...)
		for _, cap := range allCaps {
			if _, ok := supportedProviders[cap]; !ok {
				return fmt.Errorf(
					"catalogue %q references unknown capability %q",
					name, cap,
				)
			}
		}

		// Every allowed provider must be supported by at least one required capability
		for _, provider := range cat.Constraints.AllowedProviders {
			providerSupported := false
			for _, cap := range cat.RequiredCapabilities {
				if contains(supportedProviders[cap], provider) {
					providerSupported = true
					break
				}
			}
			if !providerSupported {
				return fmt.Errorf(
					"catalogue %q allows provider %q but no required capability supports it",
					name, provider,
				)
			}
		}
	}
	return nil
}

// ValidateParams checks that request dimensions respect catalog constraints.
func (c *TemplateCatalog) ValidateParams(environment, workloadProfile, availability, residency string) error {
	if len(c.Constraints.AllowedEnvironments) > 0 && !contains(c.Constraints.AllowedEnvironments, environment) {
		return fmt.Errorf("environment %q not allowed by template %q", environment, c.Name)
	}
	if len(c.Constraints.AllowedWorkloadProfiles) > 0 && !contains(c.Constraints.AllowedWorkloadProfiles, workloadProfile) {
		return fmt.Errorf("workload_profile %q not allowed by template %q", workloadProfile, c.Name)
	}
	if len(c.Constraints.AllowedAvailabilities) > 0 && !contains(c.Constraints.AllowedAvailabilities, availability) {
		return fmt.Errorf("availability %q not allowed by template %q", availability, c.Name)
	}
	if len(c.Constraints.AllowedResidencies) > 0 && !contains(c.Constraints.AllowedResidencies, residency) {
		return fmt.Errorf("residency %q not allowed by template %q", residency, c.Name)
	}
	return nil
}

// ValidateProvider checks if a provider is allowed by the catalog constraints.
func (c *TemplateCatalog) ValidateProvider(provider string) error {
	if len(c.Constraints.AllowedProviders) == 0 {
		return nil
	}
	if !contains(c.Constraints.AllowedProviders, provider) {
		return fmt.Errorf(
			"provider %q not allowed for template %q, supported: %v",
			provider, c.Name, c.Constraints.AllowedProviders,
		)
	}
	return nil
}

// CapabilityInfo represents a capability's basic information for validation.
type CapabilityInfo struct {
	Enabled  bool
	Provider string
}

// ValidateCapabilities validates enabled capabilities against catalog constraints.
func (c *TemplateCatalog) ValidateCapabilities(capabilities map[string]CapabilityInfo, defaultProvider string) error {
	// Check required capabilities are enabled
	for _, req := range c.RequiredCapabilities {
		cap, ok := capabilities[req]
		if !ok || !cap.Enabled {
			return fmt.Errorf("capability %q is required by template %q", req, c.Name)
		}
	}

	// Check forbidden capabilities are not enabled
	for _, forbidden := range c.ForbiddenCapabilities {
		cap, ok := capabilities[forbidden]
		if ok && cap.Enabled {
			return fmt.Errorf("capability %q is forbidden by template %q", forbidden, c.Name)
		}
	}

	// Validate each enabled capability's provider against catalogue constraints
	for _, cap := range capabilities {
		if !cap.Enabled {
			continue
		}
		provider := cap.Provider
		if provider == "" {
			provider = defaultProvider
		}
		if provider == "" {
			provider = "aws" // ModuleDefaultProvider fallback
		}
		if err := c.ValidateProvider(provider); err != nil {
			return err
		}
	}

	return nil
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
