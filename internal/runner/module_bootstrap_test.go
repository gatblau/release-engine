package runner

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultModuleRegistry_RegistersInfraModule(t *testing.T) {
	reg, err := NewDefaultModuleRegistry()
	require.NoError(t, err)

	mod, ok := reg.Lookup(infra.ModuleKey, infra.ModuleVersion)
	require.True(t, ok)
	require.Equal(t, infra.ModuleKey, mod.Key())
	require.Equal(t, infra.ModuleVersion, mod.Version())
}
