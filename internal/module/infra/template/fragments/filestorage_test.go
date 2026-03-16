package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorageFragment_ValidateProvisionedNeedsThroughput(t *testing.T) {
	f := &FileStorageFragment{}
	p := &template.ProvisionParams{
		FileStore: template.FileStoreParams{
			Enabled:        true,
			ThroughputMode: "provisioned",
		},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "throughput_mibs required")
}

func TestFileStorageFragment_RenderWithBackupAndReplication(t *testing.T) {
	f := &FileStorageFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion:   "eu-west-1",
		SecondaryRegion: "eu-central-1",
		BackupRequired:  true,
		DRRequired:      true,
		FileStore: template.FileStoreParams{
			Enabled:         true,
			PerformanceMode: "max-io",
			ThroughputMode:  "provisioned",
			ThroughputMiBs:  256,
			Protocol:        "nfs",
			MultiAZ:         true,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	fs := out["fileStorage"].(map[string]any)
	assert.Contains(t, fs, "backup")
	assert.Contains(t, fs, "replication")
	assert.Equal(t, 256, fs["provisionedThroughputMiBs"])
}
