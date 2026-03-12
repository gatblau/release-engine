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

type GitHubNormalizer struct{}

func NewGitHubNormalizer() *GitHubNormalizer {
	return &GitHubNormalizer{}
}

func (n *GitHubNormalizer) Provider() string { return "github" }

func (n *GitHubNormalizer) Normalize(_ context.Context, tenantID string, serviceRef string, headers map[string]string, body []byte) ([]Event, error) {
	eventType := strings.TrimSpace(headers["x-github-event"])
	deliveryID := strings.TrimSpace(headers["x-github-delivery"])

	switch eventType {
	case "push":
		return normalizeGitHubPush(tenantID, serviceRef, deliveryID, body)
	case "pull_request":
		return normalizeGitHubPullRequest(tenantID, serviceRef, deliveryID, body)
	default:
		return []Event{}, nil
	}
}

type githubPushPayload struct {
	Ref     string `json:"ref"`
	Commits []struct {
		ID        string `json:"id"`
		Timestamp string `json:"timestamp"`
		Message   string `json:"message"`
	} `json:"commits"`
}

func normalizeGitHubPush(tenantID, serviceRef, deliveryID string, body []byte) ([]Event, error) {
	var payload githubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse github push payload: %w", err)
	}

	out := make([]Event, 0, len(payload.Commits))
	for i, c := range payload.Commits {
		sha := strings.TrimSpace(c.ID)
		if sha == "" {
			continue
		}

		ts := time.Now().UTC()
		if strings.TrimSpace(c.Timestamp) != "" {
			parsed, err := time.Parse(time.RFC3339, c.Timestamp)
			if err != nil {
				return nil, fmt.Errorf("parse github push commit timestamp: %w", err)
			}
			ts = parsed.UTC()
		}

		sourceID := strings.TrimSpace(deliveryID)
		if sourceID != "" {
			sourceID = fmt.Sprintf("%s:commit.pushed:%s:%d", sourceID, sha, i)
		}

		out = append(out, Event{
			TenantID:       tenantID,
			EventType:      "commit.pushed",
			EventSource:    "github",
			ServiceRef:     serviceRef,
			CorrelationKey: sha,
			SourceEventID:  sourceID,
			EventTimestamp: ts,
			Payload: map[string]any{
				"sha":     sha,
				"ref":     payload.Ref,
				"message": c.Message,
			},
		})
	}
	return out, nil
}

type githubPullRequestPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Merged         bool   `json:"merged"`
		MergedAt       string `json:"merged_at"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		Head           struct {
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
}

func normalizeGitHubPullRequest(tenantID, serviceRef, deliveryID string, body []byte) ([]Event, error) {
	var payload githubPullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse github pull_request payload: %w", err)
	}

	if !strings.EqualFold(payload.Action, "closed") || !payload.PullRequest.Merged {
		return []Event{}, nil
	}

	sha := strings.TrimSpace(payload.PullRequest.MergeCommitSHA)
	if sha == "" {
		sha = strings.TrimSpace(payload.PullRequest.Head.SHA)
	}
	if sha == "" {
		return nil, fmt.Errorf("github pull_request merged event missing commit sha")
	}

	ts := time.Now().UTC()
	if strings.TrimSpace(payload.PullRequest.MergedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, payload.PullRequest.MergedAt)
		if err != nil {
			return nil, fmt.Errorf("parse github pull_request merged_at: %w", err)
		}
		ts = parsed.UTC()
	}

	sourceID := strings.TrimSpace(deliveryID)
	if sourceID != "" {
		sourceID = fmt.Sprintf("%s:commit.merged:%s", sourceID, sha)
	}

	return []Event{{
		TenantID:       tenantID,
		EventType:      "commit.merged",
		EventSource:    "github",
		ServiceRef:     serviceRef,
		CorrelationKey: sha,
		SourceEventID:  sourceID,
		EventTimestamp: ts,
		Payload: map[string]any{
			"sha": sha,
		},
	}}, nil
}
