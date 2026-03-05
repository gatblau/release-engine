package config

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoader_Load_MissingConfig(t *testing.T) {
	os.Clearenv()
	l := NewLoader()
	cfg, err := l.Load(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CONFIG_MISSING")
	assert.Empty(t, cfg)
}

func TestLoader_Load_Success(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	_ = os.Setenv("OIDC_ISSUER_URL", "https://auth.example.com")
	_ = os.Setenv("OIDC_AUDIENCE", "api")
	_ = os.Setenv("VOLTA_SM_SECRET_ID", "secret")
	_ = os.Setenv("VOLTA_S3_BUCKET", "bucket")
	defer os.Clearenv()

	l := NewLoader()
	cfg, err := l.Load(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 8080, cfg.HTTPPort)
	assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.DatabaseURL)
}
