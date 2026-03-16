// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type IdentityFragment struct{}

func (f *IdentityFragment) Name() string { return "identity" }

func (f *IdentityFragment) Applicable(params *template.ProvisionParams) bool {
	return params.Identity.Enabled
}

func (f *IdentityFragment) Validate(params *template.ProvisionParams) error {
	id := params.Identity
	if !id.Enabled {
		return nil
	}
	if id.Type == "" {
		return fmt.Errorf("identity.type required when identity.enabled is true")
	}
	if id.Type == "federated" && id.FederationProvider == "" {
		return fmt.Errorf("identity.federation_provider required when identity.type is federated")
	}
	for i, p := range id.Policies {
		if p.Effect == "" || p.Resource == "" {
			return fmt.Errorf("identity.policies[%d]: effect and resource are required", i)
		}
	}
	return nil
}

func (f *IdentityFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	id := params.Identity

	spec := map[string]any{
		"enabled": true,
		"type":    id.Type,
	}

	if id.ServiceAccountCount > 0 {
		spec["serviceAccounts"] = map[string]any{
			"count":      id.ServiceAccountCount,
			"namePrefix": resolve.SafeName(params.RequestName),
		}
	}

	if len(id.Policies) > 0 {
		policies := make([]map[string]any, len(id.Policies))
		for i, p := range id.Policies {
			policies[i] = map[string]any{
				"effect":   p.Effect,
				"actions":  p.Actions,
				"resource": p.Resource,
			}
			if len(p.Conditions) > 0 {
				policies[i]["conditions"] = p.Conditions
			}
		}
		spec["policies"] = policies
	}

	if id.Type == "federated" {
		spec["federation"] = map[string]any{
			"provider": id.FederationProvider,
			"audience": id.FederationAudience,
		}
	}

	spec["permissionBoundary"] = resolve.PermissionBoundary(params.Tenant, params.Environment)

	return map[string]any{"identity": spec}, nil
}
