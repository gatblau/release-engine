// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package template_test

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/require"
)

func testProvisionParams() *template.ProvisionParams {
	return &template.ProvisionParams{
		ContractVersion: "v1",
		RequestName:     "order-service-prod",
		Tenant:          "ecommerce",
		Owner:           "checkout-team",
		Environment:     "production",
		CatalogueItem:   "k8s-app",
		Namespace:       "platform-system",
		PrimaryRegion:   "eu-west-1",
		Kubernetes: template.KubernetesParams{
			Enabled:  true,
			Provider: "aws",
		},
	}
}

func TestBuildXRs_PassesFinOpsTagsThrough(t *testing.T) {
	params := testProvisionParams()
	specParts := map[string]any{
		"tags": map[string]string{
			"cost-centre":         "CC-4521",
			"service":             "order-service-prod",
			"environment":         "production",
			"owner":               "checkout-team",
			"managed-by":          "release-engine",
			"business-unit":       "retail",
			"project":             "checkout-replatform",
			"data-classification": "confidential",
			"ttl":                 "2025-12-31",
			"extra-fallback":      "keep-me",
		},
		"kubernetes": map[string]any{
			"version":  "1.24",
			"nodePool": map[string]any{},
		},
	}

	docs, err := template.BuildXRs(params, specParts)
	require.NoError(t, err)
	require.Len(t, docs, 1)

	spec := docs[0]["spec"].(map[string]any)
	parameters := spec["parameters"].(map[string]any)
	tags := parameters["tags"].(map[string]string)

	require.Equal(t, "CC-4521", tags["cost-centre"])
	require.Equal(t, "order-service-prod", tags["service"])
	require.Equal(t, "production", tags["environment"])
	require.Equal(t, "checkout-team", tags["owner"])
	require.Equal(t, "release-engine", tags["managed-by"])
	require.Equal(t, "retail", tags["business-unit"])
	require.Equal(t, "checkout-replatform", tags["project"])
	require.Equal(t, "confidential", tags["data-classification"])
	require.Equal(t, "2025-12-31", tags["ttl"])
	require.Equal(t, "keep-me", tags["extra-fallback"])
}

func TestBuildXRs_ExtractsTagsFromNestedStructure(t *testing.T) {
	params := testProvisionParams()
	specParts := map[string]any{
		"tags": map[string]any{
			"tags": map[string]string{
				"cost-centre": "CC-999",
				"service":     "nested-service",
				"environment": "staging",
				"owner":       "nested-team",
				"managed-by":  "release-engine",
			},
			"catalogue-item": "should-be-ignored",
		},
		"kubernetes": map[string]any{
			"version":  "1.24",
			"nodePool": map[string]any{},
		},
	}

	docs, err := template.BuildXRs(params, specParts)
	require.NoError(t, err)
	require.Len(t, docs, 1)

	spec := docs[0]["spec"].(map[string]any)
	parameters := spec["parameters"].(map[string]any)
	tags := parameters["tags"].(map[string]string)

	require.Equal(t, "CC-999", tags["cost-centre"])
	require.Equal(t, "nested-service", tags["service"])
	require.Equal(t, "staging", tags["environment"])
	require.Equal(t, "nested-team", tags["owner"])
	require.Equal(t, "release-engine", tags["managed-by"])
	require.NotContains(t, tags, "catalogue-item")
}
