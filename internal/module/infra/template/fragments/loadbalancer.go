// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type LoadBalancerFragment struct{}

func (f *LoadBalancerFragment) Name() string { return "load-balancer" }

func (f *LoadBalancerFragment) Applicable(params *template.ProvisionParams) bool {
	return params.LoadBalancer.Enabled
}

func (f *LoadBalancerFragment) Validate(params *template.ProvisionParams) error {
	lb := params.LoadBalancer
	if !lb.Enabled {
		return nil
	}
	if lb.Type == "" {
		return fmt.Errorf("load_balancer.type required when load_balancer.enabled is true")
	}
	if lb.Scheme == "" {
		return fmt.Errorf("load_balancer.scheme required when load_balancer.enabled is true")
	}
	if lb.Scheme == "internet-facing" && (params.IngressMode == "disabled" || params.IngressMode == "internal") {
		return fmt.Errorf("internet-facing load balancer requires ingress_mode public")
	}
	return nil
}

func (f *LoadBalancerFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	lb := params.LoadBalancer

	spec := map[string]any{
		"enabled":   true,
		"type":      lb.Type,
		"scheme":    lb.Scheme,
		"region":    params.PrimaryRegion,
		"crossZone": true,
	}

	if lb.HTTPS {
		spec["listeners"] = []map[string]any{
			{
				"port":      443,
				"protocol":  "HTTPS",
				"sslPolicy": resolve.SSLPolicy(params.Compliance),
			},
			{
				"port":       80,
				"protocol":   "HTTP",
				"redirectTo": 443,
			},
		}
	}

	if lb.WAF {
		spec["waf"] = map[string]any{
			"enabled": true,
			"ruleSet": resolve.WAFRuleSet(params.Compliance),
		}
	}

	if lb.IdleTimeout > 0 {
		spec["idleTimeout"] = lb.IdleTimeout
	}

	hc := lb.HealthCheck
	if hc.Path != "" || hc.Port > 0 {
		spec["healthCheck"] = map[string]any{
			"protocol":           resolve.Coalesce(hc.Protocol, "HTTP"),
			"path":               resolve.Coalesce(hc.Path, "/health"),
			"port":               resolve.CoalesceInt(hc.Port, 80),
			"intervalSeconds":    resolve.CoalesceInt(hc.IntervalSeconds, 30),
			"healthyThreshold":   resolve.CoalesceInt(hc.HealthyThreshold, 3),
			"unhealthyThreshold": resolve.CoalesceInt(hc.UnhealthyThreshold, 3),
		}
	}

	spec["accessLogs"] = map[string]any{"enabled": true}

	return map[string]any{"loadBalancer": spec}, nil
}
