// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package infra

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/stepapi"
)

// MapToConnectorRequest converts a map-based request to a structured ConnectorRequest.
func MapToConnectorRequest(m map[string]any) (stepapi.ConnectorRequest, error) {
	connector, _ := m["connector"].(string)
	implKey, _ := m["impl_key"].(string)
	operation, _ := m["operation"].(string)
	input, _ := m["input"].(map[string]any)

	if connector == "" || operation == "" {
		return stepapi.ConnectorRequest{}, fmt.Errorf("connector and operation are required")
	}

	return stepapi.ConnectorRequest{
		Connector: connector,
		ImplKey:   implKey,
		Operation: operation,
		Input:     input,
	}, nil
}

// ConnectorResultToMap converts a structured ConnectorResult to a map.
func ConnectorResultToMap(result *stepapi.ConnectorResult) map[string]any {
	if result == nil {
		return nil
	}

	m := map[string]any{
		"status": result.Status,
	}

	if result.Output != nil {
		m["output"] = result.Output
	}

	if result.Error != nil {
		m["error"] = map[string]any{
			"code":    result.Error.Code,
			"message": result.Error.Message,
		}
	}

	return m
}

// MapToApprovalRequest converts a map-based approval request to a structured ApprovalRequest.
func MapToApprovalRequest(m map[string]any) (stepapi.ApprovalRequest, error) {
	summary, _ := m["summary"].(string)
	detail, _ := m["detail"].(string)
	blastRadius, _ := m["blast_radius"].(string)
	policyRef, _ := m["policy_ref"].(string)

	metadataRaw, _ := m["metadata"].(map[string]any)
	metadata := make(map[string]string)
	for k, v := range metadataRaw {
		if s, ok := v.(string); ok {
			metadata[k] = s
		}
	}

	return stepapi.ApprovalRequest{
		Summary:     summary,
		Detail:      detail,
		BlastRadius: blastRadius,
		PolicyRef:   policyRef,
		Metadata:    metadata,
	}, nil
}

// ApprovalOutcomeToMap converts a structured ApprovalOutcome to a map.
func ApprovalOutcomeToMap(outcome stepapi.ApprovalOutcome) map[string]any {
	m := map[string]any{
		"decision":      outcome.Decision,
		"approver":      outcome.Approver,
		"justification": outcome.Justification,
		"decided_at":    outcome.DecidedAt,
	}

	// Also include the fields in a flat structure for backward compatibility
	m["decision"] = outcome.Decision
	m["approver"] = outcome.Approver

	return m
}
