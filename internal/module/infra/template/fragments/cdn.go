// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type CDNFragment struct{}

func (f *CDNFragment) Name() string { return "cdn" }

func (f *CDNFragment) Applicable(params *template.ProvisionParams) bool {
	return params.CDN.Enabled
}

func (f *CDNFragment) Validate(params *template.ProvisionParams) error {
	c := params.CDN
	if !c.Enabled {
		return nil
	}
	if c.OriginType == "" {
		return fmt.Errorf("cdn.origin_type required when cdn.enabled is true")
	}
	if params.WorkloadExposure == "private" {
		return fmt.Errorf("cdn cannot be enabled for private workload exposure")
	}
	return nil
}

func (f *CDNFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	c := params.CDN

	spec := map[string]any{
		"enabled":     true,
		"originType":  c.OriginType,
		"priceClass":  resolve.Coalesce(c.PriceClass, "100"),
		"httpVersion": "http2and3",
		"ipv6":        true,
	}

	if c.CachePolicyTTL > 0 {
		spec["cachePolicyTTL"] = c.CachePolicyTTL
	} else {
		spec["cachePolicyTTL"] = 86400
	}

	if len(c.CustomDomains) > 0 {
		spec["customDomains"] = c.CustomDomains
		spec["tlsCertificate"] = "acm-auto"
	}

	if c.WAF {
		spec["waf"] = map[string]any{
			"enabled": true,
			"ruleSet": resolve.WAFRuleSet(params.Compliance),
		}
	}

	spec["logging"] = map[string]any{"enabled": true}
	spec["responseHeaders"] = map[string]any{
		"strictTransportSecurity": "max-age=63072000; includeSubDomains; preload",
		"contentTypeOptions":      "nosniff",
		"frameOptions":            "DENY",
		"xssProtection":           "1; mode=block",
	}

	return map[string]any{"cdn": spec}, nil
}
