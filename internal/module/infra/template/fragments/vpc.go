// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"
	"net"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type VPCFragment struct{}

func (f *VPCFragment) Name() string { return "vpc" }

func (f *VPCFragment) Applicable(params *template.ProvisionParams) bool {
	return params.VPC.Enabled
}

func (f *VPCFragment) Validate(params *template.ProvisionParams) error {
	v := params.VPC
	if !v.Enabled {
		return nil
	}
	if v.CIDR == "" {
		return fmt.Errorf("vpc.cidr required when vpc.enabled is true")
	}
	if _, _, err := net.ParseCIDR(v.CIDR); err != nil {
		return fmt.Errorf("vpc.cidr must be valid: %w", err)
	}
	if v.PrivateSubnets == 0 {
		return fmt.Errorf("vpc.private_subnets must be >= 1")
	}
	if v.PublicSubnets > 0 && v.NATGateways == 0 {
		return fmt.Errorf("vpc.nat_gateways must be >= 1 when public subnets exist")
	}
	return nil
}

func (f *VPCFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	v := params.VPC
	azCount := resolve.CoalesceInt(v.PrivateSubnets, 3)

	spec := map[string]any{
		"enabled":           true,
		"cidr":              v.CIDR,
		"region":            params.PrimaryRegion,
		"availabilityZones": resolve.RegionAZs(params.PrimaryRegion, azCount),
		"privateSubnets":    v.PrivateSubnets,
		"publicSubnets":     v.PublicSubnets,
		"natGateways":       v.NATGateways,
		"flowLogs":          v.FlowLogs || params.Availability == "critical",
	}

	if v.TransitGateway {
		spec["transitGateway"] = map[string]any{"enabled": true}
	}
	if len(v.PeeringRequests) > 0 {
		peers := make([]map[string]any, len(v.PeeringRequests))
		for i, p := range v.PeeringRequests {
			peers[i] = map[string]any{
				"peerVpcId":   p.PeerVPCID,
				"peerAccount": p.PeerAccount,
				"peerRegion":  p.PeerRegion,
			}
		}
		spec["peering"] = peers
	}

	return map[string]any{"vpc": spec}, nil
}
