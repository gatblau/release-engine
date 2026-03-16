// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type ObservabilityFragment struct{}

func (f *ObservabilityFragment) Name() string { return "observability" }

func (f *ObservabilityFragment) Applicable(params *template.ProvisionParams) bool {
	return params.Observability.Enabled
}

func (f *ObservabilityFragment) Validate(params *template.ProvisionParams) error {
	_ = params
	return nil
}

func (f *ObservabilityFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	o := params.Observability

	spec := map[string]any{
		"enabled": true,
	}

	spec["metrics"] = map[string]any{
		"enabled":          true,
		"retentionDays":    resolve.CoalesceInt(o.MetricsRetentionDays, 90),
		"resolution":       resolve.Coalesce(o.MetricsResolution, "60s"),
		"customNamespaces": o.CustomMetricNamespaces,
	}

	logRetention := resolve.CoalesceInt(o.LogRetentionDays, 30)
	if params.DataClassification == "restricted" || params.DataClassification == "confidential" {
		logRetention = resolve.CoalesceInt(o.LogRetentionDays, 365)
	}
	spec["logging"] = map[string]any{
		"enabled":       true,
		"retentionDays": logRetention,
		"structured":    true,
		"sinkType":      resolve.Coalesce(o.LogSinkType, "cloudwatch"),
	}
	if o.ExternalLogSink != "" {
		spec["logging"].(map[string]any)["externalSink"] = o.ExternalLogSink
	}

	if o.TracingEnabled {
		spec["tracing"] = map[string]any{
			"enabled":    true,
			"sampleRate": resolve.CoalesceFloat(o.TracingSampleRate, 0.1),
			"provider":   resolve.Coalesce(o.TracingProvider, "xray"),
		}
	}

	if o.DashboardEnabled {
		spec["dashboards"] = map[string]any{
			"enabled":  true,
			"template": resolve.DashboardTemplate(params.WorkloadType),
		}
	}

	alarmSeverity := "standard"
	if params.Availability == "critical" {
		alarmSeverity = "critical"
	}
	spec["alarms"] = map[string]any{
		"enabled":         true,
		"severity":        alarmSeverity,
		"notificationArn": resolve.AlarmNotificationTarget(params.Tenant, params.Environment),
	}

	return map[string]any{"observability": spec}, nil
}
