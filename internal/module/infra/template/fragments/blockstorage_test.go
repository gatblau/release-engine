package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockStorageFragment_ValidateProvisionedRequiresIOPS(t *testing.T) {
	f := &BlockStorageFragment{}
	p := &template.ProvisionParams{
		BlockStore: template.BlockStoreParams{
			Enabled: true,
			Volumes: []template.BlockVolume{{
				Name:    "data",
				SizeGiB: 100,
				Type:    "provisioned",
			}},
		},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iops required")
}

func TestBlockStorageFragment_RenderProvisioned(t *testing.T) {
	f := &BlockStorageFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion: "eu-west-1",
		BlockStore: template.BlockStoreParams{
			Enabled: true,
			Volumes: []template.BlockVolume{{
				Name:       "data",
				SizeGiB:    100,
				Type:       "provisioned",
				IOPS:       3000,
				Throughput: 250,
			}},
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	bs := out["blockStorage"].(map[string]any)
	vols := bs["volumes"].([]map[string]any)
	assert.Equal(t, 3000, vols[0]["iops"])
	assert.Equal(t, 250, vols[0]["throughput"])
}
