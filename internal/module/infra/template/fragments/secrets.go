// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type SecretsFragment struct{}

func (f *SecretsFragment) Name() string { return "secrets" }

func (f *SecretsFragment) Applicable(params *template.ProvisionParams) bool {
	return params.Secrets.Enabled
}

func (f *SecretsFragment) Validate(params *template.ProvisionParams) error {
	s := params.Secrets
	if !s.Enabled {
		return nil
	}
	if s.Provider == "" {
		return fmt.Errorf("secrets.provider required when secrets.enabled is true")
	}
	if s.KMSKeyType != "" && s.KMSKeyType != "aws-managed" && s.KMSKeyType != "customer-managed" {
		return fmt.Errorf("secrets.kms_key_type must be aws-managed or customer-managed")
	}
	return nil
}

func (f *SecretsFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	s := params.Secrets

	spec := map[string]any{
		"enabled":  true,
		"provider": s.Provider,
		"region":   params.PrimaryRegion,
	}

	kmsType := resolve.Coalesce(s.KMSKeyType, "aws-managed")
	spec["kmsKey"] = map[string]any{
		"type":            kmsType,
		"rotationEnabled": true,
	}

	if kmsType == "customer-managed" {
		spec["kmsKey"].(map[string]any)["rotationDays"] = resolve.CoalesceInt(s.KMSRotationDays, 365)
		spec["kmsKey"].(map[string]any)["deletionWindowDays"] = 30
	}

	if s.SecretCount > 0 {
		spec["secretCount"] = s.SecretCount
		spec["namePrefix"] = resolve.SafeName(params.RequestName)
	}

	if s.AutoRotation {
		spec["autoRotation"] = map[string]any{
			"enabled":      true,
			"intervalDays": resolve.CoalesceInt(s.RotationIntervalDays, 90),
		}
	}

	if params.DRRequired && params.SecondaryRegion != "" {
		spec["replication"] = map[string]any{
			"enabled":           true,
			"destinationRegion": params.SecondaryRegion,
		}
	}

	return map[string]any{"secrets": spec}, nil
}
