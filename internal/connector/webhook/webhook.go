// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
)

// WebhookConnector implements the webhook connector contract from docs/plan.md
type WebhookConnector struct {
	connector.BaseConnector
	config connector.ConnectorConfig
	client *http.Client
	mu     sync.RWMutex
	closed bool
}

// NewWebhookConnector creates a new webhook connector.
func NewWebhookConnector(cfg connector.ConnectorConfig) (*WebhookConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeWebHook, "webhook")
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: cfg.HTTPTimeout,
	}
	return &WebhookConnector{
		BaseConnector: base,
		config:        cfg,
		client:        client,
	}, nil
}

// Validate validates operation input.
func (c *WebhookConnector) Validate(operation string, input map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return fmt.Errorf("connector is closed")
	}

	if operation != "post_callback" {
		return fmt.Errorf("unknown operation: %s", operation)
	}

	// Required fields per docs/plan.md
	requiredFields := []string{"url", "headers", "body", "idempotency_key"}
	for _, field := range requiredFields {
		if _, ok := input[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	if _, ok := input["headers"].(map[string]string); !ok {
		if headers, ok := input["headers"].(map[string]interface{}); ok {
			for k, v := range headers {
				if _, ok := v.(string); !ok {
					return fmt.Errorf("header value for %s must be a string", k)
				}
			}
		} else {
			return fmt.Errorf("headers must be a map[string]string")
		}
	}

	// body can be any object
	if _, ok := input["body"]; !ok {
		return fmt.Errorf("body is required")
	}

	return nil
}

// RequiredSecrets returns the secrets required for webhook operations.
func (c *WebhookConnector) RequiredSecrets(operation string) []string {
	// Webhook connector currently doesn't require any secrets
	return []string{}
}

// Execute performs webhook callback.
func (c *WebhookConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*connector.ConnectorResult, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	c.mu.RUnlock()

	if operation != "post_callback" {
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}

	return c.postCallback(ctx, input)
}

// postCallback performs HTTP POST to the callback URL.
func (c *WebhookConnector) postCallback(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	url, _ := input["url"].(string)
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("url is required")
	}
	bodyBytes, err := json.Marshal(input["body"])
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if headers, ok := input["headers"].(map[string]string); ok {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	} else if headers, ok := input["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	return &connector.ConnectorResult{Status: connector.StatusSuccess, Output: map[string]interface{}{"status_code": resp.StatusCode, "response_body": string(respBody)}}, nil
}

// Close closes the connector.
func (c *WebhookConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// Operations returns the list of supported operations.
func (c *WebhookConnector) Operations() []connector.OperationMeta {
	return []connector.OperationMeta{
		{
			Name:           "post_callback",
			Description:    "Post callback to webhook URL",
			RequiredFields: []string{"url", "headers", "body", "idempotency_key"},
			OptionalFields: []string{},
			IsAsync:        false,
		},
	}
}
