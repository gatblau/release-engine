// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package resolve

// MaintenanceWindow returns a maintenance window for environment and region.
func MaintenanceWindow(region, environment string) string {
	switch environment {
	case "production":
		return "sun:03:00-sun:04:00"
	default:
		return "sat:03:00-sat:04:00"
	}
}

// BackupWindow returns a backup window for a region.
func BackupWindow(region string) string {
	return "02:00-03:00"
}

// RPO returns recovery point objective based on availability.
func RPO(availability string) string {
	switch availability {
	case "critical":
		return "1h"
	case "high":
		return "4h"
	default:
		return "24h"
	}
}

// RTO returns recovery time objective based on availability.
func RTO(availability string) string {
	switch availability {
	case "critical":
		return "1h"
	case "high":
		return "4h"
	default:
		return "24h"
	}
}
