// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
)

type DNSFragment struct{}

func (f *DNSFragment) Name() string { return "dns" }

func (f *DNSFragment) Applicable(params *template.ProvisionParams) bool {
	return params.DNS.Enabled
}

func (f *DNSFragment) Validate(params *template.ProvisionParams) error {
	d := params.DNS
	if !d.Enabled {
		return nil
	}
	if d.ZoneName == "" {
		return fmt.Errorf("dns.zone_name required when dns.enabled is true")
	}
	for i, r := range d.Records {
		if len(r.Values) == 0 {
			return fmt.Errorf("dns.records[%d].values requires at least one value", i)
		}
	}
	return nil
}

func (f *DNSFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	d := params.DNS

	spec := map[string]any{
		"enabled":  true,
		"zoneName": d.ZoneName,
		"private":  d.Private,
	}

	if len(d.Records) > 0 {
		records := make([]map[string]any, len(d.Records))
		for i, r := range d.Records {
			rec := map[string]any{
				"name":   r.Name,
				"type":   r.Type,
				"values": r.Values,
			}
			if r.TTL > 0 {
				rec["ttl"] = r.TTL
			} else {
				rec["ttl"] = 300
			}
			records[i] = rec
		}
		spec["records"] = records
	}

	if d.Private {
		spec["vpcAssociation"] = true
	}

	return map[string]any{"dns": spec}, nil
}
