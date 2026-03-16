package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComplianceFragment_AlwaysApplicable(t *testing.T) {
	f := &ComplianceFragment{}
	assert.True(t, f.Applicable(&template.ProvisionParams{}))
}

func TestComplianceFragment_Render_CriticalAndEU(t *testing.T) {
	f := &ComplianceFragment{}
	p := &template.ProvisionParams{
		Residency:          "eu",
		DataClassification: "restricted",
		Availability:       "critical",
		DRRequired:         true,
		BackupRequired:     true,
		Compliance:         []string{"pci-dss"},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	c := out["compliance"].(map[string]any)
	assert.Equal(t, true, c["gdprCompliant"])
	assert.Equal(t, true, c["backupVerification"])
	assert.Equal(t, true, c["encryptionAtRest"])
	assert.Equal(t, []string{"pci-dss"}, c["frameworks"])
}
