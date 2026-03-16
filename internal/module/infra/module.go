// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package infra

import (
	"context"
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/registry"
	"gopkg.in/yaml.v3"
)

const (
	ModuleKey     = "infra.provision"
	ModuleVersion = "latest"
)

// Module implements the release engine executable module contract.
type Module struct{}

func NewModule() *Module { return &Module{} }

func (m *Module) Key() string { return ModuleKey }

func (m *Module) Version() string { return ModuleVersion }

type stepAPI interface {
	BeginStep(stepKey string) error
	EndStepOK(stepKey string, output map[string]any) error
	EndStepErr(stepKey, code, msg string) error
	SetContext(key string, value any) error
}

// Execute decodes infra params and renders XR manifests.
func (m *Module) Execute(ctx context.Context, api any, params map[string]any) error {
	_ = ctx

	step, _ := api.(stepAPI)
	if step != nil {
		_ = step.BeginStep("infra.render")
	}

	decoded, err := decodeProvisionParams(params)
	if err != nil {
		if step != nil {
			_ = step.EndStepErr("infra.render", "INFRA_PARAMS_INVALID", err.Error())
		}
		return fmt.Errorf("decode infra params: %w", err)
	}

	out, err := RenderManifests(decoded)
	if err != nil {
		if step != nil {
			_ = step.EndStepErr("infra.render", "INFRA_RENDER_FAILED", err.Error())
		}
		return fmt.Errorf("infra render failed: %w", err)
	}

	if step != nil {
		_ = step.SetContext("infra.manifest", string(out))
		_ = step.EndStepOK("infra.render", map[string]any{
			"manifest_yaml": string(out),
		})
	}

	return nil
}

func decodeProvisionParams(params map[string]any) (*template.ProvisionParams, error) {
	raw, err := yaml.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	var out template.ProvisionParams
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal params: %w", err)
	}
	return &out, nil
}

// Register registers the infra module in a module registry.
func Register(reg registry.ModuleRegistry) error {
	return reg.Register(NewModule())
}
