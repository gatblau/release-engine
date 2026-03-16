package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancerFragment_ValidateRequiresType(t *testing.T) {
	f := &LoadBalancerFragment{}
	p := &template.ProvisionParams{
		IngressMode:  "public",
		LoadBalancer: template.LoadBalancerParams{Enabled: true, Scheme: "internet-facing"},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load_balancer.type required")
}

func TestLoadBalancerFragment_RenderHTTPSWAF(t *testing.T) {
	f := &LoadBalancerFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion: "eu-west-1",
		IngressMode:   "public",
		Compliance:    []string{"pci-dss"},
		LoadBalancer: template.LoadBalancerParams{
			Enabled: true,
			Type:    "application",
			Scheme:  "internet-facing",
			HTTPS:   true,
			WAF:     true,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)
	lb := out["loadBalancer"].(map[string]any)
	assert.Contains(t, lb, "listeners")
	assert.Contains(t, lb, "waf")
	assert.Contains(t, lb, "accessLogs")
}
