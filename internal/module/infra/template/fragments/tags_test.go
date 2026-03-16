package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagsFragment_AlwaysApplicable(t *testing.T) {
	f := &TagsFragment{}
	assert.True(t, f.Applicable(&template.ProvisionParams{}))
}

func TestTagsFragment_Render_MergesExtraTags(t *testing.T) {
	f := &TagsFragment{}
	p := &template.ProvisionParams{
		Tenant:       "tenant-a",
		Owner:        "owner-a",
		Environment:  "staging",
		TemplateName: "web",
		ExtraTags: map[string]string{
			"cost-center": "cc-01",
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	tags := out["tags"].(map[string]string)
	assert.Equal(t, "tenant-a", tags["tenant"])
	assert.Equal(t, "owner-a", tags["owner"])
	assert.Equal(t, "cc-01", tags["cost-center"])
	assert.Equal(t, "release-engine", tags["managed-by"])
}
