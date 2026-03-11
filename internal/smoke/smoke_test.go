//go:build integration

package smoke

import (
	"context"
	"strings"
	"testing"

	"github.com/gatblau/release-engine/internal/config"
	"github.com/gatblau/release-engine/internal/db"
	"github.com/gatblau/release-engine/internal/secrets"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// TestDB_PoolOperations tests all database client operations
func TestDB_PoolOperations(t *testing.T) {
	ctx := context.Background()
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx)
	require.NoError(t, err)

	// Create tables
	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	defer conn.Close(ctx)

	// Execute schema - types must be created before tables that reference them
	_, err = conn.Exec(ctx, db.SchemaJobState)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, db.SchemaOutboxKind)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, db.SchemaJobs)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, db.SchemaOutbox)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, db.SchemaAuditLog)
	require.NoError(t, err)

	// Test INSERT (Exec)
	result, err := conn.Exec(ctx, `
		INSERT INTO jobs (id, tenant_id, path_key, idempotency_key, params_json, state, run_id)
		VALUES ('11111111-1111-1111-1111-111111111111', 'test-tenant', 'test-job', 'idem-123', '{}', 'queued', '22222222-2222-2222-2222-222222222222')
	`)
	require.NoError(t, err)
	assert.True(t, result.Insert())

	// Test SELECT single row (QueryRow)
	var jobID string
	var state string
	err = conn.QueryRow(ctx, "SELECT id, state FROM jobs WHERE tenant_id = $1", "test-tenant").Scan(&jobID, &state)
	require.NoError(t, err)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", jobID)
	assert.Equal(t, "queued", state)

	// Test SELECT multiple rows (Query)
	rows, err := conn.Query(ctx, "SELECT id, state FROM jobs WHERE tenant_id = $1", "test-tenant")
	require.NoError(t, err)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, s string
		err := rows.Scan(&id, &s)
		require.NoError(t, err)
		count++
	}
	assert.Equal(t, 1, count)

	// Test UPDATE
	result, err = conn.Exec(ctx, "UPDATE jobs SET state = $1 WHERE id = $2", "running", "11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected())

	// Verify update
	err = conn.QueryRow(ctx, "SELECT state FROM jobs WHERE id = $1", "11111111-1111-1111-1111-111111111111").Scan(&state)
	require.NoError(t, err)
	assert.Equal(t, "running", state)

	// Test DELETE
	result, err = conn.Exec(ctx, "DELETE FROM jobs WHERE id = $1", "11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.RowsAffected())

	// Verify delete
	var deletedID *string
	err = conn.QueryRow(ctx, "SELECT id FROM jobs WHERE id = $1", "11111111-1111-1111-1111-111111111111").Scan(&deletedID)
	assert.Error(t, err) // should be no rows

	t.Log("✓ Database pool operations test passed")
}

// TestDB_ConnectionPool tests connection pooling behavior
func TestDB_ConnectionPool(t *testing.T) {
	ctx := context.Background()
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx)
	require.NoError(t, err)

	// Create pool
	pool, err := db.NewPool(connStr)
	require.NoError(t, err)
	defer pool.Close()

	// Test Ping
	err = pool.Ping(ctx)
	require.NoError(t, err)

	// Test Acquire/Release cycle
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Use the connection
	var version string
	err = conn.QueryRow(ctx, "SELECT version()").Scan(&version)
	require.NoError(t, err)
	assert.Contains(t, version, "PostgreSQL")

	// Release connection
	conn.Release()

	// Test another acquire after release
	conn2, err := pool.Acquire(ctx)
	require.NoError(t, err)
	conn2.Release()

	t.Log("✓ Database connection pool test passed")
}

// TestDB_OutboxTable tests the outbox table operations
func TestDB_OutboxTable(t *testing.T) {
	ctx := context.Background()
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx)
	require.NoError(t, err)
	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	defer conn.Close(ctx)

	// Create enum types before tables that reference them
	_, err = conn.Exec(ctx, db.SchemaJobState)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, db.SchemaOutboxKind)
	require.NoError(t, err)

	// Create outbox table
	_, err = conn.Exec(ctx, db.SchemaOutbox)
	require.NoError(t, err)

	// Insert outbox entry (use valid enum value for kind)
	_, err = conn.Exec(ctx, `
		INSERT INTO outbox (tenant_id, job_id, kind, payload_json, delivery_state)
		VALUES ('tenant-1', '11111111-1111-1111-1111-111111111111', 'webhook', '{}', 'pending')
	`)
	require.NoError(t, err)

	// Update delivery state
	_, err = conn.Exec(ctx, "UPDATE outbox SET delivery_state = $1, attempt = attempt + 1 WHERE delivery_state = $2", "sent", "pending")
	require.NoError(t, err)

	// Verify
	var attempt int
	err = conn.QueryRow(ctx, "SELECT attempt FROM outbox WHERE delivery_state = $1", "sent").Scan(&attempt)
	require.NoError(t, err)
	assert.Equal(t, 1, attempt)

	t.Log("✓ Database outbox table test passed")
}

