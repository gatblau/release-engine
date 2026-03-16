// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type VMFragment struct{}

func (f *VMFragment) Name() string { return "vm" }

func (f *VMFragment) Applicable(params *template.ProvisionParams) bool {
	return params.VM.Enabled
}

func (f *VMFragment) Validate(params *template.ProvisionParams) error {
	vm := params.VM
	if !vm.Enabled {
		return nil
	}
	if vm.Count == 0 {
		return fmt.Errorf("vm.count required when vm.enabled is true")
	}
	if vm.InstanceFamily == "" {
		return fmt.Errorf("vm.instance_family required when vm.enabled is true")
	}
	if vm.Size == "" {
		return fmt.Errorf("vm.size required when vm.enabled is true")
	}
	if vm.OSFamily == "" {
		return fmt.Errorf("vm.os_family required when vm.enabled is true")
	}
	if vm.SpotEnabled && params.Availability == "critical" {
		return fmt.Errorf("spot instances not permitted for critical availability")
	}
	if vm.AutoScaling.Enabled && vm.AutoScaling.MaxCount < vm.AutoScaling.MinCount {
		return fmt.Errorf("vm auto_scaling max_count must be >= min_count")
	}
	if vm.AutoScaling.Enabled && vm.Count > 1 {
		return fmt.Errorf("vm.count must be 1 when auto_scaling is enabled")
	}
	if params.Availability == "critical" && !vm.MultiAZ && !vm.AutoScaling.Enabled {
		return fmt.Errorf("critical availability requires vm.multi_az or vm.auto_scaling")
	}
	for i, d := range vm.AdditionalDisks {
		if d.Type == "provisioned" && d.IOPS == 0 {
			return fmt.Errorf("additional_disks[%d].iops required for provisioned type", i)
		}
	}
	return nil
}

func (f *VMFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	vm := params.VM

	instanceType := resolve.VMInstanceType(vm.InstanceFamily, vm.Size, vm.Arch)
	amiID := resolve.VMAMI(vm.OSFamily, vm.OSImage, vm.Arch, params.PrimaryRegion)

	spec := map[string]any{
		"enabled":      true,
		"count":        vm.Count,
		"instanceType": instanceType,
		"ami":          amiID,
		"arch":         resolve.Coalesce(vm.Arch, "amd64"),
		"region":       params.PrimaryRegion,
		"multiAZ":      vm.MultiAZ,
		"subnetTier":   "private",
	}

	bootGiB := vm.BootDiskGiB
	if bootGiB == 0 {
		bootGiB = resolve.DefaultBootDiskGiB(vm.OSFamily)
	}
	bootType := vm.BootDiskType
	if bootType == "" {
		bootType = "ssd"
	}
	spec["bootDisk"] = map[string]any{
		"sizeGiB":   bootGiB,
		"type":      resolve.DiskTypeToCloud(bootType),
		"encrypted": true,
	}

	if len(vm.AdditionalDisks) > 0 {
		disks := make([]map[string]any, len(vm.AdditionalDisks))
		for i, d := range vm.AdditionalDisks {
			disk := map[string]any{
				"name":      d.Name,
				"sizeGiB":   d.SizeGiB,
				"type":      resolve.DiskTypeToCloud(d.Type),
				"mountPath": d.MountPath,
				"encrypted": true,
			}
			if d.Type == "provisioned" {
				disk["iops"] = d.IOPS
				if d.Throughput > 0 {
					disk["throughput"] = d.Throughput
				}
			}
			disks[i] = disk
		}
		spec["additionalDisks"] = disks
	}

	if vm.SSHKeyName != "" {
		spec["sshKeyName"] = vm.SSHKeyName
	}
	if vm.UserData != "" {
		spec["userData"] = vm.UserData
	}
	if vm.PlacementGroup != "" {
		spec["placementGroup"] = map[string]any{"strategy": vm.PlacementGroup}
	}
	if vm.SpotEnabled {
		spec["spot"] = map[string]any{
			"enabled":  true,
			"maxPrice": vm.SpotMaxPrice,
			"strategy": "capacity-optimized",
		}
	}
	if vm.AutoScaling.Enabled {
		spec["autoScaling"] = map[string]any{
			"enabled":     true,
			"minCount":    vm.AutoScaling.MinCount,
			"maxCount":    vm.AutoScaling.MaxCount,
			"scalePolicy": resolve.Coalesce(vm.AutoScaling.ScalePolicy, "cpu"),
			"targetCPU":   resolve.CoalesceInt(vm.AutoScaling.TargetCPU, 70),
		}
		spec["count"] = vm.AutoScaling.MinCount
	}

	spec["hardening"] = map[string]any{
		"imdsV2Only":          true,
		"ssmAgent":            true,
		"rootVolumeEncrypted": true,
	}

	return map[string]any{"vm": spec}, nil
}
