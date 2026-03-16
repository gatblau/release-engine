// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package template

import (
	"fmt"
	"strings"
)

// Validate checks global parameter constraints before any fragment runs.
func Validate(params *ProvisionParams) error {
	if params == nil {
		return fmt.Errorf("params are required")
	}

	var errs []string

	if params.ContractVersion == "" {
		errs = append(errs, "contract_version is required")
	}
	if params.RequestName == "" {
		errs = append(errs, "request_name is required")
	}
	if params.Tenant == "" {
		errs = append(errs, "tenant is required")
	}
	if params.Owner == "" {
		errs = append(errs, "owner is required")
	}
	if params.Environment == "" {
		errs = append(errs, "environment is required")
	}
	if params.TemplateName == "" {
		errs = append(errs, "template_name is required")
	}
	if params.CompositionRef == "" {
		errs = append(errs, "composition_ref is required")
	}
	if params.Namespace == "" {
		errs = append(errs, "namespace is required")
	}
	if params.PrimaryRegion == "" {
		errs = append(errs, "primary_region is required")
	}
	if params.Availability == "" {
		errs = append(errs, "availability is required")
	}
	if params.DataClassification == "" {
		errs = append(errs, "data_classification is required")
	}
	if params.IngressMode == "" {
		errs = append(errs, "ingress_mode is required")
	}
	if params.EgressMode == "" {
		errs = append(errs, "egress_mode is required")
	}

	if params.DRRequired && params.SecondaryRegion == "" {
		errs = append(errs, "secondary_region required when dr_required is true")
	}
	if params.Availability == "critical" && !params.DRRequired {
		errs = append(errs, "dr_required must be true when availability is critical")
	}
	if params.Availability == "critical" && !params.BackupRequired {
		errs = append(errs, "backup_required must be true when availability is critical")
	}
	if params.DataClassification == "restricted" && !params.BackupRequired {
		errs = append(errs, "backup_required must be true for restricted data classification")
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
