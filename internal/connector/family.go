// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

// ConnectorFamily defines a family of connectors that share the same contract.
type ConnectorFamily struct {
	Name       string                       // e.g., "git", "policy", "webhook"
	Members    []string                     // valid connector keys belonging to this family, e.g., ["git-github", "git-gitea"]
	Operations map[string]OperationContract // operation name → contract
}

// HasMember returns true if the given connector key is a member of this family.
func (f ConnectorFamily) HasMember(key string) bool {
	for _, m := range f.Members {
		if m == key {
			return true
		}
	}
	return false
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

	// Resolve returns the concrete connector for a family using the specified implementation key.
	// It validates that the implementation key is a valid member of the family before returning.
	Resolve(family, implKey string) (Connector, error)

	// GetFamilies returns all registered families (for testing/inspection).
	GetFamilies() map[string]ConnectorFamily
}
