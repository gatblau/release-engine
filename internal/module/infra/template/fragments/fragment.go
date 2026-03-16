// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import "github.com/gatblau/release-engine/internal/module/infra/template"

// Fragment is the contract implemented by each infrastructure capability/policy
// section renderer.
type Fragment interface {
	Name() string
	Applicable(params *template.ProvisionParams) bool
	Validate(params *template.ProvisionParams) error
	Render(params *template.ProvisionParams) (map[string]any, error)
}
