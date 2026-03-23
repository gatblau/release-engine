// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrossplaneValidatorHappyPath(t *testing.T) {
	validator := NewCrossplaneValidator()
	manifest := []byte(`apiVersion: infrastructure.platform.io/v1alpha1
kind: XKubernetesCluster
metadata:
  name: test
  labels:
    platform.io/tenant: demo
spec:
  parameters:
    region: eu-west-1
    version: "1.30"
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
    platform.io/tenant: demo
spec:
  size: epic
`)
	AssertGoldenYAML(t, "example", manifest)
}
