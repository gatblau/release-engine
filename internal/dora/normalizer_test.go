package dora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterResolveProviders(t *testing.T) {
	r := NewRegistry()
	n := NewGitHubNormalizer()
	r.Register(n)
	r.Register(NewGitLabNormalizer())
	r.Register(NewOpsgenieNormalizer())
	r.Register(NewDatadogNormalizer())

	resolved := r.Resolve("github")
	require.NotNil(t, resolved)
	assert.Equal(t, "github", resolved.Provider())
	assert.Equal(t, []string{"datadog", "github", "gitlab", "opsgenie"}, r.Providers())
}

func TestRegistry_RegisterDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(NewGitHubNormalizer())
	assert.Panics(t, func() { r.Register(NewGitHubNormalizer()) })
}

func TestGitHubNormalizer_Push(t *testing.T) {
	n := NewGitHubNormalizer()
	headers := map[string]string{"x-github-event": "push", "x-github-delivery": "delivery-1"}
	body := []byte(`{"ref":"refs/heads/main","commits":[{"id":"abc123","timestamp":"2026-03-12T10:00:00Z","message":"feat"}]}`)

	events, err := n.Normalize(context.Background(), "t-1", "svc-a", headers, body)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "commit.pushed", events[0].EventType)
	assert.Equal(t, "abc123", events[0].CorrelationKey)
	assert.Equal(t, "delivery-1:commit.pushed:abc123:0", events[0].SourceEventID)
}

func TestGitHubNormalizer_PullRequestMerged(t *testing.T) {
	n := NewGitHubNormalizer()
	headers := map[string]string{"x-github-event": "pull_request", "x-github-delivery": "delivery-2"}
	body := []byte(`{"action":"closed","pull_request":{"merged":true,"merged_at":"2026-03-12T11:00:00Z","merge_commit_sha":"def456"}}`)

	events, err := n.Normalize(context.Background(), "t-1", "svc-a", headers, body)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "commit.merged", events[0].EventType)
	assert.Equal(t, "def456", events[0].CorrelationKey)
}

func TestGitLabNormalizer_Push(t *testing.T) {
	n := NewGitLabNormalizer()
	headers := map[string]string{"x-gitlab-event": "Push Hook", "x-gitlab-delivery": "delivery-3"}
	body := []byte(`{"ref":"refs/heads/main","commits":[{"id":"abc123","timestamp":"2026-03-12T10:00:00Z","message":"feat"}]}`)

	events, err := n.Normalize(context.Background(), "t-1", "svc-a", headers, body)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "commit.pushed", events[0].EventType)
	assert.Equal(t, "abc123", events[0].CorrelationKey)
	assert.Equal(t, "delivery-3:commit.pushed:abc123:0", events[0].SourceEventID)
}

func TestOpsgenieNormalizer_Open(t *testing.T) {
	n := NewOpsgenieNormalizer()
	headers := map[string]string{"x-request-id": "og-1"}
	body := []byte(`{"action":"created","data":{"alert":{"id":"inc-1","createdAt":"2026-03-12T10:00:00Z"}}}`)

	events, err := n.Normalize(context.Background(), "t-1", "svc-a", headers, body)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "incident.opened", events[0].EventType)
	assert.Equal(t, "inc-1", events[0].CorrelationKey)
}

func TestDatadogNormalizer_Resolved(t *testing.T) {
	n := NewDatadogNormalizer()
	headers := map[string]string{"dd-request-id": "dd-1"}
	body := []byte(`{"event_type":"incident_resolved","id":"inc-2","status":"resolved","created_at":"2026-03-12T10:00:00Z","updated_at":"2026-03-12T11:00:00Z"}`)

	events, err := n.Normalize(context.Background(), "t-1", "svc-a", headers, body)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "incident.resolved", events[0].EventType)
	assert.Equal(t, "inc-2", events[0].CorrelationKey)
}

func TestValidateEvent(t *testing.T) {
	err := ValidateEvent(Event{TenantID: "t-1", ServiceRef: "svc", EventSource: "github", EventType: "commit.pushed", EventTimestamp: mustTime("2026-03-12T11:00:00Z")})
	assert.NoError(t, err)

	err = ValidateEvent(Event{TenantID: "t-1", ServiceRef: "svc", EventSource: "github", EventType: "unknown", EventTimestamp: mustTime("2026-03-12T11:00:00Z")})
	assert.Error(t, err)
}

func mustTime(v string) time.Time {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		panic(err)
	}
	return t
}
