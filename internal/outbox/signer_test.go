// File: internal/outbox/signer_test.go
package outbox

import (
	"testing"
)

func TestCallbackSigner_Sign(t *testing.T) {
	s := NewCallbackSigner([]byte("secret"), "key1")
	sig, id, err := s.Sign([]byte("payload"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if id != "key1" {
		t.Errorf("expected key1, got %v", id)
	}
	if sig == "" {
		t.Errorf("expected signature, got empty")
	}
}
