// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package template

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
)

// Fragment is implemented by each capability/policy renderer.
type Fragment interface {
	Name() string
	Applicable(params *ProvisionParams) bool
	Validate(params *ProvisionParams) error
	Render(params *ProvisionParams) (map[string]any, error)
}

// Engine assembles XR manifests from parameters via fragment composition.
type Engine struct {
	fragments []Fragment
	catalogs  map[string]*catalog.TemplateCatalog
}

// NewEngine creates an engine with the provided fragments.
func NewEngine(frags ...Fragment) *Engine {
	return &Engine{
		fragments: frags,
	}
}

// NewEngineWithCatalog creates an engine with an optional catalogue map.
func NewEngineWithCatalog(catalogs map[string]*catalog.TemplateCatalog, frags ...Fragment) *Engine {
	return &Engine{
		fragments: frags,
		catalogs:  catalogs,
	}
}

// Render produces the final XR manifest YAML from parameters.
func (e *Engine) Render(params *ProvisionParams) ([]byte, error) {
	if err := e.applyCatalog(params); err != nil {
		return nil, err
	}

	if err := Validate(params); err != nil {
		return nil, fmt.Errorf("global validation failed: %w", err)
	}

	if err := validateAtLeastOneCapability(params); err != nil {
		return nil, err
	}

	if err := validateCrossCutting(params); err != nil {
		return nil, err
	}

	specParts := make(map[string]any)
	capabilityParts := make(map[string]any)
	for _, frag := range e.fragments {
		if !frag.Applicable(params) {
			continue
		}

		if err := frag.Validate(params); err != nil {
			return nil, fmt.Errorf("fragment %s validation failed: %w", frag.Name(), err)
		}

		part, err := frag.Render(params)
		if err != nil {
			return nil, fmt.Errorf("fragment %s render failed: %w", frag.Name(), err)
		}

		for k, v := range part {
			specParts[k] = v
			if _, ok := paramBuilders[k]; ok {
				capabilityParts[k] = v
			}
		}
	}

	// Inject FinOps tags from TagsFragment into capabilityParts so BuildXRs can
	// forward them to every XR spec.parameters.tags field.
	// TagsFragment renders either {"tags": {"tags": <map[string]string>}} or
	// directly map[string]string depending on the renderer implementation.
	if tagsRaw, ok := specParts["tags"]; ok {
		switch tags := tagsRaw.(type) {
		case map[string]any:
			if innerTags, ok := tags["tags"].(map[string]string); ok && len(innerTags) > 0 {
				capabilityParts["tags"] = innerTags
			}
		case map[string]string:
			if len(tags) > 0 {
				capabilityParts["tags"] = tags
			}
		}
	}

	docs, err := BuildXRs(params, capabilityParts)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no supported Crossplane XRD capability enabled")
	}
	return marshalDeterministicDocuments(docs)
}

func validateAtLeastOneCapability(params *ProvisionParams) error {
	hasCapability := params.Kubernetes.Enabled ||
		params.VM.Enabled ||
		params.Database.Enabled ||
		params.ObjectStore.Enabled ||
		params.BlockStore.Enabled ||
		params.FileStore.Enabled ||
		params.VPC.Enabled ||
		params.Messaging.Enabled ||
		params.Cache.Enabled ||
		params.DNS.Enabled ||
		params.LoadBalancer.Enabled ||
		params.CDN.Enabled ||
		params.Identity.Enabled ||
		params.Secrets.Enabled ||
		params.Observability.Enabled

	if !hasCapability {
		return fmt.Errorf("at least one infrastructure capability must be enabled")
	}
	return nil
}

func validateCrossCutting(params *ProvisionParams) error {
	if params.VM.Enabled && params.Kubernetes.Enabled {
		return fmt.Errorf("cannot enable both kubernetes and virtual machines")
	}
	if params.BlockStore.Enabled && !params.VM.Enabled {
		return fmt.Errorf("block_store requires vm to be enabled")
	}
	if params.Availability == "critical" && !params.Observability.Enabled {
		return fmt.Errorf("observability must be enabled for critical availability")
	}
	return nil
}

