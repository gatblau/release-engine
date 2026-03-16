package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseFragment_ValidateRequiresCoreFields(t *testing.T) {
	f := &DatabaseFragment{}
	p := &template.ProvisionParams{
		Database: template.DatabaseParams{
			Enabled: true,
		},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.engine required")
}

func TestDatabaseFragment_RenderHAWithDR(t *testing.T) {
	f := &DatabaseFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion:      "eu-west-1",
		SecondaryRegion:    "eu-central-1",
		Availability:       "critical",
		WorkloadProfile:    "large",
		DataClassification: "confidential",
		DRRequired:         true,
		Environment:        "production",
		Database: template.DatabaseParams{
			Enabled:       true,
			Engine:        "aurora-postgres",
			Tier:          "highly-available",
			StorageGiB:    200,
			StorageType:   "ssd",
			BackupEnabled: true,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	db := out["database"].(map[string]any)
	assert.Equal(t, true, db["enabled"])
	assert.Equal(t, true, db["multiAZ"])
	assert.Contains(t, db, "backup")
	assert.Contains(t, db, "disasterRecovery")
}
