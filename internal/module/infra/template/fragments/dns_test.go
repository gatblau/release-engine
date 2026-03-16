package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDNSFragment_ValidateRequiresZone(t *testing.T) {
	f := &DNSFragment{}
	p := &template.ProvisionParams{DNS: template.DNSParams{Enabled: true}}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dns.zone_name required")
}

func TestDNSFragment_RenderPrivateZone(t *testing.T) {
	f := &DNSFragment{}
	p := &template.ProvisionParams{
		DNS: template.DNSParams{
			Enabled:  true,
			ZoneName: "example.internal",
			Private:  true,
			Records: []template.DNSRec{{
				Name:   "api",
				Type:   "A",
				Values: []string{"10.0.0.10"},
			}},
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)
	dns := out["dns"].(map[string]any)
	assert.Equal(t, true, dns["vpcAssociation"])
	assert.Contains(t, dns, "records")
}
