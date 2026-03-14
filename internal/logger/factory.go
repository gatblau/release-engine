// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Factory creates component-scoped loggers.
type Factory interface {
	New(component string) *zap.Logger
}

type factory struct {
	level  zapcore.Level
	format string
}

// NewFactory creates a new logger factory.
func NewFactory(level string, format string) (Factory, error) {
	lvl := zapcore.InfoLevel
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, &LoggerError{Err: ErrLogLevelInvalid, Code: "LOG_LEVEL_INVALID", Detail: map[string]string{"level": level}}
	}

	return &factory{
		level:  lvl,
		format: format,
	}, nil
}

// New creates a new component-scoped logger.
func (f *factory) New(component string) *zap.Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if f.format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), f.level)
	return zap.New(core).With(
		zap.String("service", "release-engine"),
		zap.String("component", component),
	)
}
