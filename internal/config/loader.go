package config

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// VoltaEnvType defines the type of environment for Volta (dev or prod)
type VoltaEnvType string

const (
	VoltaEnvDev  VoltaEnvType = "dev"
	VoltaEnvProd VoltaEnvType = "prod"
)

// VoltaStorageType defines the storage backend for Volta (s3 or file)
type VoltaStorageType string

const (
	VoltaStorageS3   VoltaStorageType = "s3"
	VoltaStorageFile VoltaStorageType = "file"
)

// Config holds immutable runtime configuration.
type Config struct {
	HTTPPort              int
	DatabaseURL           string
	OIDCIssuerURL         string
	OIDCAudience          string
	VoltaEnvType          VoltaEnvType
	VoltaStorage          VoltaStorageType
	VoltaSMSecretID       string
	VoltaS3Bucket         string
	VoltaFilePath         string
	VoltaPassphraseEnvVar string
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
		return Config{}, &ConfigError{Err: ErrConfigMissing, Code: "CONFIG_MISSING", Detail: map[string]string{"var": "DATABASE_URL"}}
	}

	oidcIssuer, err := getRequired("OIDC_ISSUER_URL")
	if err != nil {
		return Config{}, &ConfigError{Err: ErrConfigMissing, Code: "CONFIG_MISSING", Detail: map[string]string{"var": "OIDC_ISSUER_URL"}}
	}

	oidcAudience, err := getRequired("OIDC_AUDIENCE")
	if err != nil {
		return Config{}, &ConfigError{Err: ErrConfigMissing, Code: "CONFIG_MISSING", Detail: map[string]string{"var": "OIDC_AUDIENCE"}}
	}

	voltaSMSecretID, err := getRequired("VOLTA_SM_SECRET_ID")
	if err != nil {
		return Config{}, &ConfigError{Err: ErrConfigMissing, Code: "CONFIG_MISSING", Detail: map[string]string{"var": "VOLTA_SM_SECRET_ID"}}
	}

	voltaS3Bucket, err := getRequired("VOLTA_S3_BUCKET")
	if err != nil {
		return Config{}, &ConfigError{Err: ErrConfigMissing, Code: "CONFIG_MISSING", Detail: map[string]string{"var": "VOLTA_S3_BUCKET"}}
	}

	httpPortStr := os.Getenv("HTTP_PORT")
	if httpPortStr == "" {
		httpPortStr = "8080"
	}
	httpPort, err := strconv.Atoi(httpPortStr)
	if err != nil {
		return Config{}, &ConfigError{Err: ErrConfigInvalid, Code: "CONFIG_INVALID", Detail: map[string]string{"var": "HTTP_PORT"}}
	}

	// Load Volta environment type (defaults to prod for security)
	voltaEnv := VoltaEnvType(os.Getenv("VOLTA_ENV"))
	if voltaEnv == "" {
		voltaEnv = VoltaEnvProd // Default to production for security
	}

	// Load Volta storage type (defaults to s3)
	voltaStorage := VoltaStorageType(os.Getenv("VOLTA_STORAGE"))
	if voltaStorage == "" {
		voltaStorage = VoltaStorageS3 // Default to S3 for durability
	}

	// Load optional Volta file path (for file storage mode)
	voltaFilePath := os.Getenv("VOLTA_FILE_PATH")

	// Load optional Volta passphrase environment variable name
	voltaPassphraseEnvVar := os.Getenv("VOLTA_PASSPHRASE_ENV_VAR")
	if voltaPassphraseEnvVar == "" {
		voltaPassphraseEnvVar = "VOLTA_MASTER_PASSPHRASE" // #nosec G101 - This is a configuration key (env var name), not an actual secret value
	}

	return Config{
		HTTPPort:              httpPort,
		DatabaseURL:           dbURL,
		OIDCIssuerURL:         oidcIssuer,
		OIDCAudience:          oidcAudience,
		VoltaEnvType:          voltaEnv,
		VoltaStorage:          voltaStorage,
		VoltaSMSecretID:       voltaSMSecretID,
		VoltaS3Bucket:         voltaS3Bucket,
		VoltaFilePath:         voltaFilePath,
		VoltaPassphraseEnvVar: voltaPassphraseEnvVar,
	}, nil
}
