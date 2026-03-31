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
	baseGit, err := NewBaseConnector(ConnectorTypeGit, "github")
	if err != nil {
		t.Fatalf("failed to create git base connector: %v", err)
	}
	gitConn := &testConnector{BaseConnector: baseGit, operations: []string{"commit_files", "read_file"}}

	basePolicy, err := NewBaseConnector(ConnectorTypePolicy, "embedded")
	if err != nil {
		t.Fatalf("failed to create policy base connector: %v", err)
	}
	policyConn := &testConnector{BaseConnector: basePolicy, operations: []string{"evaluate"}}

	// Register connectors
	if err := concreteReg.Register(gitConn); err != nil {
		t.Fatalf("failed to register git connector: %v", err)
	}
	if err := concreteReg.Register(policyConn); err != nil {
		t.Fatalf("failed to register policy connector: %v", err)
	}

	// Register families with Members
	gitFamily := ConnectorFamily{
		Name:    "git",
		Members: []string{"git-github"},
		Operations: map[string]OperationContract{
			"commit_files": {RequiredInputFields: []string{"repo", "branch", "path_prefix", "files", "message", "idempotency_key"}},
			"read_file":    {RequiredInputFields: []string{"repo", "branch", "path"}},
		},
	}

	policyFamily := ConnectorFamily{
		Name:    "policy",
		Members: []string{"policy-embedded"},
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

	// Test resolution with per-module selection
	conn, err := familyReg.Resolve("git", "github")
	if err != nil {
		t.Fatalf("failed to resolve git family: %v", err)
	}
	if conn.Key() != "git-github" {
		t.Errorf("expected git-github connector, got %s", conn.Key())
	}

	// Test resolving policy with its member
	policyResolved, err := familyReg.Resolve("policy", "policy-embedded")
	if err != nil {
		t.Fatalf("failed to resolve policy family: %v", err)
	}
	if policyResolved.Key() != "policy-embedded" {
		t.Errorf("expected policy-embedded connector, got %s", policyResolved.Key())
	}
}

func TestFamilyRegistryUnknownFamily(t *testing.T) {
	concreteReg := NewConnectorRegistry()
	familyReg := NewFamilyRegistry(concreteReg)

	gitFamily := ConnectorFamily{
		Name:    "git",
		Members: []string{"git-github"},
		Operations: map[string]OperationContract{
			"commit_files": {RequiredInputFields: []string{"repo", "branch"}},
		},
	}

	if err := familyReg.RegisterFamily(gitFamily); err != nil {
		t.Fatalf("failed to register git family: %v", err)
	}

	// Try to resolve unknown family
	_, err := familyReg.Resolve("unknown", "git-github")
	if err == nil || !strings.Contains(err.Error(), "unknown family: unknown") {
		t.Errorf("expected error about unknown family, got: %v", err)
	}
}

func TestFamilyRegistryInvalidMember(t *testing.T) {
	concreteReg := NewConnectorRegistry()
	familyReg := NewFamilyRegistry(concreteReg)

	gitFamily := ConnectorFamily{
		Name:    "git",
		Members: []string{"git-github", "git-gitea"},
		Operations: map[string]OperationContract{
			"commit_files": {RequiredInputFields: []string{"repo", "branch"}},
		},
	}

	if err := familyReg.RegisterFamily(gitFamily); err != nil {
		t.Fatalf("failed to register git family: %v", err)
	}

	// Try to resolve with member not in the family
	_, err := familyReg.Resolve("git", "git-gitlab")
	if err == nil || !strings.Contains(err.Error(), "git-gitlab is not a member of family git") {
		t.Errorf("expected error about not being a member, got: %v", err)
	}
}

func TestFamilyRegistryMissingConnector(t *testing.T) {
	concreteReg := NewConnectorRegistry()
	familyReg := NewFamilyRegistry(concreteReg)

	gitFamily := ConnectorFamily{
		Name:    "git",
		Members: []string{"git-github", "git-nonexistent"},
		Operations: map[string]OperationContract{
			"commit_files": {RequiredInputFields: []string{"repo", "branch"}},
		},
	}

	if err := familyReg.RegisterFamily(gitFamily); err != nil {
		t.Fatalf("failed to register git family: %v", err)
	}

	// Try to resolve a member that's in the family but not registered
	_, err := familyReg.Resolve("git", "git-nonexistent")
	if err == nil || !strings.Contains(err.Error(), "connector not found") {
		t.Errorf("expected error about connector not found, got: %v", err)
	}
}

func TestConnectorFamilyHasMember(t *testing.T) {
	family := ConnectorFamily{
		Name:    "git",
		Members: []string{"git-github", "git-gitea"},
	}

	if !family.HasMember("git-github") {
		t.Error("expected HasMember to return true for git-github")
	}
	if !family.HasMember("git-gitea") {
		t.Error("expected HasMember to return true for git-gitea")
	}
	if family.HasMember("git-gitlab") {
		t.Error("expected HasMember to return false for git-gitlab")
	}
}

func TestSetupFamilyRegistry(t *testing.T) {
	concreteReg := NewConnectorRegistry()

	// Create test connectors with keys that match DefaultFamilies() members
	baseGit, _ := NewBaseConnector(ConnectorTypeGit, "github")
	gitConn := &testConnector{BaseConnector: baseGit, operations: []string{"commit_files", "read_file"}}

	basePolicy, _ := NewBaseConnector(ConnectorTypePolicy, "embedded")
	policyConn := &testConnector{BaseConnector: basePolicy, operations: []string{"evaluate"}}

	baseWebhook, _ := NewBaseConnector(ConnectorTypeOther, "webhook")
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

	// Test setup - should succeed with the new design (no global bindings required)
	familyReg, err := SetupFamilyRegistry(concreteReg, "")
	if err != nil {
		t.Fatalf("SetupFamilyRegistry failed: %v", err)
	}

	// Verify families are registered
	families := familyReg.GetFamilies()
	if len(families) == 0 {
		t.Error("expected families to be registered")
	}

	// Verify each default family is present
	expectedFamilies := []string{"git", "policy", "webhook", "infra"}
	for _, expected := range expectedFamilies {
		if _, ok := families[expected]; !ok {
			t.Errorf("expected family %s to be registered", expected)
		}
	}

	// Test per-module resolution works
	gitResolved, err := familyReg.Resolve("git", "github")
	if err != nil {
		t.Fatalf("failed to resolve git-github: %v", err)
	}
	if gitResolved.Key() != "git-github" {
		t.Errorf("expected git-github, got %s", gitResolved.Key())
	}
}
