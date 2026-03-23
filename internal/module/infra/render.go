// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package infra

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
	"github.com/gatblau/release-engine/internal/module/infra/template/fragments"
)

// RenderManifests is the public entry point used by the module runtime.
func RenderManifests(params *template.ProvisionParams) ([]byte, error) {
	catalogs, err := catalog.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("load catalog definitions: %w", err)
	}

	// Validate catalogs against supported providers at startup
	if err := catalog.ValidateCatalogs(catalogs, template.SupportedProviders); err != nil {
		return nil, fmt.Errorf("catalog validation failed: %w", err)
	}

	engine := template.NewEngineWithCatalog(
		catalogs,
		&fragments.TagsFragment{},
		&fragments.ComplianceFragment{},
		&fragments.KubernetesFragment{},
		&fragments.VMFragment{},
		&fragments.DatabaseFragment{},
		&fragments.CacheFragment{},
		&fragments.ObjectStorageFragment{},
		&fragments.BlockStorageFragment{},
		&fragments.FileStorageFragment{},
		&fragments.VPCFragment{},
		&fragments.LoadBalancerFragment{},
		&fragments.DNSFragment{},
		&fragments.CDNFragment{},
		&fragments.MessagingFragment{},
		&fragments.IdentityFragment{},
		&fragments.SecretsFragment{},
		&fragments.ObservabilityFragment{},
	)

	output, err := engine.Render(params)
	if err != nil {
		return nil, fmt.Errorf("render manifests: %w", err)
	}
	return output, nil
}
