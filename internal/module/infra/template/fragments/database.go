// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type DatabaseFragment struct{}

func (f *DatabaseFragment) Name() string { return "database" }

func (f *DatabaseFragment) Applicable(params *template.ProvisionParams) bool {
	return params.Database.Enabled
}

func (f *DatabaseFragment) Validate(params *template.ProvisionParams) error {
	db := params.Database
	if !db.Enabled {
		return nil
	}
	if db.Engine == "" {
		return fmt.Errorf("database.engine required when database.enabled is true")
	}
	if db.Tier == "" {
		return fmt.Errorf("database.tier required when database.enabled is true")
	}
	if db.StorageGiB == 0 {
		return fmt.Errorf("database.storage_gib required when database.enabled is true")
	}
	if db.Tier == "serverless" && !isAuroraEngine(db.Engine) {
		return fmt.Errorf("serverless tier only supported for aurora engines")
	}
	if db.StorageType == "provisioned" && db.IOPS == 0 {
		return fmt.Errorf("database.iops required when storage_type is provisioned")
	}
	if params.DataClassification == "restricted" && !db.BackupEnabled {
		return fmt.Errorf("database.backup_enabled must be true for restricted data classification")
	}
	return nil
}

func (f *DatabaseFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	db := params.Database

	spec := map[string]any{
		"enabled":    true,
		"engine":     db.Engine,
		"tier":       db.Tier,
		"storageGiB": db.StorageGiB,
		"region":     params.PrimaryRegion,
		"encrypted":  true,
	}

	storageType := resolve.Coalesce(db.StorageType, "ssd")
	spec["storageType"] = resolve.DiskTypeToCloud(storageType)
	if db.StorageType == "provisioned" {
		spec["iops"] = db.IOPS
	}

	switch db.Tier {
	case "standard":
		spec["instanceClass"] = resolve.DBInstanceClass(db.Engine, "standard")
		spec["multiAZ"] = false
		spec["replicas"] = 0
	case "highly-available":
		spec["instanceClass"] = resolve.DBInstanceClass(db.Engine, "ha")
		spec["multiAZ"] = true
		spec["replicas"] = resolve.ReadReplicas(params.Availability)
	case "serverless":
		spec["minCapacity"] = resolve.ServerlessMinACU(params.WorkloadProfile)
		spec["maxCapacity"] = resolve.ServerlessMaxACU(params.WorkloadProfile)
	}

	if db.BackupEnabled {
		retention := db.BackupRetentionDays
		if retention == 0 {
			retention = resolve.DefaultBackupRetention(params.DataClassification)
		}
		spec["backup"] = map[string]any{
			"enabled":             true,
			"retentionDays":       retention,
			"window":              resolve.BackupWindow(params.PrimaryRegion),
			"pointInTimeRecovery": db.PointInTimeRecovery || params.DataClassification == "restricted",
		}
	}

	mw := db.MaintenanceWindow
	if mw == "" {
		mw = resolve.MaintenanceWindow(params.PrimaryRegion, params.Environment)
	}
	spec["maintenanceWindow"] = mw

	if db.ParameterGroup != "" {
		spec["parameterGroup"] = db.ParameterGroup
	}

	if params.DRRequired && params.SecondaryRegion != "" {
		spec["disasterRecovery"] = map[string]any{
			"enabled":         true,
			"secondaryRegion": params.SecondaryRegion,
			"replicationType": resolve.DBReplicationType(db.Engine),
		}
	}

	return map[string]any{"database": spec}, nil
}

func isAuroraEngine(engine string) bool {
	return engine == "aurora-postgres" || engine == "aurora-mysql"
}
