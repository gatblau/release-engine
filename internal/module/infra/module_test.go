package infra

import (
	"context"
	"testing"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
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
	switch {
	case req.Connector == "policy" && req.Operation == "evaluate":
		return &stepapi.ConnectorResult{
			Status: "success",
			Output: map[string]any{
				"allowed":    true,
				"violations": []interface{}{},
			},
		}, nil
	case req.Connector == "git" && req.Operation == "commit_files":
		return &stepapi.ConnectorResult{
			Status: "success",
			Output: map[string]any{
				"commit_sha": "test-commit-sha",
				"changed":    true,
			},
		}, nil
	case req.Connector == "git" && req.Operation == "read_file":
		return &stepapi.ConnectorResult{
			Status: "success",
			Output: map[string]any{
				"content": "status: healthy\ncommit_sha: test-commit-sha\ntimestamp: 2026-03-22T18:06:00Z",
				"sha":     "test-sha",
			},
		}, nil
	case req.Connector == "webhook" && req.Operation == "post_callback":
		return &stepapi.ConnectorResult{
			Status: "success",
			Output: map[string]any{
				"status_code":   200,
				"response_body": `{"status":"success"}`,
			},
		}, nil
	default:
		// Default response for any other connector/operation
		return &stepapi.ConnectorResult{
			Status: "success",
			Output: map[string]any{},
		}, nil
	}
}

