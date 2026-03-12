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

type GitLabNormalizer struct{}

func NewGitLabNormalizer() *GitLabNormalizer {
	return &GitLabNormalizer{}
}

func (n *GitLabNormalizer) Provider() string { return "gitlab" }

func (n *GitLabNormalizer) Normalize(_ context.Context, tenantID string, serviceRef string, headers map[string]string, body []byte) ([]Event, error) {
	eventType := strings.TrimSpace(headers["x-gitlab-event"])
	deliveryID := strings.TrimSpace(headers["x-gitlab-delivery"])

	switch {
	case strings.EqualFold(eventType, "Push Hook"):
		return normalizeGitLabPush(tenantID, serviceRef, deliveryID, body)
	case strings.EqualFold(eventType, "Merge Request Hook"):
		return normalizeGitLabMergeRequest(tenantID, serviceRef, deliveryID, body)
	default:
		return []Event{}, nil
	}
}

type gitlabPushPayload struct {
	Ref     string `json:"ref"`
	Commits []struct {
		ID        string `json:"id"`
		Timestamp string `json:"timestamp"`
		Message   string `json:"message"`
	} `json:"commits"`
}

func normalizeGitLabPush(tenantID, serviceRef, deliveryID string, body []byte) ([]Event, error) {
	var payload gitlabPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse gitlab push payload: %w", err)
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
				return nil, fmt.Errorf("parse gitlab push commit timestamp: %w", err)
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
			EventSource:    "gitlab",
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

type gitlabMergeRequestPayload struct {
	ObjectAttributes struct {
		State          string `json:"state"`
		MergedAt       string `json:"merged_at"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		LastCommit     struct {
			ID string `json:"id"`
		} `json:"last_commit"`
	} `json:"object_attributes"`
}

func normalizeGitLabMergeRequest(tenantID, serviceRef, deliveryID string, body []byte) ([]Event, error) {
	var payload gitlabMergeRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse gitlab merge request payload: %w", err)
	}

	if !strings.EqualFold(payload.ObjectAttributes.State, "merged") {
		return []Event{}, nil
	}

	sha := strings.TrimSpace(payload.ObjectAttributes.MergeCommitSHA)
	if sha == "" {
		sha = strings.TrimSpace(payload.ObjectAttributes.LastCommit.ID)
	}
	if sha == "" {
		return nil, fmt.Errorf("gitlab merge request merged event missing commit sha")
	}

	ts := time.Now().UTC()
	if strings.TrimSpace(payload.ObjectAttributes.MergedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, payload.ObjectAttributes.MergedAt)
		if err != nil {
			return nil, fmt.Errorf("parse gitlab merge request merged_at: %w", err)
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
		EventSource:    "gitlab",
		ServiceRef:     serviceRef,
		CorrelationKey: sha,
		SourceEventID:  sourceID,
		EventTimestamp: ts,
		Payload: map[string]any{
			"sha": sha,
		},
	}}, nil
}
