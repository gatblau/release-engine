package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityFragment_ValidateRequiresType(t *testing.T) {
	f := &IdentityFragment{}
	p := &template.ProvisionParams{Identity: template.IdentityParams{Enabled: true}}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity.type required")
}

func TestIdentityFragment_RenderFederated(t *testing.T) {
	f := &IdentityFragment{}
	p := &template.ProvisionParams{
		Tenant:      "payments",
		Environment: "production",
		RequestName: "checkout",
		Identity: template.IdentityParams{
			Enabled:             true,
			Type:                "federated",
			ServiceAccountCount: 2,
			FederationProvider:  "oidc",
			FederationAudience:  "sts",
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)
	id := out["identity"].(map[string]any)
	assert.Contains(t, id, "federation")
	assert.Contains(t, id, "permissionBoundary")
}
