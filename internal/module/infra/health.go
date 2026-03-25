// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package infra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// HealthStatus represents the health status of provisioned infrastructure
type HealthStatus struct {
	Status    string              `json:"status"` // "healthy", "unhealthy", "unknown"
	Timestamp time.Time           `json:"timestamp"`
	Resources []ResourceReference `json:"resources"`
	CommitSHA string              `json:"commit_sha"`
}

// ResourceReference represents a reference to a provisioned resource
type ResourceReference struct {
	Type string `json:"type"` // e.g., "cluster", "vpc", "database"
	ARN  string `json:"arn"`  // AWS ARN or similar identifier
	ID   string `json:"id"`   // Cloud provider resource ID
}

// pollHealthStatus polls the health status via git.read_file reading ArgoCD-written status file
func pollHealthStatus(ctx context.Context, step stepAPI, repo, branch, commitSHA string, healthTimeout, pollInterval time.Duration) (*HealthStatus, error) {
	startTime := time.Now()
	statusFilePath := ".status.yaml" // ArgoCD writes this file
	pollCount := 0

	// Get logger from step API for debugging
	logger := step.Logger()
	if logger == nil {
		// Fallback to no-op logger
		logger = zap.NewNop()
	}

	logger.Info("starting health polling",
		zap.String("repo", repo),
		zap.String("branch", branch),
		zap.String("commitSHA", commitSHA),
		zap.Duration("healthTimeout", healthTimeout),
		zap.Duration("pollInterval", pollInterval))

	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			logger.Info("health polling context cancelled", zap.Error(ctx.Err()))
			return nil, fmt.Errorf("health polling cancelled: %w", ctx.Err())
		default:
		}

		// Check if timeout exceeded
		if time.Since(startTime) > healthTimeout {
			logger.Warn("health polling timeout exceeded", zap.Duration("elapsed", time.Since(startTime)))
			return nil, fmt.Errorf("health polling timeout exceeded")
		}

		pollCount++
		if pollCount%10 == 0 { // Log every 10 polls to avoid too much noise
			logger.Debug("health polling attempt",
				zap.Int("attempt", pollCount),
				zap.Duration("elapsed", time.Since(startTime)))
		}

		// Read status file via git connector
		connectorReq, err := MapToConnectorRequest(map[string]any{
			"connector": "git",
			"operation": "read_file",
			"input": map[string]any{
				"repo":   repo,
				"branch": branch,
				"path":   statusFilePath,
			},
		})
		if err != nil {
			logger.Error("failed to create connector request", zap.Error(err))
			return nil, fmt.Errorf("failed to create connector request: %w", err)
		}

		readResult, err := step.CallConnector(ctx, connectorReq)
		if err != nil {
			logger.Error("failed to read status file", zap.Error(err))
			return nil, fmt.Errorf("failed to read status file: %w", err)
		}

		// Check if file exists
		if readResult.Status == "error" {
			// File not found yet, continue polling
			logger.Debug("status file not found, waiting for next poll")
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("health polling cancelled: %w", ctx.Err())
			case <-time.After(pollInterval):
				continue
			}
		}

		content, ok := readResult.Output["content"].(string)
		if !ok {
			logger.Error("invalid status file content type", zap.Any("output", readResult.Output))
			return nil, fmt.Errorf("invalid status file content")
		}

		// Parse status file with proper validation
		status, err := parseStatusFile(content, commitSHA)
		if err != nil {
			logger.Error("failed to parse status file", zap.Error(err))
			return nil, fmt.Errorf("failed to parse status file: %w", err)
		}

		logger.Debug("parsed health status",
			zap.String("status", status.Status),
			zap.String("commitSHA", status.CommitSHA),
			zap.Time("timestamp", status.Timestamp))

		if status.Status == "healthy" {
			logger.Info("health polling successful",
				zap.String("status", status.Status),
				zap.Int("pollCount", pollCount),
				zap.Duration("totalTime", time.Since(startTime)))
			return status, nil
		}

		// If unhealthy, continue polling for health transition
		logger.Debug("status not healthy yet", zap.String("currentStatus", status.Status))
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("health polling cancelled: %w", ctx.Err())
		case <-time.After(pollInterval):
			continue
		}
	}
}

