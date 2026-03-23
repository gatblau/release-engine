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
		CatalogueItem:      catalog.K8sAppName,
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
		CostCentre:         "cost-center-123",
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

func TestValidate_CostCentreRequired(t *testing.T) {
	p := testParams()
	p.CostCentre = ""

	err := Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cost_centre is required for FinOps cost allocation")
}

func TestValidate_TTLFormat(t *testing.T) {
	p := testParams()
	p.TTL = "invalid-date-format"

	err := Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ttl must be in ISO 8601 date format (YYYY-MM-DD)")
}

func TestValidate_TTLValidFormat(t *testing.T) {
	p := testParams()
	p.TTL = "2025-12-31"

	err := Validate(p)
	require.NoError(t, err)
}
