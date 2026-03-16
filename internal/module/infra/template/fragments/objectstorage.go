// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
)

type ObjectStorageFragment struct{}

func (f *ObjectStorageFragment) Name() string { return "object-storage" }

func (f *ObjectStorageFragment) Applicable(params *template.ProvisionParams) bool {
	return params.ObjectStore.Enabled
}

func (f *ObjectStorageFragment) Validate(params *template.ProvisionParams) error {
	s := params.ObjectStore
	if !s.Enabled {
		return nil
	}
	if s.Class == "" {
		return fmt.Errorf("object_store.class required when object_store.enabled is true")
	}
	if params.DataClassification == "restricted" && !s.Versioning {
		return fmt.Errorf("object_store.versioning must be true for restricted data classification")
	}
	return nil
}

func (f *ObjectStorageFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	s := params.ObjectStore

	bucketCount := s.BucketCount
	if bucketCount == 0 {
		bucketCount = 1
	}

	spec := map[string]any{
		"enabled":           true,
		"class":             s.Class,
		"versioning":        s.Versioning,
		"region":            params.PrimaryRegion,
		"encrypted":         true,
		"blockPublicAccess": true,
		"bucketCount":       bucketCount,
	}

	switch s.Class {
	case "infrequent":
		spec["lifecycleRules"] = []map[string]any{{"transition": "STANDARD_IA", "days": 30}}
	case "archive":
		spec["lifecycleRules"] = []map[string]any{
			{"transition": "STANDARD_IA", "days": 30},
			{"transition": "GLACIER", "days": 90},
		}
	}

	if s.RetentionDays > 0 {
		spec["retention"] = map[string]any{"mode": "GOVERNANCE", "days": s.RetentionDays}
	}

	if params.DRRequired && params.SecondaryRegion != "" {
		spec["replication"] = map[string]any{
			"enabled":           true,
			"destinationRegion": params.SecondaryRegion,
		}
	}

	if hasCompliance(params.Compliance, "gdpr") {
		spec["objectLock"] = map[string]any{
			"enabled":       true,
			"retentionMode": "GOVERNANCE",
			"retentionDays": maxInt(s.RetentionDays, 90),
		}
	}

	return map[string]any{"objectStorage": spec}, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func hasCompliance(compliance []string, target string) bool {
	for _, c := range compliance {
		if c == target {
			return true
		}
	}
	return false
}
