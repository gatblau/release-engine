package secrets

import (
	"context"
	"fmt"
	"os"
	"sync"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	appconfig "github.com/gatblau/release-engine/internal/config"
	volta "github.com/gatblau/volta/pkg"
	"go.uber.org/zap"
)

// SecretsManagerClient defines the interface for AWS Secrets Manager operations
type SecretsManagerClient interface {
	GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error)
}

// VaultManagerFactory creates VaultManagerService instances
type VaultManagerFactory interface {
	Create(options volta.Options, s3Config volta.S3Config, auditLogger interface{}) (volta.VaultManagerService, error)
	CreateFileStore(options volta.Options, basePath string, auditLogger interface{}) volta.VaultManagerService
}

// defaultVaultManagerFactory creates real vault manager instances
type defaultVaultManagerFactory struct{}

func (f *defaultVaultManagerFactory) Create(options volta.Options, s3Config volta.S3Config, auditLogger interface{}) (volta.VaultManagerService, error) {
	return volta.NewVaultManagerS3Store(options, s3Config, nil)
}

func (f *defaultVaultManagerFactory) CreateFileStore(options volta.Options, basePath string, auditLogger interface{}) volta.VaultManagerService {
	return volta.NewVaultManagerFileStore(options, basePath, nil)
}

// defaultSecretsManagerClient creates real AWS Secrets Manager clients
type defaultSecretsManagerClient struct{}

func (c *defaultSecretsManagerClient) GetSecretValue(ctx context.Context, input *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	svc := secretsmanager.NewFromConfig(awsCfg)
	return svc.GetSecretValue(ctx, input)
}

// VaultService wrapper interface for direct volta usage
type VaultService interface {
	// UseSecret retrieves and decrypts a secret, passing the decrypted data to the callback fn.
	// The callback is called synchronously, and the decrypted data is cleared after the callback returns.
	UseSecret(secretKey string, fn func(data []byte) error) error
	// UseSecrets retrieves and decrypts multiple secrets, passing the decrypted data to the callback fn.
	// The callback is called synchronously, and the decrypted data is cleared after the callback returns.
	UseSecrets(secretKeys []string, fn func(secrets map[string][]byte) error) error
	// Close releases resources associated with the vault
	Close() error
}

type voltaVaultService struct {
	v volta.VaultService
}

func (s *voltaVaultService) UseSecret(secretKey string, fn func(data []byte) error) error {
	return s.v.UseSecret(secretKey, fn)
}

func (s *voltaVaultService) UseSecrets(secretKeys []string, fn func(secrets map[string][]byte) error) error {
	return s.v.UseSecrets(secretKeys, fn)
}

func (s *voltaVaultService) Close() error {
	return s.v.Close()
}

// Manager manages volta lifecycle
type Manager struct {
	logger              *zap.Logger
	cfg                 *appconfig.Config
	mu                  sync.Mutex
	vaults              map[string]volta.VaultService
	vm                  volta.VaultManagerService
	secretsManager      SecretsManagerClient
	vaultManagerFactory VaultManagerFactory
}

// NewManager creates a new Volta Manager.
func NewManager(logger *zap.Logger, cfg *appconfig.Config) (*Manager, error) {
	factory := &defaultVaultManagerFactory{}
	return NewManagerWithDeps(logger, cfg, factory, &defaultSecretsManagerClient{})
}

