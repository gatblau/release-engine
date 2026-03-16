package catalog_test

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAll_LoadsEmbeddedDefinitions(t *testing.T) {
	cats, err := catalog.LoadAll()
	require.NoError(t, err)

	require.Contains(t, cats, catalog.K8sAppName)
	require.Contains(t, cats, catalog.VMAppName)
	require.Contains(t, cats, catalog.DataProcName)

	assert.NotEmpty(t, cats[catalog.K8sAppName].CompositionRef)
}

func TestCatalogNameConstants_MatchEmbeddedDefinitions(t *testing.T) {
	cats, err := catalog.LoadAll()
	require.NoError(t, err)

	want := []string{catalog.K8sAppName, catalog.VMAppName, catalog.DataProcName}
	for _, name := range want {
		assert.Contains(t, cats, name, "catalog constant %q must match an embedded definition name", name)
	}
	assert.Len(t, cats, len(want), "update constants when adding/removing catalog definitions")
}

func TestTemplateCatalog_ValidateParams(t *testing.T) {
	cat := &catalog.TemplateCatalog{
		Name: "test",
		Constraints: catalog.CatalogConstraints{
			AllowedEnvironments:     []string{"development", "production"},
			AllowedWorkloadProfiles: []string{"small", "medium"},
			AllowedAvailabilities:   []string{"standard", "high"},
			AllowedResidencies:      []string{"eu", "us"},
		},
	}

	assert.NoError(t, cat.ValidateParams("development", "small", "standard", "eu"))

	err := cat.ValidateParams("staging", "small", "standard", "eu")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
}