// parseStatusFile parses the ArgoCD status file content
func parseStatusFile(content string, expectedCommitSHA string) (*HealthStatus, error) {
	// Define a temporary struct for parsing
	var parsed struct {
		Status    string              `yaml:"status"`
		Timestamp string              `yaml:"timestamp"`
		CommitSHA string              `yaml:"commit_sha"`
		Resources []ResourceReference `yaml:"resources"`
	}

	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		// Fallback to simple parsing if YAML is malformed
		fmt.Printf("[WARN] Failed to parse status YAML: %v, falling back to simple parser\n", err)
		return parseStatusFileSimple(content, expectedCommitSHA)
	}

	status := &HealthStatus{
		Status:    parsed.Status,
		CommitSHA: parsed.CommitSHA,
		Resources: parsed.Resources,
	}

	// Parse timestamp
	if parsed.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, parsed.Timestamp); err == nil {
			status.Timestamp = t
		} else {
			status.Timestamp = time.Now()
		}
	} else {
		status.Timestamp = time.Now()
	}

	// Set default status if empty
	if status.Status == "" {
		status.Status = "unknown"
	}

	// Validate commit SHA matches expected
	if status.CommitSHA != expectedCommitSHA {
		return nil, fmt.Errorf("status file commit SHA mismatch: expected %s, got %s", expectedCommitSHA, status.CommitSHA)
	}

	return status, nil
}

// parseStatusFileSimple provides backward compatibility with simple parsing
func parseStatusFileSimple(content string, expectedCommitSHA string) (*HealthStatus, error) {
	lines := strings.Split(content, "\n")
	var status HealthStatus
	status.Status = "unknown"
	status.Timestamp = time.Now()

	for _, line := range lines {
		if strings.Contains(line, "status:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				status.Status = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "commit_sha:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				status.CommitSHA = strings.TrimSpace(parts[1])
			}
		}
	}

	// Validate commit SHA matches expected
	if status.CommitSHA != expectedCommitSHA {
		return nil, fmt.Errorf("status file commit SHA mismatch: expected %s, got %s", expectedCommitSHA, status.CommitSHA)
	}

	return &status, nil
}

// checkHealth performs a health check using the Query() method internally
func checkHealth(ctx context.Context, step stepAPI, repo, branch, commitSHA string) (*HealthStatus, error) {
	logger := step.Logger()
	if logger == nil {
		logger = zap.NewNop()
	}

	logger.Debug("checking health via Query()",
		zap.String("repo", repo),
		zap.String("branch", branch),
		zap.String("commitSHA", commitSHA))

	// For now, use the original pollHealthStatus
	// In production, we could switch to pollHealthStatusParallel
	// but for tests, we need to use the original method
	return pollHealthStatus(ctx, step, repo, branch, commitSHA, 30*time.Second, 500*time.Millisecond)
}

// remediationRecommit performs remediation by recommitting manifests with force-sync annotation
func remediationRecommit(ctx context.Context, step stepAPI, repo, branch, pathPrefix string, files map[string]any, message string) (string, error) {
	// Add force-sync annotation to message
	forceSyncMessage := fmt.Sprintf("%s [force-sync]", message)

	connectorReq, err := MapToConnectorRequest(map[string]any{
		"connector": "git",
		"operation": "commit_files",
		"input": map[string]any{
			"repo":            repo,
			"branch":          branch,
			"path_prefix":     pathPrefix,
			"files":           files,
			"message":         forceSyncMessage,
			"idempotency_key": fmt.Sprintf("remediation-%d", time.Now().Unix()),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create connector request: %w", err)
	}

	commitResult, err := step.CallConnector(ctx, connectorReq)
	if err != nil {
		return "", fmt.Errorf("remediation commit failed: %w", err)
	}

	// Extract commit SHA from structured result
	var commitSHA string
	if commitResult.Output != nil {
		if sha, ok := commitResult.Output["commit_sha"].(string); ok {
			commitSHA = sha
		}
	}

	if commitSHA == "" {
		return "", fmt.Errorf("invalid commit SHA in remediation result")
	}

	return commitSHA, nil
}