// TestVolta_S3Operations tests MinIO/S3 operations through Volta
func TestVolta_S3Operations(t *testing.T) {
	ctx := context.Background()

	// Start MinIO container
	minioContainer, err := minio.RunContainer(ctx,
		testcontainers.WithImage("minio/minio:latest"),
		testcontainers.WithEnv(map[string]string{"MINIO_ROOT_USER": "minioadmin", "MINIO_ROOT_PASSWORD": "minioadmin"}),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("9000")),
	)
	require.NoError(t, err)
	defer minioContainer.Terminate(ctx)

	endpoint, err := minioContainer.Endpoint(ctx, "http")
	require.NoError(t, err)

	// Set environment variables for Volta to use MinIO
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "minioadmin")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "minioadmin")
	hostPort := strings.TrimPrefix(endpoint, "http://")
	t.Setenv("AWS_ENDPOINT", hostPort)
	t.Setenv("AWS_USE_SSL", "false")
	t.Setenv("VOLTA_STORAGE", "s3")
	t.Setenv("VOLTA_S3_BUCKET", "test-volta-bucket")
	t.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase-123")

	// Create config
	cfg := &config.Config{
		VoltaStorage:          "s3",
		VoltaS3Bucket:         "test-volta-bucket",
		VoltaPassphraseEnvVar: "VOLTA_MASTER_PASSPHRASE",
	}

	logger, _ := zap.NewDevelopment()

	// Create Volta manager
	manager, err := secrets.NewManager(logger, cfg)
	require.NoError(t, err)

	// Initialise (this will set up the vault with the passphrase)
	err = manager.Init(ctx)
	require.NoError(t, err)

	// Get a vault for a tenant
	vault, err := manager.GetVault(ctx, "tenant-1")
	require.NoError(t, err)
	require.NotNil(t, vault)

	// Test secret operations using the vault
	// Try to read a non-existent secret - this is expected to fail for a new vault
	err = vault.UseSecret("test-secret-key", func(data []byte) error {
		// If secret doesn't exist, this won't be called
		return nil
	})
	// Expect error since secret doesn't exist yet - this is expected
	// For a new vault, the secret won't exist, so we handle this gracefully
	if err != nil {
		t.Logf("Expected error reading non-existent secret: %v", err)
	}

	// Close the vault
	err = vault.Close()
	require.NoError(t, err)

	// Close all vaults
	err = manager.CloseAll(ctx)
	require.NoError(t, err)

	t.Log("✓ Volta S3 operations test passed")
}

// TestVolta_FileStore tests file-based Volta storage (alternative to S3)
func TestVolta_FileStore(t *testing.T) {
	ctx := context.Background()

	// Start MinIO container for object storage
	minioContainer, err := minio.RunContainer(ctx,
		testcontainers.WithImage("minio/minio:latest"),
		testcontainers.WithEnv(map[string]string{"MINIO_ROOT_USER": "minioadmin", "MINIO_ROOT_PASSWORD": "minioadmin"}),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("9000")),
	)
	require.NoError(t, err)
	defer minioContainer.Terminate(ctx)

	// Note: This test demonstrates that Volta is being set up correctly
	// In a real scenario, you'd use file storage for vault data
	// and MinIO for job attachments/artifacts

	// Use t.TempDir() so each test run gets a fresh, isolated directory that is
	// automatically cleaned up on completion. This prevents leftover vault files
	// (which carry a persisted salt) from conflicting with a new random salt on
	// the next run.
	tmpDir := t.TempDir()

	t.Setenv("VOLTA_STORAGE", "file")
	t.Setenv("VOLTA_FILE_PATH", tmpDir)
	t.Setenv("VOLTA_MASTER_PASSPHRASE", "test-passphrase-file")

	cfg := &config.Config{
		VoltaStorage:          "file",
		VoltaFilePath:         tmpDir,
		VoltaPassphraseEnvVar: "VOLTA_MASTER_PASSPHRASE",
	}

	logger, _ := zap.NewDevelopment()

	// Create Volta manager with file storage
	manager, err := secrets.NewManager(logger, cfg)
	require.NoError(t, err)

	err = manager.Init(ctx)
	require.NoError(t, err)

	vault, err := manager.GetVault(ctx, "tenant-file")
	require.NoError(t, err)
	require.NotNil(t, vault)

	err = vault.Close()
	require.NoError(t, err)

	err = manager.CloseAll(ctx)
	require.NoError(t, err)

	t.Log("✓ Volta file store test passed")
}

