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
		Tenant:        "tenant-a",
		Owner:         "owner-a",
		Environment:   "staging",
		CatalogueItem: "web",
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

func TestTagsFragment_Render_FinOpsTags(t *testing.T) {
	f := &TagsFragment{}
	p := &template.ProvisionParams{
		ContractVersion:    "v1",
		RequestName:        "order-service-prod",
		Tenant:             "ecommerce",
		Owner:              "checkout-team",
		Environment:        "production",
		CatalogueItem:      "k8s-app",
		CostCentre:         "CC-4521",
		BusinessUnit:       "retail",
		Project:            "checkout-replatform",
		DataClassification: "confidential",
		TTL:                "2025-12-31",
		ExtraTags: map[string]string{
			"application": "order-service",
			"team":        "checkout-team",
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	tags := out["tags"].(map[string]string)

	// Mandatory FinOps tags
	assert.Equal(t, "CC-4521", tags["cost-centre"])
	assert.Equal(t, "order-service-prod", tags["service"])
	assert.Equal(t, "production", tags["environment"])
	assert.Equal(t, "checkout-team", tags["owner"])
	assert.Equal(t, "release-engine", tags["managed-by"])

	// Optional FinOps tags
	assert.Equal(t, "retail", tags["business-unit"])
	assert.Equal(t, "checkout-replatform", tags["project"])
	assert.Equal(t, "confidential", tags["data-classification"])
	assert.Equal(t, "2025-12-31", tags["ttl"])

	// Backward compatibility tags
	assert.Equal(t, "ecommerce", tags["tenant"])
	assert.Equal(t, "k8s-app", tags["catalogue-item"])

	// Extra tags
	assert.Equal(t, "order-service", tags["application"])
	assert.Equal(t, "checkout-team", tags["team"])
}

func TestTagsFragment_Render_OptionalFieldsEmpty(t *testing.T) {
	f := &TagsFragment{}
	p := &template.ProvisionParams{
		RequestName:   "test-service",
		Tenant:        "test-tenant",
		Owner:         "test-owner",
		Environment:   "development",
		CatalogueItem: "test-item",
		CostCentre:    "CC-1234",
		// BusinessUnit, Project, DataClassification, TTL intentionally left empty
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	tags := out["tags"].(map[string]string)

	// Mandatory tags should be present
	assert.Equal(t, "CC-1234", tags["cost-centre"])
	assert.Equal(t, "test-service", tags["service"])
	assert.Equal(t, "development", tags["environment"])
	assert.Equal(t, "test-owner", tags["owner"])
	assert.Equal(t, "release-engine", tags["managed-by"])

	// Optional tags should NOT be present when empty
	assert.NotContains(t, tags, "business-unit")
	assert.NotContains(t, tags, "project")
	assert.NotContains(t, tags, "data-classification")
	assert.NotContains(t, tags, "ttl")
}

func TestTagsFragment_Render_ProductionScenario(t *testing.T) {
	f := &TagsFragment{}
	p := &template.ProvisionParams{
		ContractVersion:    "v1",
		RequestName:        "order-service-prod",
		Tenant:             "ecommerce",
		Owner:              "checkout-team",
		Environment:        "production",
		CatalogueItem:      "k8s-app",
		Namespace:          "platform-system",
		PrimaryRegion:      "eu-west-1",
		Availability:       "high",
		DataClassification: "confidential",
		IngressMode:        "private",
		EgressMode:         "nat",
		CostCentre:         "CC-4521",
		BusinessUnit:       "retail",
		Project:            "checkout-replatform",
		TTL:                "", // Production - no TTL
		ExtraTags: map[string]string{
			"application": "order-service",
			"team":        "checkout-team",
		},
		Kubernetes: template.KubernetesParams{
			Enabled: true,
			Tier:    "standard",
			Size:    "medium",
			MultiAZ: true,
			Version: "1.30",
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	tags := out["tags"].(map[string]string)

	// Mandatory FinOps tags
	assert.Equal(t, "CC-4521", tags["cost-centre"])
	assert.Equal(t, "order-service-prod", tags["service"])
	assert.Equal(t, "production", tags["environment"])
	assert.Equal(t, "checkout-team", tags["owner"])
	assert.Equal(t, "release-engine", tags["managed-by"])

	// Optional FinOps tags (business-unit and project should be present, ttl should not)
	assert.Equal(t, "retail", tags["business-unit"])
	assert.Equal(t, "checkout-replatform", tags["project"])
	assert.Equal(t, "confidential", tags["data-classification"])
	assert.NotContains(t, tags, "ttl") // Empty TTL should not be included

	// Backward compatibility tags
	assert.Equal(t, "ecommerce", tags["tenant"])
	assert.Equal(t, "k8s-app", tags["catalogue-item"])

	// Extra tags
	assert.Equal(t, "order-service", tags["application"])
	assert.Equal(t, "checkout-team", tags["team"])
}
