package template

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testParams() *ProvisionParams {
	return &ProvisionParams{
		ContractVersion:    "v1",
		RequestName:        "checkout-prod",
		Tenant:             "payments",
		Owner:              "platform-team",
		Environment:        "production",
		TemplateName:       catalog.K8sAppName,
		CompositionRef:     "composition-web-v1",
		Namespace:          "platform-system",
		Residency:          "eu",
		PrimaryRegion:      "eu-west-1",
		SecondaryRegion:    "eu-central-1",
		Availability:       "high",
		DataClassification: "confidential",
		IngressMode:        "public",
		EgressMode:         "nat",
		DRRequired:         true,
		BackupRequired:     true,
		Kubernetes:         KubernetesParams{Enabled: true},
	}
}

func TestValidate_OK(t *testing.T) {
	err := Validate(testParams())
	require.NoError(t, err)
}

func TestValidate_MissingRequired(t *testing.T) {
	p := testParams()
	p.RequestName = ""
	p.PrimaryRegion = ""

	err := Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request_name is required")
	assert.Contains(t, err.Error(), "primary_region is required")
}

func TestValidate_CriticalRequiresDRAndBackup(t *testing.T) {
	p := testParams()
	p.Availability = "critical"
	p.DRRequired = false
	p.BackupRequired = false

	err := Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dr_required must be true when availability is critical")
	assert.Contains(t, err.Error(), "backup_required must be true when availability is critical")
}
