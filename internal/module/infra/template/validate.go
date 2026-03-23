// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package template

import (
	"fmt"
	"strings"
	"time"
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
	if params.CatalogueItem == "" {
		errs = append(errs, "catalogue_item is required")
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
	if params.CostCentre == "" {
		errs = append(errs, "cost_centre is required for FinOps cost allocation")
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

	// Validate TTL format if provided
	if params.TTL != "" {
		if _, err := time.Parse("2006-01-02", params.TTL); err != nil {
			errs = append(errs, fmt.Sprintf("ttl must be in ISO 8601 date format (YYYY-MM-DD), got: %s", params.TTL))
		}
	}

	// Validate cost centre pattern (basic validation - can be enhanced)
	if params.CostCentre != "" && !strings.Contains(params.CostCentre, "-") {
		// Warn but don't fail - some cost centres might not follow this pattern
		// This is just a suggestion for best practice
		// We'll add a log message or handle this appropriately
		fmt.Printf("Warning: cost centre '%s' does not follow expected pattern (contains '-')\n", params.CostCentre)
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
