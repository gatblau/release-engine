package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretsFragment_ValidateRequiresProvider(t *testing.T) {
	f := &SecretsFragment{}
	p := &template.ProvisionParams{Secrets: template.SecretsParams{Enabled: true}}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secrets.provider required")
}

func TestSecretsFragment_RenderCustomerManagedKMS(t *testing.T) {
	f := &SecretsFragment{}
	p := &template.ProvisionParams{
		RequestName:     "checkout",
		PrimaryRegion:   "eu-west-1",
		SecondaryRegion: "eu-central-1",
		DRRequired:      true,
		Secrets: template.SecretsParams{
			Enabled:              true,
			Provider:             "secrets-manager",
			KMSKeyType:           "customer-managed",
			AutoRotation:         true,
			RotationIntervalDays: 30,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)
	s := out["secrets"].(map[string]any)
	assert.Contains(t, s, "kmsKey")
	assert.Contains(t, s, "autoRotation")
	assert.Contains(t, s, "replication")
}
