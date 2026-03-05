package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Pool manages the DB connection pool.
type Pool interface {
	Acquire(ctx context.Context) (*pgxpool.Conn, error)
	Ping(ctx context.Context) error
	Close()
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
		return nil, fmt.Errorf("DB_UNAVAILABLE: %w", err)
	}

	return &pool{db: db}, nil
}

func (p *pool) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	return p.db.Acquire(ctx)
}

func (p *pool) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.db.Ping(ctx)
}

func (p *pool) Close() {
	p.db.Close()
}
