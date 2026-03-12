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

type DatadogNormalizer struct{}

func NewDatadogNormalizer() *DatadogNormalizer {
	return &DatadogNormalizer{}
}

func (n *DatadogNormalizer) Provider() string { return "datadog" }

func (n *DatadogNormalizer) Normalize(_ context.Context, tenantID string, serviceRef string, headers map[string]string, body []byte) ([]Event, error) {
	var payload struct {
		EventType string `json:"event_type"`
		ID        string `json:"id"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse datadog payload: %w", err)
	}

	incidentID := strings.TrimSpace(payload.ID)
	if incidentID == "" {
		return nil, fmt.Errorf("datadog payload missing id")
	}

	timestamp := time.Now().UTC()
	if raw := strings.TrimSpace(payload.CreatedAt); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			timestamp = parsed.UTC()
		}
	}

	eventType := ""
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	eventHint := strings.ToLower(strings.TrimSpace(payload.EventType))
	switch {
	case strings.Contains(eventHint, "resolve") || strings.Contains(status, "resolved") || strings.Contains(status, "ok"):
		eventType = "incident.resolved"
		if raw := strings.TrimSpace(payload.UpdatedAt); raw != "" {
			if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
				timestamp = parsed.UTC()
			}
		}
	case strings.Contains(eventHint, "open") || strings.Contains(eventHint, "trigger") || strings.Contains(status, "trigger") || strings.Contains(status, "alert"):
		eventType = "incident.opened"
	default:
		return []Event{}, nil
	}

	deliveryID := strings.TrimSpace(headers["dd-request-id"])
	if deliveryID == "" {
		deliveryID = strings.TrimSpace(headers["x-datadog-request-id"])
	}
	if deliveryID == "" {
		deliveryID = strings.TrimSpace(headers["x-datadog-delivery"])
	}
	sourceID := deliveryID
	if sourceID != "" {
		sourceID = fmt.Sprintf("%s:%s:%s", sourceID, eventType, incidentID)
	}

	return []Event{{
		TenantID:       tenantID,
		EventType:      eventType,
		EventSource:    "datadog",
		ServiceRef:     serviceRef,
		CorrelationKey: incidentID,
		SourceEventID:  sourceID,
		EventTimestamp: timestamp,
		Payload: map[string]any{
			"incident_id": incidentID,
			"status":      payload.Status,
			"event_type":  payload.EventType,
		},
	}}, nil
}