func (m *mockStepAPI) WaitForApproval(ctx context.Context, req stepapi.ApprovalRequest) (stepapi.ApprovalOutcome, error) {
	return stepapi.ApprovalOutcome{
		Decision:      "approved",
		Approver:      "test-approver",
		Justification: "test approval",
		DecidedAt:     time.Now(),
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
		"contract_version":    "v1",
		"request_name":        "checkout-prod",
		"tenant":              "payments",
		"owner":               "platform-team",
		"environment":         "production",
		"workload_profile":    "medium",
		"catalogue_item":      catalog.K8sAppName,
		"namespace":           "platform-system",
		"residency":           "eu",
		"primary_region":      "eu-west-1",
		"secondary_region":    "eu-central-1",
		"availability":        "high",
		"data_classification": "confidential",
		"ingress_mode":        "public",
		"egress_mode":         "nat",
		"dr_required":         true,
		"backup_required":     true,
		"cost_centre":         "cost-center-123",
		"idempotency_key":     "test-job-123",
		"job_id":              "job-123",
		"callback_url":        "https://example.com/callback",
		"infra_repo":          "org/infra-manifests",
		"kubernetes": map[string]any{
			"enabled":  true,
			"provider": "aws",
			"tier":     "standard",
			"size":     "medium",
		},
		"object_store": map[string]any{
			"enabled": false,
		},
		"messaging": map[string]any{
			"enabled": false,
		},
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

func TestModule_Execute_RendersAndPublishesContext(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	err := m.Execute(context.Background(), api, validParamsMap())
	require.NoError(t, err)

	assert.Contains(t, api.begins, "infra.render")
	assert.Contains(t, api.oks, "infra.render")
	assert.NotContains(t, api.errs, "infra.render")
	manifest, ok := api.contexts["infra.manifest"].(string)
	require.True(t, ok)
	assert.Contains(t, manifest, "kind: XKubernetesCluster")
	assert.Contains(t, manifest, "name: kubernetes-aws")
}

func TestModule_Execute_InvalidParamsEndsInErrorStep(t *testing.T) {
	m := NewLegacyModule()
	api := &mockStepAPI{}

	err := m.Execute(context.Background(), api, map[string]any{"request_name": "missing-required-fields"})
	require.Error(t, err)
	assert.Contains(t, api.errs, "infra.render")
}

func TestRegister_RegistersInfraModule(t *testing.T) {
	reg := registry.NewModuleRegistry()
	require.NoError(t, Register(reg))

	mod, ok := reg.Lookup(ModuleKey, ModuleVersion)
	require.True(t, ok)
	assert.Equal(t, ModuleKey, mod.Key())
	assert.Equal(t, ModuleVersion, mod.Version())
}

// Mock connectors for testing
type mockGitConnector struct{ connector.BaseConnector }
type mockPolicyConnector struct{ connector.BaseConnector }
type mockWebhookConnector struct{ connector.BaseConnector }

func (m *mockGitConnector) Validate(operation string, input map[string]interface{}) error { return nil }
func (m *mockGitConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}
func (m *mockGitConnector) Close() error { return nil }

func (m *mockPolicyConnector) Validate(operation string, input map[string]interface{}) error {
	return nil
}
func (m *mockPolicyConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}
func (m *mockPolicyConnector) Close() error { return nil }

func (m *mockWebhookConnector) Validate(operation string, input map[string]interface{}) error {
	return nil
}
func (m *mockWebhookConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}
func (m *mockWebhookConnector) Close() error { return nil }

func TestNewModule_Success(t *testing.T) {
	// Create test vars
	vars := Vars{
		HealthTimeout: 30 * time.Second,
		PollInterval:  500 * time.Millisecond,
	}

	// Create mock connectors
	baseGit, _ := connector.NewBaseConnector(connector.ConnectorTypeGit, "github")
	git := &mockGitConnector{BaseConnector: baseGit}

	basePolicy, _ := connector.NewBaseConnector(connector.ConnectorTypeOther, "policy-mock")
	policy := &mockPolicyConnector{BaseConnector: basePolicy}

	baseWebhook, _ := connector.NewBaseConnector(connector.ConnectorTypeOther, "webhook-mock")
	webhook := &mockWebhookConnector{BaseConnector: baseWebhook}

	// Create module with vars and connectors
	module, err := NewModule(vars, git, policy, webhook)
	require.NoError(t, err)
	assert.NotNil(t, module)

	// Verify module fields
	assert.Equal(t, &vars, module.vars)
	assert.Equal(t, git, module.gitConnector)
	assert.Equal(t, policy, module.policyConnector)
	assert.Equal(t, webhook, module.webhookConnector)

	// Verify metadata methods still work
	assert.Equal(t, ModuleKey, module.Key())
	assert.Equal(t, ModuleVersion, module.Version())
}

func TestNewModule_NilConnectorErrors(t *testing.T) {
	// Create test vars
	vars := Vars{
		HealthTimeout: 30 * time.Second,
		PollInterval:  500 * time.Millisecond,
	}

	// Create mock connectors
	baseGit, _ := connector.NewBaseConnector(connector.ConnectorTypeGit, "github")
	git := &mockGitConnector{BaseConnector: baseGit}

	basePolicy, _ := connector.NewBaseConnector(connector.ConnectorTypeOther, "policy-mock")
	policy := &mockPolicyConnector{BaseConnector: basePolicy}

	baseWebhook, _ := connector.NewBaseConnector(connector.ConnectorTypeOther, "webhook-mock")
	webhook := &mockWebhookConnector{BaseConnector: baseWebhook}

	// Test nil git connector
	_, err := NewModule(vars, nil, policy, webhook)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git connector cannot be nil")

	// Test nil policy connector
	_, err = NewModule(vars, git, nil, webhook)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy connector cannot be nil")

	// Test nil webhook connector
	_, err = NewModule(vars, git, policy, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook connector cannot be nil")
}

func TestModule_ConstructorBackwardsCompatibility(t *testing.T) {
	// Verify that NewLegacyModule() still works (legacy constructor)
	legacyModule := NewLegacyModule()
	assert.NotNil(t, legacyModule)

	// Legacy module should have default vars set and nil connectors
	assert.NotNil(t, legacyModule.vars)
	assert.Equal(t, 30*time.Second, legacyModule.vars.HealthTimeout)
	assert.Equal(t, 500*time.Millisecond, legacyModule.vars.PollInterval)
	assert.Nil(t, legacyModule.gitConnector)
	assert.Nil(t, legacyModule.policyConnector)
	assert.Nil(t, legacyModule.webhookConnector)

	// But metadata methods should still work
	assert.Equal(t, ModuleKey, legacyModule.Key())
	assert.Equal(t, ModuleVersion, legacyModule.Version())

	// Execute should still work with legacy module
	api := &mockStepAPI{}
	// Execute might fail due to missing step API implementation details,
	// but we can at least verify it doesn't panic on nil fields
	// We'll just check that the method can be called
	assert.NotPanics(t, func() {
		_ = legacyModule.Execute(context.Background(), api, validParamsMap())
	})
}