// NewManagerWithDeps creates a new Volta Manager with injectable dependencies for testing
func NewManagerWithDeps(logger *zap.Logger, cfg *appconfig.Config, factory VaultManagerFactory, smClient SecretsManagerClient) (*Manager, error) {
	var vm volta.VaultManagerService
	var err error

	// Determine storage type and create appropriate vault manager
	switch cfg.VoltaStorage {
	case appconfig.VoltaStorageFile:
		// Development mode: use file-based storage
		// Salt can be provided via env var for development stability
		options := buildVoltaOptions(cfg, true)
		vm = factory.CreateFileStore(options, cfg.VoltaFilePath, nil)

	case appconfig.VoltaStorageS3:
		// Production mode: use S3 storage with automatic salt persistence
		// Salt is NOT provided - volta will handle persistence automatically
		options := buildVoltaOptions(cfg, false)
		s3Config := volta.S3Config{
			AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			Bucket:          cfg.VoltaS3Bucket,
			Region:          os.Getenv("AWS_REGION"),
			Endpoint:        os.Getenv("AWS_ENDPOINT"),
			UseSSL:          os.Getenv("AWS_USE_SSL") == "true",
			KeyPrefix:       "volta/",
		}
		vm, err = factory.Create(options, s3Config, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 vault manager: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported volta storage type: %s", cfg.VoltaStorage)
	}

	return &Manager{
		logger:              logger,
		cfg:                 cfg,
		vaults:              make(map[string]volta.VaultService),
		vm:                  vm,
		secretsManager:      smClient,
		vaultManagerFactory: factory,
	}, nil
}

// buildVoltaOptions constructs volta.Options based on environment configuration
func buildVoltaOptions(cfg *appconfig.Config, includeSalt bool) volta.Options {
	passphraseEnvVar := cfg.VoltaPassphraseEnvVar
	if passphraseEnvVar == "" {
		passphraseEnvVar = "VOLTA_MASTER_PASSPHRASE" // #nosec G101 - This is a configuration key (env var name), not an actual secret value
	}

	options := volta.Options{
		EnvPassphraseVar: passphraseEnvVar,
		EnableMemoryLock: true,
	}

	// If VOLTA_SALT env var is set, use it (useful for pinning a known salt in dev/CI).
	// Otherwise, leave DerivationSalt empty — Volta will auto-generate and persist the salt
	// to the store on first use, exactly as it does for S3 mode. This prevents a mismatch
	// between the randomly-generated in-memory salt and any salt already persisted on disk.
	if includeSalt {
		if saltStr := os.Getenv("VOLTA_SALT"); saltStr != "" {
			options.DerivationSalt = []byte(saltStr)
		}
	}
	// In production (S3) mode, DerivationSalt is NOT set - volta handles persistence automatically

	return options
}

// Init bootstraps the volta manager by fetching the master passphrase from AWS Secrets Manager
// and initialising each tenant vault
func (m *Manager) Init(ctx context.Context) error {
	// Use the configured env var name, or fall back to default if not set
	passphraseEnvVar := m.cfg.VoltaPassphraseEnvVar
	if passphraseEnvVar == "" {
		passphraseEnvVar = "VOLTA_MASTER_PASSPHRASE" // #nosec G101 - This is a configuration key (env var name), not an actual secret value
	}

	// Check if passphrase is provided via environment variable (preferred for production)
	passphrase := os.Getenv(passphraseEnvVar)
	if passphrase == "" {
		// Fallback: fetch from AWS Secrets Manager
		var err error
		passphrase, err = m.fetchPassphrase(ctx)
		if err != nil {
			return fmt.Errorf("volta bootstrap failed: %w", err)
		}
		// Set the passphrase in the environment variable so Volta can access it
		// This is safe because Volta will read it when accessing vaults
		if err := os.Setenv(passphraseEnvVar, passphrase); err != nil {
			return fmt.Errorf("failed to set passphrase env var: %w", err)
		}
	}

	// Get vault for each tenant to initialize them with the passphrase
	// In a multi-tenant setup, we would iterate over tenant IDs
	// For now, we initialize on-demand in GetVault
	m.logger.Info("volta manager initialized successfully")
	return nil
}

func (m *Manager) fetchPassphrase(ctx context.Context) (string, error) {
	// Get secret value
	result, err := m.secretsManager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &m.cfg.VoltaSMSecretID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	// SecretString contains the passphrase
	if result.SecretString == nil {
		return "", fmt.Errorf("secret value is empty")
	}

	return *result.SecretString, nil
}

// GetVault retrieves/creates a vault service for the specified tenant
func (m *Manager) GetVault(ctx context.Context, tenantID string) (VaultService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if v, ok := m.vaults[tenantID]; ok {
		return &voltaVaultService{v: v}, nil
	}

	v, err := m.vm.GetVault(tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant vault unavailable: %w", err)
	}

	m.vaults[tenantID] = v
	return &voltaVaultService{v: v}, nil
}

// CloseAll cleans up all vault resources
func (m *Manager) CloseAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, v := range m.vaults {
		if err := v.Close(); err != nil {
			m.logger.Error("failed to close vault", zap.Error(err))
		}
	}
	m.vaults = make(map[string]volta.VaultService)

	if vm, ok := m.vm.(interface{ CloseAll() error }); ok {
		return vm.CloseAll()
	}
	return nil
}

// GetVaultManager returns the underlying vault manager (for testing)
func (m *Manager) GetVaultManager() volta.VaultManagerService {
	return m.vm
}
