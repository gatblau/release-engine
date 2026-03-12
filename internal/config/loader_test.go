package config

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestLoader_Load(t *testing.T) {
	// Setup environment
	_ = os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	_ = os.Setenv("OIDC_ISSUER_URL", "https://issuer")
	_ = os.Setenv("OIDC_AUDIENCE", "audience")
	_ = os.Setenv("DORA_PROMETHEUS_ENABLED", "false")
	_ = os.Setenv("VOLTA_SM_SECRET_ID", "secret")
	_ = os.Setenv("VOLTA_S3_BUCKET", "bucket")
	defer os.Clearenv()

	loader := NewLoader()
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if cfg.HTTPPort != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.HTTPPort)
	}
	if cfg.DoraPrometheusEnabled {
		t.Errorf("expected DoraPrometheusEnabled false, got true")
	}
}

func TestLoader_Load_InvalidDoraPrometheusEnabled(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	_ = os.Setenv("OIDC_ISSUER_URL", "https://issuer")
	_ = os.Setenv("OIDC_AUDIENCE", "audience")
	_ = os.Setenv("DORA_PROMETHEUS_ENABLED", "not-a-bool")
	_ = os.Setenv("VOLTA_SM_SECRET_ID", "secret")
	_ = os.Setenv("VOLTA_S3_BUCKET", "bucket")
	defer os.Clearenv()

	loader := NewLoader()
	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Errorf("expected ConfigError, got %T", err)
	}
	if cfgErr.Code != "CONFIG_INVALID" {
		t.Errorf("expected code CONFIG_INVALID, got %s", cfgErr.Code)
	}
}

func TestLoader_Load_MissingVar(t *testing.T) {
	// Environment empty
	os.Clearenv()

	loader := NewLoader()
	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Errorf("expected ConfigError, got %T", err)
	}

	if cfgErr.Code != "CONFIG_MISSING" {
		t.Errorf("expected code CONFIG_MISSING, got %s", cfgErr.Code)
	}
}