func (e *Engine) applyCatalog(params *ProvisionParams) error {
	if len(e.catalogs) == 0 {
		return nil
	}

	catName := params.CatalogueItem
	if catName == "" {
		return nil
	}

	cat, ok := e.catalogs[catName]
	if !ok {
		return fmt.Errorf("catalog %q not found", catName)
	}

	applyCatalogDefaults(params, cat)

	if err := cat.ValidateParams(params.Environment, params.WorkloadProfile, params.Availability, params.Residency); err != nil {
		return fmt.Errorf("catalog validation failed: %w", err)
	}

	// Build capability info map for validation
	capabilities := map[string]catalog.CapabilityInfo{
		"blockStorage":  {Enabled: params.BlockStore.Enabled, Provider: params.BlockStore.Provider},
		"cache":         {Enabled: params.Cache.Enabled, Provider: params.Cache.Provider},
		"cdn":           {Enabled: params.CDN.Enabled, Provider: params.CDN.Provider},
		"database":      {Enabled: params.Database.Enabled, Provider: params.Database.Provider},
		"dns":           {Enabled: params.DNS.Enabled, Provider: params.DNS.Provider},
		"fileStorage":   {Enabled: params.FileStore.Enabled, Provider: params.FileStore.Provider},
		"identity":      {Enabled: params.Identity.Enabled, Provider: params.Identity.Provider},
		"kubernetes":    {Enabled: params.Kubernetes.Enabled, Provider: params.Kubernetes.Provider},
		"loadBalancer":  {Enabled: params.LoadBalancer.Enabled, Provider: params.LoadBalancer.Provider},
		"messaging":     {Enabled: params.Messaging.Enabled, Provider: params.Messaging.Provider},
		"objectStorage": {Enabled: params.ObjectStore.Enabled, Provider: params.ObjectStore.Provider},
		"observability": {Enabled: params.Observability.Enabled, Provider: params.Observability.Provider},
		"secrets":       {Enabled: params.Secrets.Enabled, Provider: params.Secrets.Provider},
		"vm":            {Enabled: params.VM.Enabled, Provider: params.VM.Provider},
		"vpc":           {Enabled: params.VPC.Enabled, Provider: params.VPC.Provider},
	}

	if err := cat.ValidateCapabilities(capabilities, params.DefaultProvider); err != nil {
		return fmt.Errorf("catalog capability validation failed: %w", err)
	}

	return nil
}

func applyCatalogDefaults(params *ProvisionParams, cat *catalog.TemplateCatalog) {
	if cat == nil || cat.Defaults == nil {
		return
	}

	if params.Kubernetes.Tier == "" {
		if v, ok := cat.Defaults["kubernetes_tier"].(string); ok {
			params.Kubernetes.Tier = v
		}
	}
	if params.Database.Engine == "" {
		if v, ok := cat.Defaults["database_engine"].(string); ok {
			params.Database.Engine = v
		}
	}
	if params.Database.Tier == "" {
		if v, ok := cat.Defaults["database_tier"].(string); ok {
			params.Database.Tier = v
		}
	}
	if !params.Database.BackupEnabled {
		if v, ok := cat.Defaults["database_backup_enabled"].(bool); ok {
			params.Database.BackupEnabled = v
		}
	}
	if params.Cache.Engine == "" {
		if v, ok := cat.Defaults["cache_engine"].(string); ok {
			params.Cache.Engine = v
		}
	}
	if params.Cache.Tier == "" {
		if v, ok := cat.Defaults["cache_tier"].(string); ok {
			params.Cache.Tier = v
		}
	}
	if params.ObjectStore.Class == "" {
		if v, ok := cat.Defaults["object_storage_class"].(string); ok {
			params.ObjectStore.Class = v
		}
	}
	if !params.ObjectStore.Versioning {
		if v, ok := cat.Defaults["object_storage_versioning"].(bool); ok {
			params.ObjectStore.Versioning = v
		}
	}
}
