// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type CacheFragment struct{}

func (f *CacheFragment) Name() string { return "cache" }

func (f *CacheFragment) Applicable(params *template.ProvisionParams) bool {
	return params.Cache.Enabled
}

func (f *CacheFragment) Validate(params *template.ProvisionParams) error {
	c := params.Cache
	if !c.Enabled {
		return nil
	}
	if c.Engine == "" {
		return fmt.Errorf("cache.engine required when cache.enabled is true")
	}
	if c.Tier == "" {
		return fmt.Errorf("cache.tier required when cache.enabled is true")
	}
	if c.Tier == "clustered" && c.ReplicaCount < 1 {
		return fmt.Errorf("cache.replica_count must be >= 1 for clustered tier")
	}
	if c.Engine == "memcached" && c.Tier == "clustered" {
		return fmt.Errorf("memcached does not support clustered tier")
	}
	if params.Availability == "critical" && c.ReplicaCount < 2 {
		return fmt.Errorf("cache.replica_count must be >= 2 for critical availability")
	}
	return nil
}

func (f *CacheFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	c := params.Cache

	nodeType := c.NodeType
	if nodeType == "" {
		nodeType = resolve.CacheNodeType(c.Engine, params.WorkloadProfile)
	}

	spec := map[string]any{
		"enabled":           true,
		"engine":            c.Engine,
		"tier":              c.Tier,
		"nodeType":          nodeType,
		"version":           resolve.CacheEngineVersion(c.Engine, c.Version),
		"replicaCount":      c.ReplicaCount,
		"region":            params.PrimaryRegion,
		"encrypted":         true,
		"transitEncryption": true,
		"authEnabled":       true,
	}

	if c.Tier == "clustered" {
		spec["clusterMode"] = map[string]any{
			"enabled":    true,
			"shardCount": resolve.CacheShardCount(c.Engine, params.WorkloadProfile),
		}
	}

	if c.SnapshotRetentionDays > 0 {
		spec["snapshot"] = map[string]any{
			"enabled":       true,
			"retentionDays": c.SnapshotRetentionDays,
			"window":        resolve.BackupWindow(params.PrimaryRegion),
		}
	}

	spec["maintenanceWindow"] = resolve.MaintenanceWindow(params.PrimaryRegion, params.Environment)

	return map[string]any{"cache": spec}, nil
}
