// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package resolve

// Coalesce returns the first non-empty string.
func Coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// CoalesceInt returns the first non-zero int.
func CoalesceInt(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

// CoalesceFloat returns the first non-zero float64.
func CoalesceFloat(values ...float64) float64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

// SafeName sanitizes a name for use in cloud resource identifiers.
// Phase 1 keeps this as a pass-through.
func SafeName(name string) string {
	return name
}

// SSLPolicy returns the SSL policy based on compliance requirements.
func SSLPolicy(compliance []string) string {
	for _, c := range compliance {
		if c == "pci-dss" || c == "hipaa" {
			return "ELBSecurityPolicy-TLS13-1-2-2021-06"
		}
	}
	return "ELBSecurityPolicy-TLS13-1-0-2021-06"
}

// WAFRuleSet returns the WAF rule set based on compliance.
func WAFRuleSet(compliance []string) string {
	for _, c := range compliance {
		if c == "pci-dss" {
			return "AWSManagedRulesPCIDSSRuleSet"
		}
	}
	return "AWSManagedRulesCommonRuleSet"
}

// PermissionBoundary returns the IAM permission boundary ARN.
func PermissionBoundary(tenant, environment string) string {
	return "arn:aws:iam::policy/" + tenant + "-" + environment + "-boundary"
}

// AlarmNotificationTarget returns the SNS ARN for alarm routing.
func AlarmNotificationTarget(tenant, environment string) string {
	return "arn:aws:sns:*:*:" + tenant + "-" + environment + "-alarms"
}

// DashboardTemplate returns a dashboard template name.
func DashboardTemplate(workloadType string) string {
	return workloadType + "-standard"
}

// DBReplicationType returns the replication strategy for DR.
func DBReplicationType(engine string) string {
	if engine == "aurora-postgres" || engine == "aurora-mysql" {
		return "aurora-global-database"
	}
	return "cross-region-read-replica"
}
