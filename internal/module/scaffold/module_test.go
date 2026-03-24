// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package scaffold

import (
	"context"
	"testing"

	"github.com/gatblau/release-engine/internal/registry"
	"github.com/gatblau/release-engine/internal/stepapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockStepAPI struct {
	begins   []string
	oks      []string
	errs     []string
	contexts map[string]any
}

func (m *mockStepAPI) BeginStep(stepKey string) error {
	m.begins = append(m.begins, stepKey)
	return nil
}

func (m *mockStepAPI) EndStepOK(stepKey string, _ map[string]any) error {
	m.oks = append(m.oks, stepKey)
	return nil
}

func (m *mockStepAPI) EndStepErr(stepKey, _, _ string) error {
	m.errs = append(m.errs, stepKey)
	return nil
}

func (m *mockStepAPI) SetContext(key string, value any) error {
	if m.contexts == nil {
		m.contexts = map[string]any{}
	}
	m.contexts[key] = value
	return nil
}

func (m *mockStepAPI) CallConnector(ctx context.Context, req stepapi.ConnectorRequest) (*stepapi.ConnectorResult, error) {
	// Simple mock that returns success for any connector call
	return &stepapi.ConnectorResult{
		Status: "success",
		Output: map[string]any{},
	}, nil
}

func (m *mockStepAPI) WaitForApproval(ctx context.Context, req stepapi.ApprovalRequest) (stepapi.ApprovalOutcome, error) {
	return stepapi.ApprovalOutcome{
		Decision:      "approved",
		Approver:      "test-approver",
		Justification: "test approval",
	}, nil
}

func (m *mockStepAPI) GetContext(key string) (any, bool) {
	return nil, false
}

func (m *mockStepAPI) IsCancelled() bool {
	return false
}

func (m *mockStepAPI) Logger() *zap.Logger {
	return zap.NewNop()
}

func validParamsMap() map[string]any {
	return map[string]any{
		"customer_id":  "customer-x",
		"service_name": "my-service",
		"owner":        "team-alpha",
		"org":          "my-org",
		"template":     "go-grpc",
		"callback_url": "https://example.com/callback",
		"job_id":       "job-123",
	}
}

func TestModule_ImplementsRegistryContract(t *testing.T) {
	var _ registry.Module = NewLegacyModule()
}

func TestModule_Metadata(t *testing.T) {
	m := NewLegacyModule()
	assert.Equal(t, ModuleKey, m.Key())
	assert.Equal(t, ModuleVersion, m.Version())
}

func TestModule_SecretContext_WithCustomerID(t *testing.T) {
	module, err := NewModule("customer-x")
	require.NoError(t, err)

	secretCtx := module.SecretContext()
	assert.Equal(t, "customer-x", secretCtx.TenantID)
}

func TestModule_SecretContext_LegacyModule(t *testing.T) {
	// Legacy module without customer ID at construction
	m := NewLegacyModule()
	secretCtx := m.SecretContext()
	assert.Equal(t, "", secretCtx.TenantID)
}

func TestModule_Execute_ValidatesCustomerID(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	params := validParamsMap()
	delete(params, "customer_id")

	err := m.Execute(context.Background(), api, params)
	require.Error(t, err)
	assert.Contains(t, api.errs, "scaffold.validate")
	assert.Contains(t, err.Error(), "customer_id parameter is required")
}

func TestModule_Execute_ValidatesServiceName(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	params := validParamsMap()
	delete(params, "service_name")

	err := m.Execute(context.Background(), api, params)
	require.Error(t, err)
	assert.Contains(t, api.errs, "scaffold.validate")
	assert.Contains(t, err.Error(), "service_name parameter is required")
}

func TestModule_Execute_ValidatesOwner(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	params := validParamsMap()
	delete(params, "owner")

	err := m.Execute(context.Background(), api, params)
	require.Error(t, err)
	assert.Contains(t, api.errs, "scaffold.validate")
	assert.Contains(t, err.Error(), "owner parameter is required")
}

