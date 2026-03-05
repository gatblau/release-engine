package logger

import (
	"errors"
	"testing"
)

func TestNewFactory(t *testing.T) {
	f, err := NewFactory("debug", "json")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	logger := f.New("test-component")
	if logger == nil {
		t.Fatal("expected logger, got nil")
	}
}

func TestNewFactory_InvalidLevel(t *testing.T) {
	_, err := NewFactory("invalid", "json")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var lErr *LoggerError
	if !errors.As(err, &lErr) {
		t.Errorf("expected LoggerError, got %T", err)
	}

	if lErr.Code != "LOG_LEVEL_INVALID" {
		t.Errorf("expected code LOG_LEVEL_INVALID, got %s", lErr.Code)
	}
}
