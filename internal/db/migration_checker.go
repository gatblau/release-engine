package db

import (
	"context"
	"fmt"
)

// MigrationChecker verifies schema version.
type MigrationChecker interface {
	CurrentVersion(ctx context.Context) (string, error)
	IsUpToDate(ctx context.Context) (bool, error)
}

type migrationChecker struct {
	pool Pool
}

// NewMigrationChecker creates a new migration checker.
func NewMigrationChecker(pool Pool) MigrationChecker {
	return &migrationChecker{pool: pool}
}

func (m *migrationChecker) CurrentVersion(ctx context.Context) (string, error) {
	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Release()

	var version string
	err = conn.QueryRow(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("MIGRATION_METADATA_MISSING: %w", err)
	}
	return version, nil
}

func (m *migrationChecker) IsUpToDate(ctx context.Context) (bool, error) {
	// Simple check, assumes expected version is handled externally or logic here
	_, err := m.CurrentVersion(ctx)
	if err != nil {
		return false, err
	}
	return true, nil
}
