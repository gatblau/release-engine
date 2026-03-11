package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gatblau/release-engine/internal/config"
	"github.com/gatblau/release-engine/internal/db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: release-engine <command> [args]\ncommands: db create, db ping, db init")
	}

	switch os.Args[1] {
	case "db":
		return handleDBCommand()
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

func loadConfig() (string, error) {
	ctx := context.Background()
	loader := config.NewLoader()
	cfg, err := loader.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load configuration: %w", err)
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
	dbURL, err := loadConfig()
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
	dbURL, err := loadConfig()
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
