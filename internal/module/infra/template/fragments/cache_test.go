package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheFragment_ValidateRules(t *testing.T) {
	f := &CacheFragment{}
	p := &template.ProvisionParams{
		Availability: "critical",
		Cache: template.CacheParams{
			Enabled:      true,
			Engine:       "memcached",
			Tier:         "clustered",
			ReplicaCount: 1,
		},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "memcached does not support clustered tier")
}

func TestCacheFragment_RenderClustered(t *testing.T) {
	f := &CacheFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion:   "eu-west-1",
		WorkloadProfile: "large",
		Environment:     "production",
		Cache: template.CacheParams{
			Enabled:      true,
			Engine:       "redis",
			Tier:         "clustered",
			ReplicaCount: 2,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	c := out["cache"].(map[string]any)
	assert.Equal(t, true, c["enabled"])
	assert.Contains(t, c, "clusterMode")
	assert.Contains(t, c, "maintenanceWindow")
}
