package http

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApprovalWorker_Tick_RecordsWorkerMetric(t *testing.T) {
	metrics := &approvalMetricsStub{}
	service := NewApprovalService(NewPolicyEngine())
	service.AttachMetrics(metrics)

	worker := NewApprovalWorker(service, 10*time.Millisecond)
	worker.Tick(context.Background())

	assert.Equal(t, 1, metrics.workerTicks)
}
