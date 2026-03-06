//go:build integration

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPool_Integration(t *testing.T) {
	ctx := context.Background()
	connStr := SetupTestPostgres(ctx, t)

	pool, err := NewPool(connStr)
	assert.NoError(t, err)
	defer pool.Close()

	err = pool.Ping(ctx)
	assert.NoError(t, err)
}

func TestMigrationChecker_Integration(t *testing.T) {
	ctx := context.Background()
	connStr := SetupTestPostgres(ctx, t)

	pool, err := NewPool(connStr)
	assert.NoError(t, err)
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	_, err = conn.Exec(ctx, "CREATE TABLE schema_migrations (version TEXT)")
	assert.NoError(t, err)
	_, err = conn.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ('1.0.0')")
	assert.NoError(t, err)
	conn.Release()

	checker := NewMigrationChecker(pool)
	v, err := checker.CurrentVersion(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "1.0.0", v)
}
