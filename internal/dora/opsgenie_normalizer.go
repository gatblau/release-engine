// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package dora

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type OpsgenieNormalizer struct{}

func NewOpsgenieNormalizer() *OpsgenieNormalizer {
	return &OpsgenieNormalizer{}
}

func (n *OpsgenieNormalizer) Provider() string { return "opsgenie" }

func (n *OpsgenieNormalizer) Normalize(_ context.Context, tenantID string, serviceRef string, headers map[string]string, body []byte) ([]Event, error) {
	var payload struct {
		Action string `json:"action"`
		Data   struct {
			Alert struct {
				ID        string `json:"id"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
			} `json:"alert"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse opsgenie payload: %w", err)
	}

	incidentID := strings.TrimSpace(payload.Data.Alert.ID)
	if incidentID == "" {
		return nil, fmt.Errorf("opsgenie payload missing alert id")
	}

	timestamp := time.Now().UTC()
	if raw := strings.TrimSpace(payload.Data.Alert.CreatedAt); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			timestamp = parsed.UTC()
		}
	}

	eventType := ""
	action := strings.ToLower(strings.TrimSpace(payload.Action))
	switch action {
	case "create", "created", "opened", "open":
		eventType = "incident.opened"
	case "close", "closed", "resolved", "acknowledged":
		eventType = "incident.resolved"
		if raw := strings.TrimSpace(payload.Data.Alert.UpdatedAt); raw != "" {
			if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
				timestamp = parsed.UTC()
			}
		}
	default:
		return []Event{}, nil
	}

	deliveryID := strings.TrimSpace(headers["x-request-id"])
	if deliveryID == "" {
		deliveryID = strings.TrimSpace(headers["x-opsgenie-webhook-id"])
	}
	if deliveryID == "" {
		deliveryID = strings.TrimSpace(headers["x-opsgenie-delivery"])
	}
	sourceID := deliveryID
	if sourceID != "" {
		sourceID = fmt.Sprintf("%s:%s:%s", sourceID, eventType, incidentID)
	}

	return []Event{{
		TenantID:       tenantID,
		EventType:      eventType,
		EventSource:    "opsgenie",
		ServiceRef:     serviceRef,
		CorrelationKey: incidentID,
		SourceEventID:  sourceID,
		EventTimestamp: timestamp,
		Payload: map[string]any{
			"incident_id": incidentID,
			"action":      payload.Action,
		},
	}}, nil
}
