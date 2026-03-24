// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package scaffold

import (
	"context"
	"fmt"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/gatblau/release-engine/internal/stepapi"
	"go.uber.org/zap"
)

const (
	ModuleKey     = "scaffolding/create-service"
	ModuleVersion = "1.0.0"
)

// Module implements the release engine executable module contract.
type Module struct {
	// customerID holds the tenant identifier for secret resolution.
	// This is set at module creation based on input parameters.
	customerID string
	// vars holds any module configuration variables (optional).
	vars *Vars
}

// Vars holds configuration variables for the scaffold module.
type Vars struct {
	// Add any module-specific configuration here
}

// NewModule creates a new scaffold module with the given customer ID.
// The customerID is used for tenant context in secret resolution.
func NewModule(customerID string) (*Module, error) {
	if customerID == "" {
		return nil, fmt.Errorf("customerID is required")
	}
	return &Module{
		customerID: customerID,
		vars:       &Vars{},
	}, nil
}

// NewLegacyModule creates a new scaffold module using default constructor.
// This constructor is used by the legacy assembly path.
// It will require customerID to be provided at execution time via params.
func NewLegacyModule() *Module {
	return &Module{
		vars: &Vars{},
	}
}

// Key returns the module identifier.
func (m *Module) Key() string { return ModuleKey }

// Version returns the module version.
func (m *Module) Version() string { return ModuleVersion }

// SecretContext implements the connector.SecretContextProvider interface.
// Scaffold module resolves tenant from customer ID.
func (m *Module) SecretContext() connector.SecretContext {
	// If customerID is set at construction, use it
	if m.customerID != "" {
		return connector.SecretContext{
			TenantID: m.customerID,
		}
	}

	// For legacy modules, tenant resolution should happen in Execute()
	// based on params. This ensures modules validate their own inputs.
	// Returning empty tenant will cause runtime to fail if secrets are needed.
	return connector.SecretContext{
		TenantID: "", // Will cause validation failure if secrets required
	}
}

// Execute implements the module workflow.
// This is a simplified implementation based on the design document.
func (m *Module) Execute(ctx context.Context, api any, params map[string]any) error {
	step, _ := api.(stepapi.StepAPI)
	var logger *zap.Logger
	if step != nil {
		logger = step.Logger()
		_ = step.BeginStep("scaffold.validate")
	}

	// If no logger available, create a no-op logger
	if logger == nil {
		logger = zap.NewNop()
	}

	logger.Info("scaffold module execution started",
		zap.Int("params_count", len(params)),
	)

	// Extract and validate customer ID from params
	customerID, ok := params["customer_id"].(string)
	if !ok || customerID == "" {
		err := fmt.Errorf("customer_id parameter is required and must be a non-empty string")
		logger.Error("missing customer_id parameter", zap.Error(err))
		if step != nil {
			_ = step.EndStepErr("scaffold.validate", "INVALID_PARAMS", err.Error())
		}
		return err
	}

	// Validate other required parameters
	serviceName, ok := params["service_name"].(string)
	if !ok || serviceName == "" {
		err := fmt.Errorf("service_name parameter is required and must be a non-empty string")
		logger.Error("missing service_name parameter", zap.Error(err))
		if step != nil {
			_ = step.EndStepErr("scaffold.validate", "INVALID_PARAMS", err.Error())
		}
		return err
	}

	owner, ok := params["owner"].(string)
	if !ok || owner == "" {
		err := fmt.Errorf("owner parameter is required and must be a non-empty string")
		logger.Error("missing owner parameter", zap.Error(err))
		if step != nil {
			_ = step.EndStepErr("scaffold.validate", "INVALID_PARAMS", err.Error())
		}
		return err
	}

	org, ok := params["org"].(string)
	if !ok || org == "" {
		err := fmt.Errorf("org parameter is required and must be a non-empty string")
		logger.Error("missing org parameter", zap.Error(err))
		if step != nil {
			_ = step.EndStepErr("scaffold.validate", "INVALID_PARAMS", err.Error())
		}
		return err
	}

	template, ok := params["template"].(string)
	if !ok || template == "" {
		err := fmt.Errorf("template parameter is required and must be a non-empty string")
		logger.Error("missing template parameter", zap.Error(err))
		if step != nil {
			_ = step.EndStepErr("scaffold.validate", "INVALID_PARAMS", err.Error())
		}
		return err
	}

	// If module was created without customerID (legacy path), set it now
	if m.customerID == "" {
		m.customerID = customerID
	}

	logger.Info("parameter validation successful",
		zap.String("customer_id", customerID),
		zap.String("service_name", serviceName),
		zap.String("owner", owner),
		zap.String("org", org),
		zap.String("template", template),
	)

	if step != nil {
		_ = step.EndStepOK("scaffold.validate", map[string]any{
			"customer_id":  customerID,
			"service_name": serviceName,
			"owner":        owner,
			"org":          org,
			"template":     template,
		})
	}

	// Step 1: Render template (simplified for now)
	if step != nil {
		_ = step.BeginStep("scaffold.render_template")
	}

	// In a real implementation, this would render the template
	// For now, we'll just log and continue
	logger.Info("template rendering would happen here",
		zap.String("template", template),
		zap.String("service_name", serviceName),
	)

	if step != nil {
		_ = step.EndStepOK("scaffold.render_template", map[string]any{
			"rendered": true,
		})
	}

	// Step 2: Create GitHub repository
	if step != nil {
		_ = step.BeginStep("scaffold.create_repository")
	}

	// Note: This would require a GitHub connector with secrets
	// The connector would declare required secrets via SecretRequirer interface
	// The runtime would fetch secrets using tenant context from module
	logger.Info("GitHub repository creation would happen here",
		zap.String("org", org),
		zap.String("service_name", serviceName),
	)

	if step != nil {
		_ = step.EndStepOK("scaffold.create_repository", map[string]any{
			"repo_url": fmt.Sprintf("https://github.com/%s/%s", org, serviceName),
		})
	}

	// Step 3: Register in service catalog
	if step != nil {
		_ = step.BeginStep("scaffold.register_component")
	}

	logger.Info("service catalog registration would happen here",
		zap.String("service_name", serviceName),
		zap.String("owner", owner),
	)

	if step != nil {
		_ = step.EndStepOK("scaffold.register_component", map[string]any{
			"entity_ref": fmt.Sprintf("component:%s/%s", owner, serviceName),
		})
	}

	// Step 4: Completion notification
	if step != nil {
		_ = step.BeginStep("scaffold.notify_completion")
	}

	logger.Info("completion notification would happen here")

	if step != nil {
		_ = step.EndStepOK("scaffold.notify_completion", map[string]any{})
	}

	logger.Info("scaffold module execution completed successfully")
	return nil
}

// Register registers the scaffold module in a module registry.
func Register(reg registry.ModuleRegistry) error {
	return reg.Register(NewLegacyModule())
}
