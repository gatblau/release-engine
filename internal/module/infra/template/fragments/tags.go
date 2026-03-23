// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import "github.com/gatblau/release-engine/internal/module/infra/template"

// TagsFragment renders standard labels/tags and merges user-defined extras.
type TagsFragment struct{}

func (f *TagsFragment) Name() string                                { return "tags" }
func (f *TagsFragment) Applicable(_ *template.ProvisionParams) bool { return true }
func (f *TagsFragment) Validate(_ *template.ProvisionParams) error  { return nil }

func (f *TagsFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	tags := map[string]string{
		// Mandatory FinOps tags
		"cost-centre": params.CostCentre,
		"service":     params.RequestName, // Use RequestName as service identifier
		"environment": params.Environment,
		"owner":       params.Owner,
		"managed-by":  "release-engine",

		// Existing tags for backward compatibility
		"tenant":         params.Tenant,
		"catalogue-item": params.CatalogueItem,
	}

	// Optional FinOps tags (add only if not empty)
	if params.BusinessUnit != "" {
		tags["business-unit"] = params.BusinessUnit
	}
	if params.Project != "" {
		tags["project"] = params.Project
	}
	if params.DataClassification != "" {
		tags["data-classification"] = params.DataClassification
	}
	if params.TTL != "" {
		tags["ttl"] = params.TTL
	}

	// Merge user-defined extra tags (extra_tags overrides any automatic tags)
	for k, v := range params.ExtraTags {
		tags[k] = v
	}

	return map[string]any{"tags": tags}, nil
}
