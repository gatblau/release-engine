package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVPCFragment_ValidateRequiresCIDR(t *testing.T) {
	f := &VPCFragment{}
	p := &template.ProvisionParams{
		VPC: template.VPCParams{Enabled: true, PrivateSubnets: 2},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vpc.cidr required")
}

func TestVPCFragment_RenderWithPeering(t *testing.T) {
	f := &VPCFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion: "eu-west-1",
		Availability:  "critical",
		VPC: template.VPCParams{
			Enabled:        true,
			CIDR:           "10.0.0.0/16",
			PrivateSubnets: 3,
			PublicSubnets:  2,
			NATGateways:    1,
			PeeringRequests: []template.VPCPeer{{
				PeerVPCID:   "vpc-123",
				PeerAccount: "1234567890",
				PeerRegion:  "eu-central-1",
			}},
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)
	vpc := out["vpc"].(map[string]any)
	assert.Equal(t, true, vpc["flowLogs"])
	assert.Contains(t, vpc, "peering")
}
