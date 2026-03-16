package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCDNFragment_ValidateDisallowsPrivateExposure(t *testing.T) {
	f := &CDNFragment{}
	p := &template.ProvisionParams{
		WorkloadExposure: "private",
		CDN:              template.CDNParams{Enabled: true, OriginType: "s3"},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cdn cannot be enabled for private workload exposure")
}

func TestCDNFragment_RenderWithWAF(t *testing.T) {
	f := &CDNFragment{}
	p := &template.ProvisionParams{
		Compliance: []string{"pci-dss"},
		CDN: template.CDNParams{
			Enabled:    true,
			OriginType: "lb",
			WAF:        true,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)
	cdn := out["cdn"].(map[string]any)
	assert.Contains(t, cdn, "waf")
	assert.Contains(t, cdn, "logging")
}
