package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMFragment_ValidateRequiresCoreFields(t *testing.T) {
	f := &VMFragment{}
	p := &template.ProvisionParams{
		VM: template.VMParams{
			Enabled: true,
			Count:   1,
		},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vm.instance_family required")
}

func TestVMFragment_RenderSpotAndAutoscaling(t *testing.T) {
	f := &VMFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion: "eu-west-1",
		VM: template.VMParams{
			Enabled:        true,
			Count:          1,
			InstanceFamily: "general",
			Size:           "medium",
			OSFamily:       "linux",
			Arch:           "amd64",
			SpotEnabled:    true,
			SpotMaxPrice:   0.4,
			AutoScaling: template.VMAutoScale{
				Enabled:  true,
				MinCount: 2,
				MaxCount: 4,
			},
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)

	vm := out["vm"].(map[string]any)
	assert.Equal(t, 2, vm["count"])
	assert.Contains(t, vm, "spot")
	assert.Contains(t, vm, "autoScaling")
	assert.Contains(t, vm, "hardening")
}
