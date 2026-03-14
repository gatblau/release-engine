// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package outbox

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// CallbackSigner handles HMAC signing of callback payloads.
type CallbackSigner interface {
	Sign(payload []byte) (signature string, keyID string, err error)
	Verify(payload []byte, signature string, keyID string) bool
}

type callbackSigner struct {
	activeKey []byte
	activeID  string
}

// NewCallbackSigner creates a new CallBackSigner.
func NewCallbackSigner(activeKey []byte, activeID string) CallbackSigner {
	return &callbackSigner{activeKey: activeKey, activeID: activeID}
}

func (s *callbackSigner) Sign(payload []byte) (string, string, error) {
	mac := hmac.New(sha256.New, s.activeKey)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil)), s.activeID, nil
}

func (s *callbackSigner) Verify(payload []byte, signature string, keyID string) bool {
	return false // Not implemented
}