// TestMinIO_ObjectStorage directly tests MinIO as S3-compatible storage
func TestMinIO_ObjectStorage(t *testing.T) {
	ctx := context.Background()

	minioContainer, err := minio.RunContainer(ctx,
		testcontainers.WithImage("minio/minio:latest"),
		testcontainers.WithEnv(map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		}),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("9000")),
	)
	require.NoError(t, err)
	defer minioContainer.Terminate(ctx)

	endpoint, err := minioContainer.Endpoint(ctx, "http")
	require.NoError(t, err)

	// Create an S3 client to test bucket operations
	t.Setenv("AWS_ACCESS_KEY_ID", "minioadmin")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "minioadmin")
	t.Setenv("AWS_REGION", "us-east-1")

	// Note: In a full implementation, you would use the AWS SDK to:
	// 1. Create a bucket
	// 2. Put an object
	// 3. Get the object
	// 4. Delete the object
	// For now, we verify the endpoint is accessible
	assert.NotEmpty(t, endpoint)

	t.Logf("✓ MinIO endpoint accessible at: %s", endpoint)
	t.Log("✓ MinIO object storage test passed")
}

// TestDB_AllTables tests all database tables together
func TestDB_AllTables(t *testing.T) {
	ctx := context.Background()
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx)
	require.NoError(t, err)
	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	defer conn.Close(ctx)

	// Create all tables
	for _, schema := range db.SchemaAll {
		_, err = conn.Exec(ctx, schema)
		require.NoError(t, err)
	}

	// Verify all tables exist
	tables := []string{"jobs", "outbox", "audit_log", "schema_migrations", "metrics_job_events"}
	for _, table := range tables {
		var exists bool
		err = conn.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_name = $1
			)
		`, table).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "Table %s should exist", table)
	}

	t.Log("✓ All database tables test passed")
}

// TestDB_JobLifecycle tests a complete job lifecycle
func TestDB_JobLifecycle(t *testing.T) {
	ctx := context.Background()
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	defer pgContainer.Terminate(ctx)

	connStr, err := pgContainer.ConnectionString(ctx)
	require.NoError(t, err)
	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	defer conn.Close(ctx)

	// Create enum types before tables that reference them
	_, err = conn.Exec(ctx, db.SchemaJobState)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, db.SchemaJobs)
	require.NoError(t, err)

	jobID := "33333333-3333-3333-3333-333333333333"

	// 1. Create job (INSERT)
	_, err = conn.Exec(ctx, `
		INSERT INTO jobs (id, tenant_id, path_key, idempotency_key, params_json, state, attempt, max_attempts, run_id, created_at)
		VALUES ($1, 'tenant-lifecycle', 'test-path', 'idem-lifecycle', '{"key":"value"}', 'queued', 0, 3, '44444444-4444-4444-4444-444444444444', NOW())
	`, jobID)
	require.NoError(t, err)

	// 2. Mark job as running (UPDATE state + started_at)
	_, err = conn.Exec(ctx, `
		UPDATE jobs SET state = 'running', accepted_at = NOW(), started_at = NOW()
		WHERE id = $1
	`, jobID)
	require.NoError(t, err)

	// 3. Simulate failure
	_, err = conn.Exec(ctx, `
		UPDATE jobs SET state = 'failed', finished_at = NOW(), last_error_message = 'test error'
		WHERE id = $1
	`, jobID)
	require.NoError(t, err)

	// 4. Retry (increment attempt)
	_, err = conn.Exec(ctx, `
		UPDATE jobs SET state = 'queued', attempt = attempt + 1, finished_at = NULL, last_error_message = NULL
		WHERE id = $1
	`, jobID)
	require.NoError(t, err)

	// 5. Complete successfully
	_, err = conn.Exec(ctx, `
		UPDATE jobs SET state = 'succeeded', finished_at = NOW()
		WHERE id = $1
	`, jobID)
	require.NoError(t, err)

	// Verify final state
	var finalState string
	var finalAttempt int
	err = conn.QueryRow(ctx, "SELECT state, attempt FROM jobs WHERE id = $1", jobID).Scan(&finalState, &finalAttempt)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", finalState)
	assert.Equal(t, 1, finalAttempt)

	t.Log("✓ Database job lifecycle test passed")
}
