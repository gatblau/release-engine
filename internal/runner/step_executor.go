// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type StepExecutor struct {
	registry connector.ConnectorRegistry
	logger   *zap.Logger
	metrics  *observability.MetricsExporter
	tracer   *observability.TracingService
}

func NewStepExecutor(
	registry connector.ConnectorRegistry,
	logger *zap.Logger,
	metrics *observability.MetricsExporter,
	tracer *observability.TracingService,
) *StepExecutor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StepExecutor{
		registry: registry,
		logger:   logger,
		metrics:  metrics,
		tracer:   tracer,
	}
}

func (s *StepExecutor) Execute(
	ctx context.Context,
	connectorKey string,
	operation string,
	input map[string]interface{},
	timeout time.Duration,
) (res *connector.ConnectorResult, err error) {
	start := time.Now()

	s.logger.Info("connector.execute.start",
		zap.String("connector", connectorKey),
		zap.String("operation", operation),
	)

	conn, ok := s.registry.Lookup(connectorKey)
	if !ok {
		return nil, fmt.Errorf("connector not found: %s", connectorKey)
	}

	// Panic recovery and metrics logging
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("connector panic: %v", r)
			res = &connector.ConnectorResult{
				Status: connector.StatusTerminalError,
				Error: &connector.ConnectorError{
					Code:    "PANIC",
					Message: fmt.Sprintf("%v", r),
				},
			}
		}

		status := "success"
		if err != nil || (res != nil && res.Status == connector.StatusTerminalError) {
			status = "error"
		} else if res != nil && res.Status == connector.StatusRetryableError {
			status = "transient_error"
		}

		duration := time.Since(start)
		s.logger.Info("connector.execute.end",
			zap.String("connector", connectorKey),
			zap.String("operation", operation),
			zap.String("status", status),
			zap.Duration("duration", duration),
		)

		if s.metrics != nil {
			s.metrics.RecordConnectorCall(connectorKey, operation, status, duration)
		}
	}()

	var cancel context.CancelFunc
	spanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if s.tracer != nil {
		var span interface{}
		spanCtx, span = s.tracer.StartSpan(spanCtx, fmt.Sprintf("connector.%s.%s", connectorKey, operation))
		if spanAttr, ok := span.(interface {
			End(...interface{})
			SetAttributes(...attribute.KeyValue)
			RecordError(error, ...interface{})
		}); ok {
			spanAttr.SetAttributes(
				attribute.String("release.connector", connectorKey),
				attribute.String("release.operation", operation),
			)
			defer func() {
				if err != nil {
					spanAttr.RecordError(err)
				}
				spanAttr.End()
			}()
		}
	}

	res, err = conn.Execute(spanCtx, operation, input)

	// State Mapping: Normalize results
	if err != nil && res == nil {
		res = &connector.ConnectorResult{
			Status: connector.StatusTerminalError,
			Error: &connector.ConnectorError{
				Code:    "EXECUTION_ERROR",
				Message: err.Error(),
			},
		}
	}
	return res, err
}
