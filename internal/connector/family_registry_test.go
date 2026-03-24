// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"context"
	"strings"
	"testing"
)

type testConnector struct {
	BaseConnector
	operations []string
}

func (c *testConnector) Validate(operation string, input map[string]interface{}) error { return nil }
func (c *testConnector) Execute(ctx context.Context, operation string, input map[string]interface{}, secrets map[string][]byte) (*ConnectorResult, error) {
	return &ConnectorResult{Status: StatusSuccess}, nil
}
func (c *testConnector) Close() error { return nil }
func (c *testConnector) Operations() []OperationMeta {
	var metas []OperationMeta
	for _, op := range c.operations {
		metas = append(metas, OperationMeta{Name: op})
	}
	return metas
}

func TestFamilyRegistry(t *testing.T) {
	concreteReg := NewConnectorRegistry()
	familyReg := NewFamilyRegistry(concreteReg)

	// Create test connectors
	baseGit, _ := NewBaseConnector(ConnectorTypeGit, "github")
	gitConn := &testConnector{BaseConnector: baseGit, operations: []string{"commit_files", "read_file"}}

	basePolicy, _ := NewBaseConnector(ConnectorTypeOther, "policy")
	policyConn := &testConnector{BaseConnector: basePolicy, operations: []string{"evaluate"}}

	// Register connectors
	if err := concreteReg.Register(gitConn); err != nil {
		t.Fatalf("failed to register git connector: %v", err)
	}
	if err := concreteReg.Register(policyConn); err != nil {
		t.Fatalf("failed to register policy connector: %v", err)
	}

	// Register families
	gitFamily := ConnectorFamily{
		Name: "git",
		Operations: map[string]OperationContract{
			"commit_files": {RequiredInputFields: []string{"repo", "branch", "path_prefix", "files", "message", "idempotency_key"}},
			"read_file":    {RequiredInputFields: []string{"repo", "branch", "path"}},
		},
	}

	policyFamily := ConnectorFamily{
		Name: "policy",
		Operations: map[string]OperationContract{
			"evaluate": {RequiredInputFields: []string{"policy_bundle", "resource"}},
		},
	}

	if err := familyReg.RegisterFamily(gitFamily); err != nil {
		t.Fatalf("failed to register git family: %v", err)
	}
	if err := familyReg.RegisterFamily(policyFamily); err != nil {
		t.Fatalf("failed to register policy family: %v", err)
	}

	// Bind implementations
	if err := familyReg.BindImplementation("git", "git-github"); err != nil {
		t.Fatalf("failed to bind git family: %v", err)
	}
	if err := familyReg.BindImplementation("policy", "other-policy"); err != nil {
		t.Fatalf("failed to bind policy family: %v", err)
	}

	// Test resolution
	conn, err := familyReg.Resolve("git")
	if err != nil {
		t.Fatalf("failed to resolve git family: %v", err)
	}
	if conn.Key() != "git-github" {
		t.Errorf("expected git-github connector, got %s", conn.Key())
	}

	// Test validation
	if err := familyReg.ValidateBindings(); err != nil {
		t.Errorf("validation should pass: %v", err)
	}

	// Test missing binding error
	familyReg2 := NewFamilyRegistry(concreteReg)
	if err := familyReg2.RegisterFamily(gitFamily); err != nil {
		t.Fatalf("failed to register git family: %v", err)
	}
	err = familyReg2.ValidateBindings()
	if err == nil || !strings.Contains(err.Error(), "family git has no bound implementation") {
		t.Errorf("expected validation error about missing binding, got: %v", err)
	}
}

func TestFamilyRegistryMissingConnector(t *testing.T) {
	concreteReg := NewConnectorRegistry()
	familyReg := NewFamilyRegistry(concreteReg)

	gitFamily := ConnectorFamily{
		Name: "git",
		Operations: map[string]OperationContract{
			"commit_files": {RequiredInputFields: []string{"repo", "branch", "path_prefix", "files", "message", "idempotency_key"}},
		},
	}

	if err := familyReg.RegisterFamily(gitFamily); err != nil {
		t.Fatalf("failed to register git family: %v", err)
	}

	// Try to bind to non-existent connector
	err := familyReg.BindImplementation("git", "git-nonexistent")
	if err == nil || !strings.Contains(err.Error(), "connector not found") {
		t.Errorf("expected error about connector not found, got: %v", err)
	}
}

func TestFamilyRegistryContractViolation(t *testing.T) {
	concreteReg := NewConnectorRegistry()
	familyReg := NewFamilyRegistry(concreteReg)

	// Create connector with only one operation
	baseGit, _ := NewBaseConnector(ConnectorTypeGit, "github")
	gitConn := &testConnector{BaseConnector: baseGit, operations: []string{"commit_files"}}

	if err := concreteReg.Register(gitConn); err != nil {
		t.Fatalf("failed to register git connector: %v", err)
	}

	// Register family requiring two operations
	gitFamily := ConnectorFamily{
		Name: "git",
		Operations: map[string]OperationContract{
			"commit_files": {RequiredInputFields: []string{"repo", "branch", "path_prefix", "files", "message", "idempotency_key"}},
			"read_file":    {RequiredInputFields: []string{"repo", "branch", "path"}},
		},
	}

	if err := familyReg.RegisterFamily(gitFamily); err != nil {
		t.Fatalf("failed to register git family: %v", err)
	}

	if err := familyReg.BindImplementation("git", "git-github"); err != nil {
		t.Fatalf("failed to bind git family: %v", err)
	}

	// Validation should fail
	err := familyReg.ValidateBindings()
	if err == nil || !strings.Contains(err.Error(), "operation read_file not supported") {
		t.Errorf("expected validation error about missing operation, got: %v", err)
	}
}

func TestSetupFamilyRegistry(t *testing.T) {
	concreteReg := NewConnectorRegistry()

	// Create test connectors
	baseGit, _ := NewBaseConnector(ConnectorTypeGit, "github")
	gitConn := &testConnector{BaseConnector: baseGit, operations: []string{"commit_files", "read_file"}}

	basePolicy, _ := NewBaseConnector(ConnectorTypeOther, "mockpolicy")
	policyConn := &testConnector{BaseConnector: basePolicy, operations: []string{"evaluate"}}

	baseWebhook, _ := NewBaseConnector(ConnectorTypeOther, "mockwebhook")
	webhookConn := &testConnector{BaseConnector: baseWebhook, operations: []string{"post_callback"}}

	// Register connectors
	if err := concreteReg.Register(gitConn); err != nil {
		t.Fatalf("failed to register git connector: %v", err)
	}
	if err := concreteReg.Register(policyConn); err != nil {
		t.Fatalf("failed to register policy connector: %v", err)
	}
	if err := concreteReg.Register(webhookConn); err != nil {
		t.Fatalf("failed to register webhook connector: %v", err)
	}

	// Test setup without config file - should fail because no bindings
	_, err := SetupFamilyRegistry(concreteReg, "")
	if err == nil {
		t.Fatalf("expected setup to fail without bindings, but it succeeded")
	}

	// Verify error message mentions missing bindings
	if !strings.Contains(err.Error(), "family git has no bound implementation") ||
		!strings.Contains(err.Error(), "family policy has no bound implementation") ||
		!strings.Contains(err.Error(), "family webhook has no bound implementation") {
		t.Errorf("expected error about missing bindings, got: %v", err)
	}

	// Note: we cannot call GetFamilies() because SetupFamilyRegistry failed
}
