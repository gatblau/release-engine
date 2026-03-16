package fragments

import (
	"testing"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessagingFragment_ValidateRequiresCounts(t *testing.T) {
	f := &MessagingFragment{}
	p := &template.ProvisionParams{
		Messaging: template.MessagingParams{Enabled: true, Tier: "standard"},
	}

	err := f.Validate(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of messaging.queue_count or messaging.topic_count")
}

func TestMessagingFragment_RenderQueuesAndTopics(t *testing.T) {
	f := &MessagingFragment{}
	p := &template.ProvisionParams{
		PrimaryRegion: "eu-west-1",
		Messaging: template.MessagingParams{
			Enabled:    true,
			Tier:       "enterprise",
			QueueCount: 2,
			TopicCount: 1,
			DLQEnabled: true,
		},
	}

	out, err := f.Render(p)
	require.NoError(t, err)
	m := out["messaging"].(map[string]any)
	assert.Contains(t, m, "queues")
	assert.Contains(t, m, "topics")
	assert.Equal(t, true, m["clusterMode"])
}
