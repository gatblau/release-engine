// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type FileStorageFragment struct{}

func (f *FileStorageFragment) Name() string { return "file-storage" }

func (f *FileStorageFragment) Applicable(params *template.ProvisionParams) bool {
	return params.FileStore.Enabled
}

func (f *FileStorageFragment) Validate(params *template.ProvisionParams) error {
	fs := params.FileStore
	if !fs.Enabled {
		return nil
	}
	if fs.ThroughputMode == "provisioned" && fs.ThroughputMiBs == 0 {
		return fmt.Errorf("file_store.throughput_mibs required when throughput_mode is provisioned")
	}
	if params.Availability == "critical" && !fs.MultiAZ {
		return fmt.Errorf("file_store.multi_az must be true when availability is critical")
	}
	return nil
}

func (f *FileStorageFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	fs := params.FileStore

	spec := map[string]any{
		"enabled":         true,
		"performanceMode": resolve.Coalesce(fs.PerformanceMode, "general-purpose"),
		"throughputMode":  resolve.Coalesce(fs.ThroughputMode, "bursting"),
		"protocol":        resolve.Coalesce(fs.Protocol, "nfs"),
		"multiAZ":         fs.MultiAZ,
		"encrypted":       true,
		"region":          params.PrimaryRegion,
	}

	if fs.SizeGiB > 0 {
		spec["sizeGiB"] = fs.SizeGiB
	}

	if fs.ThroughputMode == "provisioned" {
		spec["provisionedThroughputMiBs"] = fs.ThroughputMiBs
	}

	if params.BackupRequired {
		spec["backup"] = map[string]any{
			"enabled": true,
			"window":  resolve.BackupWindow(params.PrimaryRegion),
		}
	}

	if params.DRRequired && params.SecondaryRegion != "" {
		spec["replication"] = map[string]any{
			"enabled":           true,
			"destinationRegion": params.SecondaryRegion,
		}
	}

	return map[string]any{"fileStorage": spec}, nil
}
