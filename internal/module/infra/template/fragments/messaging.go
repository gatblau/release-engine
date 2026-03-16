// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package fragments

import (
	"fmt"

	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/resolve"
)

type MessagingFragment struct{}

func (f *MessagingFragment) Name() string { return "messaging" }

func (f *MessagingFragment) Applicable(params *template.ProvisionParams) bool {
	return params.Messaging.Enabled
}

func (f *MessagingFragment) Validate(params *template.ProvisionParams) error {
	m := params.Messaging
	if !m.Enabled {
		return nil
	}
	if m.Tier == "" {
		return fmt.Errorf("messaging.tier required when messaging.enabled is true")
	}
	if m.QueueCount == 0 && m.TopicCount == 0 {
		return fmt.Errorf("at least one of messaging.queue_count or messaging.topic_count must be > 0")
	}
	if m.DLQEnabled && m.DLQMaxRetry == 0 {
		return fmt.Errorf("messaging.dlq_max_retry required when messaging.dlq_enabled is true")
	}
	return nil
}

func (f *MessagingFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	m := params.Messaging

	spec := map[string]any{
		"enabled":   true,
		"tier":      m.Tier,
		"region":    params.PrimaryRegion,
		"encrypted": true,
	}

	if m.QueueCount > 0 {
		queueSpec := map[string]any{
			"count": m.QueueCount,
			"fifo":  m.FIFO,
		}
		if m.DLQEnabled {
			queueSpec["deadLetterQueue"] = map[string]any{
				"enabled":         true,
				"maxReceiveCount": resolve.CoalesceInt(m.DLQMaxRetry, 5),
			}
		}
		spec["queues"] = queueSpec
	}

	if m.TopicCount > 0 {
		spec["topics"] = map[string]any{
			"count": m.TopicCount,
		}
	}

	if m.Tier == "enterprise" {
		spec["clusterMode"] = true
		spec["messageRetention"] = "14d"
	}

	return map[string]any{"messaging": spec}, nil
}
