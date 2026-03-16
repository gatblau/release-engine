// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type KubernetesFragment struct{}

func (f *KubernetesFragment) Name() string { return "kubernetes" }

func (f *KubernetesFragment) Applicable(params *template.ProvisionParams) bool {
	return params.Kubernetes.Enabled
}

func (f *KubernetesFragment) Validate(params *template.ProvisionParams) error {
	k := params.Kubernetes
	if !k.Enabled {
		return nil
	}
	if k.Tier == "" {
		return fmt.Errorf("kubernetes.tier required when kubernetes.enabled is true")
	}
	if k.Size == "" {
		return fmt.Errorf("kubernetes.size required when kubernetes.enabled is true")
	}
	if params.Availability == "critical" && !k.MultiAZ {
		return fmt.Errorf("kubernetes.multi_az must be true when availability is critical")
	}
	return nil
}

func (f *KubernetesFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	k := params.Kubernetes

	nodePool := resolve.KubernetesNodePool(k.Size, k.Tier)

	spec := map[string]any{
		"enabled":  true,
		"tier":     k.Tier,
		"version":  resolve.KubernetesVersion(k.Tier, k.Version),
		"multiAZ":  k.MultiAZ,
		"nodePool": nodePool,
		"region":   params.PrimaryRegion,
	}

	if k.NodePoolCount > 1 {
		spec["additionalNodePools"] = k.NodePoolCount - 1
	}

	if k.Tier == "advanced" {
		spec["addons"] = map[string]any{
			"clusterAutoscaler": true,
			"metricsServer":     true,
			"certManager":       true,
		}
	}

	return map[string]any{"kubernetes": spec}, nil
}
