package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gatblau/release-engine/internal/config"
	"github.com/gatblau/release-engine/internal/connector"
	giteaconnector "github.com/gatblau/release-engine/internal/connector/gitea"
	gitconnector "github.com/gatblau/release-engine/internal/connector/github"
	policyconnector "github.com/gatblau/release-engine/internal/connector/policy"
	connectortesting "github.com/gatblau/release-engine/internal/connector/testing"
	webhookconnector "github.com/gatblau/release-engine/internal/connector/webhook"
	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/logger"
	runnerpkg "github.com/gatblau/release-engine/internal/runner"
	"github.com/gatblau/release-engine/internal/scheduler"
	"github.com/gatblau/release-engine/internal/secrets"
	"github.com/gatblau/release-engine/internal/transport/http"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		_, err = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: release-engine <command> [args]\ncommands: db create, db ping, db init, serve")
	}

	switch os.Args[1] {
	case "db":
		return handleDBCommand()
	case "serve":
		return serve()
	default:
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
}

func handleDBCommand() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: release-engine db <subcommand>\nsubcommands: create, ping, init")
	}

	switch os.Args[2] {
	case "create":
		return dbCreate()
	case "ping":
		return dbPing()
	case "init":
		return dbInit()
	default:
		return fmt.Errorf("unknown db subcommand: %s", os.Args[2])
	}
}

func loadConfig() (config.Config, error) {
	ctx := context.Background()
	loader := config.NewLoader()
	return loader.Load(ctx)
}

func loadConfigDBURL() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	return cfg.DatabaseURL, nil
}

func createPool(dbURL string) (db.Pool, error) {
	pool, err := db.NewPool(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}
	return pool, nil
}

