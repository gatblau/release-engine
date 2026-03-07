package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPolicyEngine(t *testing.T) {
	pe := NewPolicyEngine()
	assert.NotNil(t, pe)
}

func TestPolicyEngine_Evaluate(t *testing.T) {
	pe := NewPolicyEngine()

	result := pe.Evaluate("test-request-id")
	assert.True(t, result)
}

func TestPolicyEngine_Evaluate_MultipleIDs(t *testing.T) {
	pe := NewPolicyEngine()

	// Test with different request IDs
	assert.True(t, pe.Evaluate("request-1"))
	assert.True(t, pe.Evaluate("request-2"))
	assert.True(t, pe.Evaluate(""))
}

func TestNewIdempotencyService(t *testing.T) {
	is := NewIdempotencyService()
	assert.NotNil(t, is)
}

func TestIdempotencyService_Proccess(t *testing.T) {
	is := NewIdempotencyService()

	result := is.Proccess("test-transaction-id")
	assert.True(t, result)
}

func TestIdempotencyService_Proccess_MultipleIDs(t *testing.T) {
	is := NewIdempotencyService()

	// Test with different transaction IDs
	assert.True(t, is.Proccess("tx-001"))
	assert.True(t, is.Proccess("tx-002"))
	assert.True(t, is.Proccess(""))
}
