package http

import (
	"context"
	"time"
)

// ApprovalWorker polls approval steps and applies escalation/expiry rules.
type ApprovalWorker struct {
	service  *ApprovalService
	interval time.Duration
}

func NewApprovalWorker(service *ApprovalService, interval time.Duration) *ApprovalWorker {
	if service == nil {
		service = NewApprovalService(NewPolicyEngine())
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &ApprovalWorker{service: service, interval: interval}
}

func (w *ApprovalWorker) Tick(ctx context.Context) {
	start := time.Now()
	status := "success"
	defer func() {
		if w.service != nil && w.service.metrics != nil {
			w.service.metrics.RecordApprovalWorkerTick(status, time.Since(start))
		}
	}()

	w.service.TickApprovals(ctx)
}

func (w *ApprovalWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Tick(ctx)
		}
	}
}
