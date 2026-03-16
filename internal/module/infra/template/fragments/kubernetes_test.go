package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetesFragment_ValidateRequiresTierAndSize(t *testing.T) {
	f := &KubernetesFragment{}
	p := &template.ProvisionParams{
		Availability: "high",
		Kubernetes: template.KubernetesParams{
			Enabled: true,
		},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubernetes.tier required")
}

func TestKubernetesFragment_RenderAdvancedTier(t *testing.T) {
	f := &KubernetesFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion: "eu-west-1",
		Kubernetes: template.KubernetesParams{
			Enabled:       true,
			Tier:          "advanced",
			Size:          "medium",
			Version:       "1.31",
			MultiAZ:       true,
			NodePoolCount: 3,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	k := out["kubernetes"].(map[string]any)
	assert.Equal(t, true, k["enabled"])
	assert.Equal(t, "1.31", k["version"])
	assert.Equal(t, 2, k["additionalNodePools"])
	assert.Contains(t, k, "addons")
}
