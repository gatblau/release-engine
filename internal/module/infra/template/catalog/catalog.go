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
	CompositionRef        string             `yaml:"composition_ref"`
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

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
