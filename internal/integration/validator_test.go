// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrossplaneValidatorHappyPath(t *testing.T) {
	validator := NewCrossplaneValidator()
	manifest := []byte(`apiVersion: v1
kind: Config
metadata:
  name: test
  labels:
    tenant: demo
spec:
  size: small
`)
	require.NoError(t, validator.ValidateYAML(manifest))
}

func TestCrossplaneValidatorMissingField(t *testing.T) {
	validator := NewCrossplaneValidator()
	manifest := []byte(`apiVersion: v1
kind: Config
metadata:
  name: test
spec:
  size: small
`)
	require.ErrorContains(t, validator.ValidateYAML(manifest), "metadata.labels must be defined")
}

func TestAssertGoldenYAMLMatches(t *testing.T) {
	manifest := []byte(`apiVersion: v1
kind: Config
metadata:
  name: golden
  labels:
    tenant: demo
spec:
  size: epic
`)
	AssertGoldenYAML(t, "example", manifest)
}
