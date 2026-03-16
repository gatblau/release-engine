// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import "github.com/gatblau/release-engine/internal/module/infra/template"

// ComplianceFragment renders always-on compliance controls and guardrails.
type ComplianceFragment struct{}

func (f *ComplianceFragment) Name() string                                { return "compliance" }
func (f *ComplianceFragment) Applicable(_ *template.ProvisionParams) bool { return true }
func (f *ComplianceFragment) Validate(_ *template.ProvisionParams) error  { return nil }

func (f *ComplianceFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	compliance := map[string]any{
		"dataResidency":          params.Residency,
		"dataClassification":     params.DataClassification,
		"encryptionAtRest":       true,
		"encryptionInTransit":    true,
		"auditLogging":           true,
		"disasterRecovery":       params.DRRequired,
		"backupRequired":         params.BackupRequired,
		"recoveryPointObjective": map[string]any{"target": params.Availability},
	}

	if params.Residency == "eu" {
		compliance["gdprCompliant"] = true
	}

	if params.Availability == "critical" {
		compliance["changeManagement"] = "full"
		compliance["backupVerification"] = true
	}

	if len(params.Compliance) > 0 {
		compliance["frameworks"] = params.Compliance
	}

	return map[string]any{"compliance": compliance}, nil
}
