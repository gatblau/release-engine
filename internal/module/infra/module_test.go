package infra

import (
	"context"
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func validParamsMap() map[string]any {
	return map[string]any{
		"contract_version":    "v1",
		"request_name":        "checkout-prod",
		"tenant":              "payments",
		"owner":               "platform-team",
		"environment":         "production",
		"workload_profile":    "medium",
		"template_name":       catalog.K8sAppName,
		"composition_ref":     "composition-web-v1",
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
		"kubernetes": map[string]any{
			"enabled": true,
			"tier":    "standard",
			"size":    "medium",
		},
	}
}

func TestModule_ImplementsRegistryContract(t *testing.T) {
	var _ registry.Module = NewModule()
}

func TestModule_Metadata(t *testing.T) {
	m := NewModule()
	assert.Equal(t, ModuleKey, m.Key())
	assert.Equal(t, ModuleVersion, m.Version())
}

func TestModule_Execute_RendersAndPublishesContext(t *testing.T) {
	m := NewModule()
	api := &mockStepAPI{}

	err := m.Execute(context.Background(), api, validParamsMap())
	require.NoError(t, err)

	assert.Contains(t, api.begins, "infra.render")
	assert.Contains(t, api.oks, "infra.render")
	assert.NotContains(t, api.errs, "infra.render")
	manifest, ok := api.contexts["infra.manifest"].(string)
	require.True(t, ok)
	assert.Contains(t, manifest, "InfrastructureRequest")
}

func TestModule_Execute_InvalidParamsEndsInErrorStep(t *testing.T) {
	m := NewModule()
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
