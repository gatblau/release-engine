package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservabilityFragment_Applicable(t *testing.T) {
	f := &ObservabilityFragment{}
	assert.False(t, f.Applicable(&template.ProvisionParams{}))
}

func TestObservabilityFragment_RenderEnhanced(t *testing.T) {
	f := &ObservabilityFragment{}
	p := &template.ProvisionParams{
		Tenant:             "payments",
		Environment:        "production",
		WorkloadType:       "web",
		Availability:       "critical",
		DataClassification: "confidential",
		Observability: template.ObservabilityParams{
			Enabled:          true,
			TracingEnabled:   true,
			DashboardEnabled: true,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)
	o := out["observability"].(map[string]any)
	assert.Contains(t, o, "metrics")
	assert.Contains(t, o, "logging")
	assert.Contains(t, o, "tracing")
	assert.Contains(t, o, "dashboards")
	assert.Contains(t, o, "alarms")
}