// dbCreate creates the database schema
func dbCreate() error {
	dbURL, err := loadConfigDBURL()
	if err != nil {
		return err
	}

	pool, err := createPool(dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	ctx := context.Background()

	// Get a connection from the pool to execute statements
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	// Execute each schema statement
	for _, schema := range db.SchemaAll {
		_, err := conn.Exec(ctx, schema)
		if err != nil {
			return fmt.Errorf("failed to execute schema: %w", err)
		}
	}

	fmt.Println("Database schema created successfully")
	return nil
}

// dbPing checks database connectivity
func dbPing() error {
	dbURL, err := loadConfigDBURL()
	if err != nil {
		return err
	}

	pool, err := createPool(dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	ctx := context.Background()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	fmt.Println("Database connection successful")
	return nil
}

// dbInit combines db create and db ping
func dbInit() error {
	fmt.Println("Creating database schema...")
	if err := dbCreate(); err != nil {
		return err
	}

	fmt.Println("Verifying database connection...")
	if err := dbPing(); err != nil {
		return err
	}

	fmt.Println("Database initialized successfully")
	return nil
}

// serve starts the HTTP server
func serve() error {
	ctx := context.Background()

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create logger
	logFactory, err := logger.NewFactory("debug", "console")
	if err != nil {
		return fmt.Errorf("failed to create logger factory: %w", err)
	}
	log := logFactory.New("release-engine")

	// Create database pool
	pool, err := createPool(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to create database pool: %w", err)
	}
	defer pool.Close()

	// Ensure database schema exists before starting the server.
	if err := dbCreate(); err != nil {
		return fmt.Errorf("failed to initialize database schema: %w", err)
	}

	// Create typed connector registry and register runtime connector implementations
	typedRegistry := connector.NewTypedConnectorRegistry()
	defer func(typedRegistry connector.TypedConnectorRegistry) {
		err = typedRegistry.Close()
		if err != nil {
			fmt.Printf("failed to close connector registry: %v\n", err)
		}
	}(typedRegistry)
	if err := registerRuntimeConnectors(typedRegistry); err != nil {
		return fmt.Errorf("failed to register runtime connectors: %w", err)
	}

	// Create family registry and register all families with their valid members
	familyRegistry := connector.NewFamilyRegistry(typedRegistry)
	for _, family := range connector.DefaultFamilies() {
		if err = familyRegistry.RegisterFamily(family); err != nil {
			return fmt.Errorf("failed to register connector family %s: %w", family.Name, err)
		}
	}
	// Note: Connector implementations are resolved per-module at assembly time
	// via FamilyRegistry.Resolve(family, implKey). No global binding needed.

	// Bootstrap the module registry from config-managed modules (passes familyRegistry for per-module resolution)
	moduleRegistry, err := runnerpkg.BootstrapWithConfig(ctx, typedRegistry, familyRegistry)
	if err != nil {
		return fmt.Errorf("failed to bootstrap module registry: %w", err)
	}

	// Create Volta manager for secrets management (Phase 3)
	voltaManager, err := secrets.NewManager(log, &cfg)
	if err != nil {
		return fmt.Errorf("failed to create Volta manager: %w", err)
	}

	// Initialize Volta manager (fetches master passphrase)
	if err := voltaManager.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize Volta manager: %w", err)
	}
	defer func(voltaManager *secrets.Manager, ctx context.Context) {
		err = voltaManager.CloseAll(ctx)
		if err != nil {
			fmt.Printf("failed to close Volta manager: %v", err)
		}
	}(voltaManager, ctx)

	runnerSvc := runnerpkg.NewRunnerService(pool, familyRegistry, voltaManager, nil, moduleRegistry)
	leaseMgr := scheduler.NewLeaseManager(pool)
	schedulerSvc := scheduler.NewSchedulerService(pool, moduleRegistry, leaseMgr, 25*time.Millisecond, runnerSvc)

	// Create HTTP server with secrets support (Phase 3)
	srv := http.NewServerWithSecrets(cfg.HTTPPort, log, pool, voltaManager, moduleRegistry, cfg.OIDCIssuerURL, cfg.OIDCAudience)
	srv.RegisterRoutes()

	// Setup graceful shutdown
	serverCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		if err := schedulerSvc.Start(serverCtx); err != nil {
			log.Error("Scheduler error", zap.Error(err))
		}
	}()
	go func() {
		log.Info("Starting HTTP server", zap.Int("port", cfg.HTTPPort), zap.String("oidc_issuer", cfg.OIDCIssuerURL))
		serverErrCh <- srv.Start(serverCtx)
	}()

	// Wait for shutdown signal or server error
	select {
	case <-serverCtx.Done():
		log.Info("Shutdown signal received")
	case err := <-serverErrCh:
		if err != nil {
			log.Error("Server error", zap.Error(err))
			return err
		}
	}

	// Graceful shutdown
	log.Info("Shutting down HTTP server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Failed to shutdown server gracefully", zap.Error(err))
		return err
	}

	log.Info("Server shutdown complete")
	return nil
}

func registerRuntimeConnectors(reg connector.TypedConnectorRegistry) error {
	// Register GitHub connector
	cfg := connector.DefaultConnectorConfig()
	cfg.Extra = map[string]string{"base_url": os.Getenv("GIT_CONNECTOR_BASE_URL")}
	gitConn, err := gitconnector.NewGitHubConnector(cfg)
	if err != nil {
		return err
	}
	if err = reg.Register(gitConn); err != nil {
		return err
	}

	// Register Gitea connector
	giteaCfg := connector.DefaultConnectorConfig()
	giteaCfg.Extra = map[string]string{"base_url": os.Getenv("GITEA_CONNECTOR_BASE_URL")}
	giteaConn, err := giteaconnector.NewGiteaConnector(giteaCfg)
	if err != nil {
		return err
	}
	if err = reg.Register(giteaConn); err != nil {
		return err
	}

	crossplaneConn, err := connectortesting.NewCrossplaneMockConnector()
	if err != nil {
		return err
	}
	if err = reg.Register(crossplaneConn); err != nil {
		return err
	}

	policyConn, err := policyconnector.NewPolicyConnector(connector.DefaultConnectorConfig())
	if err != nil {
		return err
	}
	if err = reg.Register(policyConn); err != nil {
		return err
	}

	webhookConn, err := webhookconnector.NewWebhookConnector(connector.DefaultConnectorConfig())
	if err != nil {
		return err
	}
	if err = reg.Register(webhookConn); err != nil {
		return err
	}

	return nil
}
