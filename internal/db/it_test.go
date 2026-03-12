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

func TestSchemaDefinesScheduleColumn(t *testing.T) {
	ctx := context.Background()
	connStr := SetupTestPostgres(ctx, t)

	pool, err := NewPool(connStr)
	assert.NoError(t, err)
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	assert.NoError(t, err)
	defer conn.Release()

	for _, schema := range SchemaAll {
		_, err := conn.Exec(ctx, schema)
		assert.NoError(t, err)
	}

	checkColumn := func(table string) {
		var exists bool
		row := conn.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public'
			AND table_name = $1
			AND column_name = 'schedule'
		)`, table)
		assert.NoError(t, row.Scan(&exists))
		assert.True(t, exists, "table %s should have schedule column", table)
	}

	checkColumn("jobs")
	checkColumn("jobs_read")
}
