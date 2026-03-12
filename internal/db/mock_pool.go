// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package db

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/mock"
)

// MockPool is a mock implementation of db.Pool
type MockPool struct {
	mock.Mock
}

func (m *MockPool) Acquire(ctx context.Context) (Conn, error) {
	args := m.Called(ctx)
	return args.Get(0).(Conn), args.Error(1)
}

func (m *MockPool) Ping(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockPool) Close() {
	m.Called()
}

// MockConn is a mock implementation of db.Conn
type MockConn struct {
	mock.Mock
}

func (m *MockConn) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	calledArgs := m.Called(ctx, sql, args)
	if calledArgs.Get(0) == nil {
		return nil, calledArgs.Error(1)
	}
	return calledArgs.Get(0).(pgx.Rows), calledArgs.Error(1)
}

func (m *MockConn) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return m.Called(ctx, sql, args).Get(0).(pgx.Row)
}

func (m *MockConn) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockConn) Release() {
	m.Called()
}
