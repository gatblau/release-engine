package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPool(t *testing.T) {
	// Should fail with intentionally bad URL
	pool, err := NewPool("invalid-url")
	assert.Error(t, err)
	assert.Nil(t, pool)
}