func TestModule_Execute_ValidatesOrg(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	params := validParamsMap()
	delete(params, "org")

	err := m.Execute(context.Background(), api, params)
	require.Error(t, err)
	assert.Contains(t, api.errs, "scaffold.validate")
	assert.Contains(t, err.Error(), "org parameter is required")
}

func TestModule_Execute_ValidatesTemplate(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	params := validParamsMap()
	delete(params, "template")

	err := m.Execute(context.Background(), api, params)
	require.Error(t, err)
	assert.Contains(t, api.errs, "scaffold.validate")
	assert.Contains(t, err.Error(), "template parameter is required")
}

func TestModule_Execute_SuccessfulValidation(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	err := m.Execute(context.Background(), api, validParamsMap())
	require.NoError(t, err)

	// Verify validation step completed successfully
	assert.Contains(t, api.begins, "scaffold.validate")
	assert.Contains(t, api.oks, "scaffold.validate")
	assert.NotContains(t, api.errs, "scaffold.validate")
}

func TestModule_Execute_SetsCustomerIDOnLegacyModule(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	// Initially customerID should be empty
	secretCtx := m.SecretContext()
	assert.Equal(t, "", secretCtx.TenantID)

	err := m.Execute(context.Background(), api, validParamsMap())
	require.NoError(t, err)

	// After execution, customerID should be set from params
	secretCtx = m.SecretContext()
	assert.Equal(t, "customer-x", secretCtx.TenantID)
}

func TestNewModule_ValidatesCustomerID(t *testing.T) {
	_, err := NewModule("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "customerID is required")
}

func TestNewModule_Success(t *testing.T) {
	module, err := NewModule("customer-x")
	require.NoError(t, err)
	assert.NotNil(t, module)
	assert.Equal(t, "customer-x", module.customerID)
}

func TestRegister_RegistersScaffoldModule(t *testing.T) {
	reg := registry.NewModuleRegistry()
	require.NoError(t, Register(reg))

	mod, ok := reg.Lookup(ModuleKey, ModuleVersion)
	require.True(t, ok)
	assert.Equal(t, ModuleKey, mod.Key())
	assert.Equal(t, ModuleVersion, mod.Version())
}

// Test for cross-tenant isolation (conceptual test)
func TestCrossTenantIsolation_Conceptual(t *testing.T) {
	// This test documents the expected behavior rather than testing implementation
	// In a real integration test, we would:
	// 1. Create two modules with different tenant contexts
	// 2. Set up Volta with secrets for each tenant
	// 3. Verify module A can only access tenant A's secrets
	// 4. Verify module B can only access tenant B's secrets
	// 5. Verify module A cannot access tenant B's secrets

	moduleA, _ := NewModule("tenant-a")
	moduleB, _ := NewModule("tenant-b")

	assert.Equal(t, "tenant-a", moduleA.SecretContext().TenantID)
	assert.Equal(t, "tenant-b", moduleB.SecretContext().TenantID)

	// The actual isolation is enforced by:
	// 1. Volta's vault.GetVault(tenantID) which returns tenant-specific vault
	// 2. Physical key construction: "tenants/{tenantID}/{logicalKey}"
	// 3. Module-owned tenant resolution via SecretContextProvider interface
}

// Test error paths (conceptual test)
func TestErrorPaths_Conceptual(t *testing.T) {
	// This test documents expected error scenarios:
	// 1. Missing secrets - connector requires secrets but Volta doesn't have them
	//    Expected: SECRET_EXECUTION_FAILED error
	// 2. Invalid tenant - module returns empty tenant ID but connector requires secrets
	//    Expected: TENANT_CONTEXT_MISSING error (if module doesn't implement SecretContextProvider)
	//    Expected: VAULT_UNAVAILABLE error (if tenant ID is empty/invalid)
	// 3. Volta failures - Volta service unavailable
	//    Expected: VAULT_UNAVAILABLE or VOLTA_NOT_CONFIGURED errors
	// 4. Module missing - stepAPIAdapter.module is nil but connector requires secrets
	//    Expected: MODULE_MISSING error

	// These error paths are tested in the runner's step_api.go tests
	// and integration tests would verify end-to-end behavior
}
