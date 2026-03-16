// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type BlockStorageFragment struct{}

func (f *BlockStorageFragment) Name() string { return "block-storage" }

func (f *BlockStorageFragment) Applicable(params *template.ProvisionParams) bool {
	return params.BlockStore.Enabled
}

func (f *BlockStorageFragment) Validate(params *template.ProvisionParams) error {
	bs := params.BlockStore
	if !bs.Enabled {
		return nil
	}
	if len(bs.Volumes) == 0 {
		return fmt.Errorf("block_store.volumes required when block_store.enabled is true")
	}
	for i, v := range bs.Volumes {
		if v.Type == "provisioned" && v.IOPS == 0 {
			return fmt.Errorf("block_store.volumes[%d].iops required for provisioned type", i)
		}
		if v.SizeGiB == 0 {
			return fmt.Errorf("block_store.volumes[%d].size_gib is required", i)
		}
	}
	return nil
}

func (f *BlockStorageFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	bs := params.BlockStore

	volumes := make([]map[string]any, len(bs.Volumes))
	for i, v := range bs.Volumes {
		vol := map[string]any{
			"name":      v.Name,
			"sizeGiB":   v.SizeGiB,
			"type":      resolve.DiskTypeToCloud(v.Type),
			"multiAZ":   v.MultiAZ,
			"encrypted": true,
			"region":    params.PrimaryRegion,
		}
		if v.Type == "provisioned" {
			vol["iops"] = v.IOPS
			if v.Throughput > 0 {
				vol["throughput"] = v.Throughput
			}
		}
		if v.SnapshotSchedule != "" {
			vol["snapshotSchedule"] = v.SnapshotSchedule
		}
		volumes[i] = vol
	}

	return map[string]any{
		"blockStorage": map[string]any{
			"enabled": true,
			"volumes": volumes,
		},
	}, nil
}
