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
		"tenant":      params.Tenant,
		"owner":       params.Owner,
		"environment": params.Environment,
		"managed-by":  "release-engine",
		"template":    params.TemplateName,
	}

	for k, v := range params.ExtraTags {
		tags[k] = v
	}

	return map[string]any{"tags": tags}, nil
}
