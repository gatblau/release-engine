package secrets

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	appconfig "github.com/gatblau/release-engine/internal/config"
	"github.com/gatblau/volta"
	"github.com/gatblau/volta/persist"
	"go.uber.org/zap"
)

// mockVaultManagerFactory implements VaultManagerFactory for testing
type mockVaultManagerFactory struct {
	vm  volta.VaultManagerService
	err error
}

func (f *mockVaultManagerFactory) Create(options volta.Options, s3Config persist.S3Config, auditLogger interface{}) (volta.VaultManagerService, error) {
	return f.vm, f.err
}

func (f *mockVaultManagerFactory) CreateFileStore(options volta.Options, basePath string, auditLogger interface{}) volta.VaultManagerService {
	return f.vm
}

// mockSecretsManagerClient implements SecretsManagerClient for testing
type mockSecretsManagerClient struct {
	secretValue string
	err         error
}

func (c *mockSecretsManagerClient) GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &secretsmanager.GetSecretValueOutput{
		SecretString: &c.secretValue,
	}, nil
}

// Ensure mockSecretsManagerClient implements SecretsManagerClient
var _ SecretsManagerClient = (*mockSecretsManagerClient)(nil)

// Ensure mockVaultManagerFactory implements VaultManagerFactory
var _ VaultManagerFactory = (*mockVaultManagerFactory)(nil)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	// Save original env var and restore after test
	originalEnv := os.Getenv("VOLTA_MASTER_PASSPHRASE")
	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	defer func() { _ = os.Setenv("VOLTA_MASTER_PASSPHRASE", originalEnv) }()

	manager, err := NewManager(logger, cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if manager.logger != logger {
		t.Error("Logger should be set")
	}

	if manager.cfg != cfg {
		t.Error("Config should be set")
	}

	if manager.vaults == nil {
		t.Error("Vaults map should be initialized")
	}

	if manager.vm == nil {
		t.Error("Vault manager should be initialized")
	}
}

