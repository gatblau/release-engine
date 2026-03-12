// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Pool manages the DB connection pool.
type Pool interface {
	Acquire(ctx context.Context) (Conn, error)
	Ping(ctx context.Context) error
	Close()
}

type Conn interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Release()
}

type pool struct {
	db *pgxpool.Pool
}

// NewPool creates a new connection pool.
func NewPool(dbURL string) (Pool, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("DB_INVALID_URL: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 5

	db, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, &DBError{Err: ErrDBUnavailable, Code: "DB_UNAVAILABLE", Detail: map[string]string{"error": err.Error()}}
	}

	return &pool{db: db}, nil
}

func (p *pool) Acquire(ctx context.Context) (Conn, error) {
	c, err := p.db.Acquire(ctx)
	return c, err
}

func (p *pool) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.db.Ping(ctx)
}

func (p *pool) Close() {
	p.db.Close()
}
