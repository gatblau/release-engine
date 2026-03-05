package config

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds immutable runtime configuration.
type Config struct {
	HTTPPort        int
	DatabaseURL     string
	OIDCIssuerURL   string
	OIDCAudience    string
	VoltaSMSecretID string
	VoltaS3Bucket   string
}

// Loader loads and validates configuration.
type Loader interface {
	Load(ctx context.Context) (Config, error)
}

type loader struct{}

// NewLoader creates a new configuration loader.
func NewLoader() Loader {
	return &loader{}
}

// Load loads configuration from environment variables.
func (l *loader) Load(ctx context.Context) (Config, error) {
	_ = godotenv.Load()

	getRequired := func(key string) (string, error) {
		val := os.Getenv(key)
		if val == "" {
			return "", fmt.Errorf("missing required configuration: %s", key)
		}
		return val, nil
	}

	dbURL, err := getRequired("DATABASE_URL")
	if err != nil {
		return Config{}, fmt.Errorf("CONFIG_MISSING: %w", err)
	}

	oidcIssuer, err := getRequired("OIDC_ISSUER_URL")
	if err != nil {
		return Config{}, fmt.Errorf("CONFIG_MISSING: %w", err)
	}

	oidcAudience, err := getRequired("OIDC_AUDIENCE")
	if err != nil {
		return Config{}, fmt.Errorf("CONFIG_MISSING: %w", err)
	}

	voltaSMSecretID, err := getRequired("VOLTA_SM_SECRET_ID")
	if err != nil {
		return Config{}, fmt.Errorf("CONFIG_MISSING: %w", err)
	}

	voltaS3Bucket, err := getRequired("VOLTA_S3_BUCKET")
	if err != nil {
		return Config{}, fmt.Errorf("CONFIG_MISSING: %w", err)
	}

	httpPortStr := os.Getenv("HTTP_PORT")
	if httpPortStr == "" {
		httpPortStr = "8080"
	}
	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		return Config{}, fmt.Errorf("CONFIG_INVALID: invalid HTTP_PORT: %w", err)
	}

	return Config{
		HTTPPort:        httpPort,
		DatabaseURL:     dbURL,
		OIDCIssuerURL:   oidcIssuer,
		OIDCAudience:    oidcAudience,
		VoltaSMSecretID: voltaSMSecretID,
		VoltaS3Bucket:   voltaS3Bucket,
	}, nil
}