func TestNewManagerWithDeps(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	// Create mock factory
	factory := &mockVaultManagerFactory{
		vm: nil, // Will be set when Create is called
	}

	// Create mock secrets manager
	smClient := &mockSecretsManagerClient{
		secretValue: "test-secret",
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()

	manager, err := NewManagerWithDeps(logger, cfg, factory, smClient)
	if err != nil {
		t.Fatalf("NewManagerWithDeps failed: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if manager.vaultManagerFactory != factory {
		t.Error("Factory should be set")
	}

	if manager.secretsManager != smClient {
		t.Error("Secrets manager should be set")
	}
}

func TestNewManagerWithDeps_FactoryError(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	// Create mock factory that returns error
	factory := &mockVaultManagerFactory{
		vm:  nil,
		err: errors.New("factory error"),
	}

	smClient := &mockSecretsManagerClient{
		secretValue: "test-secret",
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()

	_, err := NewManagerWithDeps(logger, cfg, factory, smClient)
	if err == nil {
		t.Fatal("Expected error from factory")
	}
}

func TestNewManager_WithSSL(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	// Set env vars
	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	_ = os.Setenv("AWS_USE_SSL", "true")
	_ = os.Setenv("AWS_REGION", "us-west-2")
	_ = os.Setenv("AWS_ENDPOINT", "s3.amazonaws.com")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()
	defer func() { _ = os.Unsetenv("AWS_USE_SSL") }()
	defer func() { _ = os.Unsetenv("AWS_REGION") }()
	defer func() { _ = os.Unsetenv("AWS_ENDPOINT") }()

	manager, err := NewManager(logger, cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestNewManager_DefaultRegion(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	// Set env vars (no region)
	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	_ = os.Unsetenv("AWS_REGION")
	defer func() { _ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "") }()

	manager, err := NewManager(logger, cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestNewManager_WithCustomS3Config(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "my-custom-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	_ = os.Setenv("AWS_REGION", "eu-west-1")
	_ = os.Setenv("AWS_ENDPOINT", "s3.eu-west-1.amazonaws.com")
	_ = os.Setenv("AWS_USE_SSL", "true")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()
	defer func() { _ = os.Unsetenv("AWS_REGION") }()
	defer func() { _ = os.Unsetenv("AWS_ENDPOINT") }()
	defer func() { _ = os.Unsetenv("AWS_USE_SSL") }()

	manager, err := NewManager(logger, cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestManager_Init_WithEnvPassphrase(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
	}

	factory := &mockVaultManagerFactory{}
	smClient := &mockSecretsManagerClient{}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()

	manager := &Manager{
		logger:              logger,
		cfg:                 cfg,
		vaults:              make(map[string]volta.VaultService),
		vm:                  nil,
		secretsManager:      smClient,
		vaultManagerFactory: factory,
	}

	err := manager.Init(context.Background())
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}

func TestManager_Init_WithSecretsManager(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket:   "test-bucket",
		VoltaSMSecretID: "arn:aws:secretsmanager:us-east-1:123456789012:secret:volta-master",
	}

	factory := &mockVaultManagerFactory{}
	smClient := &mockSecretsManagerClient{
		secretValue: "my-master-passphrase",
	}

	// Make sure env passphrase is NOT set
	_ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE")

	manager := &Manager{
		logger:              logger,
		cfg:                 cfg,
		vaults:              make(map[string]volta.VaultService),
		vm:                  nil,
		secretsManager:      smClient,
		vaultManagerFactory: factory,
	}

	err := manager.Init(context.Background())
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}

func TestManager_Init_SecretsManagerError(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket:   "test-bucket",
		VoltaSMSecretID: "arn:aws:secretsmanager:us-east-1:123456789012:secret:volta-master",
	}

	factory := &mockVaultManagerFactory{}
	smClient := &mockSecretsManagerClient{
		err: errors.New("secrets manager error"),
	}

	// Make sure env passphrase is NOT set
	_ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE")

	manager := &Manager{
		logger:              logger,
		cfg:                 cfg,
		vaults:              make(map[string]volta.VaultService),
		vm:                  nil,
		secretsManager:      smClient,
		vaultManagerFactory: factory,
	}

	err := manager.Init(context.Background())
	if err == nil {
		t.Fatal("Expected error from Secrets Manager")
	}
}

func TestManager_Init_EmptySecret(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket:   "test-bucket",
		VoltaSMSecretID: "arn:aws:secretsmanager:us-east-1:123456789012:secret:volta-master",
	}

	factory := &mockVaultManagerFactory{}
	smClient := &mockSecretsManagerClient{
		secretValue: "", // Empty secret - will be treated as no passphrase
	}

	// Make sure env passphrase is NOT set
	_ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE")

	manager := &Manager{
		logger:              logger,
		cfg:                 cfg,
		vaults:              make(map[string]volta.VaultService),
		vm:                  nil,
		secretsManager:      smClient,
		vaultManagerFactory: factory,
	}

	// Empty secret is acceptable - the vault just won't have a passphrase set
	// This is not an error condition, just a warning
	err := manager.Init(context.Background())
	if err != nil {
		t.Fatalf("Init should not fail for empty secret: %v", err)
	}
}

func TestManager_CloseAll(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()

	manager := &Manager{
		logger:              logger,
		cfg:                 cfg,
		vaults:              make(map[string]volta.VaultService),
		vm:                  nil,
		secretsManager:      nil,
		vaultManagerFactory: nil,
	}

	err := manager.CloseAll(context.Background())
	if err != nil {
		t.Fatalf("CloseAll failed: %v", err)
	}
}

func TestManager_GetVaultManager(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()

	manager, err := NewManager(logger, cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	vm := manager.GetVaultManager()
	if vm == nil {
		t.Error("GetVaultManager should return non-nil")
	}
}

func TestConfigValidation(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name       string
		bucketName string
		wantErr    bool
	}{
		{
			name:       "valid bucket name",
			bucketName: "my-vault-bucket",
			wantErr:    false,
		},
		{
			name:       "empty bucket name",
			bucketName: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
			defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()

			cfg := &appconfig.Config{
				VoltaS3Bucket: tt.bucketName,
				VoltaStorage:  appconfig.VoltaStorageS3,
			}

			_, err := NewManager(logger, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewManager_WithKeyPrefix(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	_ = os.Setenv("AWS_REGION", "us-east-1")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()
	defer func() { _ = os.Unsetenv("AWS_REGION") }()

	manager, err := NewManager(logger, cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestNewManager_WithAccessKey(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
		VoltaStorage:  appconfig.VoltaStorageS3,
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	_ = os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	_ = os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()
	defer func() { _ = os.Unsetenv("AWS_ACCESS_KEY_ID") }()
	defer func() { _ = os.Unsetenv("AWS_SECRET_ACCESS_KEY") }()

	manager, err := NewManager(logger, cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestManager_Init_MultipleTenants(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket:   "test-bucket",
		VoltaSMSecretID: "arn:aws:secretsmanager:us-east-1:123456789012:secret:volta-master",
	}

	factory := &mockVaultManagerFactory{}
	smClient := &mockSecretsManagerClient{
		secretValue: "my-master-passphrase",
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()

	manager := &Manager{
		logger:              logger,
		cfg:                 cfg,
		vaults:              make(map[string]volta.VaultService),
		vm:                  nil,
		secretsManager:      smClient,
		vaultManagerFactory: factory,
	}

	// Call Init multiple times - should be idempotent
	err := manager.Init(context.Background())
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = manager.Init(context.Background())
	if err != nil {
		t.Fatalf("Init second call failed: %v", err)
	}
}

func TestManager_CloseAll_MultipleTimes(t *testing.T) {
	logger := zap.NewNop()
	cfg := &appconfig.Config{
		VoltaS3Bucket: "test-bucket",
	}

	_ = os.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase")
	defer func() { _ = os.Unsetenv("VOLTA_MASTER_PASSPHRASE") }()

	manager := &Manager{
		logger:              logger,
		cfg:                 cfg,
		vaults:              make(map[string]volta.VaultService),
		vm:                  nil,
		secretsManager:      nil,
		vaultManagerFactory: nil,
	}

	// Call CloseAll multiple times - should be idempotent
	err := manager.CloseAll(context.Background())
	if err != nil {
		t.Fatalf("CloseAll failed: %v", err)
	}

	err = manager.CloseAll(context.Background())
	if err != nil {
		t.Fatalf("CloseAll second call failed: %v", err)
	}
}
