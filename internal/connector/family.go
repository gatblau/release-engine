// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

// ConnectorFamily defines a family of connectors that share the same contract.
type ConnectorFamily struct {
	Name       string                       // e.g., "git", "policy", "webhook"
	Operations map[string]OperationContract // operation name → contract
}

// OperationContract defines the input/output contract for a connector operation.
type OperationContract struct {
	RequiredInputFields  []string
	RequiredOutputFields []string
}

// FamilyRegistry manages connector families and their implementations.
type FamilyRegistry interface {
	// RegisterFamily registers a family contract.
	RegisterFamily(family ConnectorFamily) error

	// BindImplementation binds a concrete connector to a family.
	BindImplementation(familyName, connectorKey string) error

	// Resolve returns the concrete connector for a family.
	Resolve(familyName string) (Connector, error)

	// ValidateBindings validates all family bindings and their contracts.
	ValidateBindings() error

	// GetFamilies returns all registered families (for testing/inspection).
	GetFamilies() map[string]ConnectorFamily

	// GetBindings returns all family bindings (for testing/inspection).
	GetBindings() map[string]string
}
