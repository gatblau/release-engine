// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package template

import (
	"fmt"
	"sort"

	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
	"gopkg.in/yaml.v3"
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
		}
	}

	xr := BuildXR(params, specParts)
	return marshalDeterministic(xr)
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

	catName := params.TemplateName
	if catName == "" {
		return nil
	}

	cat, ok := e.catalogs[catName]
	if !ok {
		return fmt.Errorf("catalog %q not found", catName)
	}

	applyCatalogDefaults(params, cat)

	if params.CompositionRef == "" {
		params.CompositionRef = cat.CompositionRef
	}

	if err := cat.ValidateParams(params.Environment, params.WorkloadProfile, params.Availability, params.Residency); err != nil {
		return fmt.Errorf("catalog validation failed: %w", err)
	}

	if err := validateCatalogCapabilities(params, cat); err != nil {
		return fmt.Errorf("catalog capability validation failed: %w", err)
	}

	return nil
}

func applyCatalogDefaults(params *ProvisionParams, cat *catalog.TemplateCatalog) {
	if cat == nil || cat.Defaults == nil {
		return
	}

	if params.CompositionRef == "" {
		if v, ok := cat.Defaults["composition_ref"].(string); ok && v != "" {
			params.CompositionRef = v
		}
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

func validateCatalogCapabilities(params *ProvisionParams, cat *catalog.TemplateCatalog) error {
	capabilityMap := map[string]bool{
		"kubernetes":     params.Kubernetes.Enabled,
		"vm":             params.VM.Enabled,
		"database":       params.Database.Enabled,
		"object_storage": params.ObjectStore.Enabled,
		"object_store":   params.ObjectStore.Enabled,
		"block_storage":  params.BlockStore.Enabled,
		"block_store":    params.BlockStore.Enabled,
		"file_storage":   params.FileStore.Enabled,
		"file_store":     params.FileStore.Enabled,
		"vpc":            params.VPC.Enabled,
		"messaging":      params.Messaging.Enabled,
		"cache":          params.Cache.Enabled,
		"dns":            params.DNS.Enabled,
		"load_balancer":  params.LoadBalancer.Enabled,
		"cdn":            params.CDN.Enabled,
		"identity":       params.Identity.Enabled,
		"secrets":        params.Secrets.Enabled,
		"observability":  params.Observability.Enabled,
	}

	for _, forbidden := range cat.ForbiddenCapabilities {
		if capabilityMap[forbidden] {
			return fmt.Errorf("capability %q is forbidden for template %q", forbidden, cat.Name)
		}
	}

	for _, required := range cat.RequiredCapabilities {
		if !capabilityMap[required] {
			return fmt.Errorf("capability %q is required for template %q", required, cat.Name)
		}
	}

	return nil
}

// marshalDeterministic produces YAML with sorted map keys for stable output.
func marshalDeterministic(v any) ([]byte, error) {
	node := &yaml.Node{}
	raw, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, node); err != nil {
		return nil, err
	}
	sortYAMLNode(node)
	return yaml.Marshal(node)
}

func sortYAMLNode(node *yaml.Node) {
	if node == nil {
		return
	}

	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			sortYAMLNode(child)
		}
		return
	}

	if node.Kind == yaml.MappingNode {
		pairs := make([]struct{ Key, Value *yaml.Node }, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			pairs[i/2] = struct{ Key, Value *yaml.Node }{node.Content[i], node.Content[i+1]}
		}
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].Key.Value < pairs[j].Key.Value
		})
		for i, p := range pairs {
			node.Content[i*2] = p.Key
			node.Content[i*2+1] = p.Value
			sortYAMLNode(p.Value)
		}
		return
	}

	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			sortYAMLNode(child)
		}
	}
}
