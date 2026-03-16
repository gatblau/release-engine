package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectStorageFragment_ValidateRestrictedNeedsVersioning(t *testing.T) {
	f := &ObjectStorageFragment{}
	p := &template.ProvisionParams{
		DataClassification: "restricted",
		ObjectStore: template.ObjectStoreParams{
			Enabled:    true,
			Class:      "standard",
			Versioning: false,
		},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "object_store.versioning must be true")
}

func TestObjectStorageFragment_RenderWithDRAndGDPR(t *testing.T) {
	f := &ObjectStorageFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion:   "eu-west-1",
		SecondaryRegion: "eu-central-1",
		DRRequired:      true,
		Compliance:      []string{"gdpr"},
		ObjectStore: template.ObjectStoreParams{
			Enabled:       true,
			Class:         "archive",
			Versioning:    true,
			RetentionDays: 30,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	os := out["objectStorage"].(map[string]any)
	assert.Contains(t, os, "replication")
	assert.Contains(t, os, "objectLock")
	assert.Contains(t, os, "lifecycleRules")
}
