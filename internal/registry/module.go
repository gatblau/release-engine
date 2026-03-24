// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package registry

// QueryRequest represents a request to query a module for domain-specific data.
type QueryRequest struct {
	// Name is the query identifier, e.g., "list-resources", "resource-health"
	Name string `json:"name"`
	// Params contains query-specific parameters
	Params map[string]any `json:"params"`
}

// QueryResult represents the result of a module query.
type QueryResult struct {
	// Data contains the query result data (must be JSON-serializable)
	Data any `json:"data"`
	// Status indicates the query result status: "ok", "error", "partial"
	Status string `json:"status"`
	// Error contains a domain-level error message (empty if Status is "ok")
	Error string `json:"error,omitempty"`
}

// ModuleDescriptor describes a module's capabilities for discovery.
type ModuleDescriptor struct {
	// Name is the module's clean name, e.g., "infra", "scaffold"
	Name string `json:"name"`
	// Domain is the module's domain, e.g., "infrastructure", "deployment"
	Domain string `json:"domain"`
	// Operations describes the operations this module can execute
	Operations []OperationDescriptor `json:"operations"`
	// Queries describes the queries this module can answer
	Queries []QueryDescriptor `json:"queries"`
	// EntityTypes describes the entity types this module manages
	EntityTypes []EntityTypeDescriptor `json:"entity_types"`
}

// OperationDescriptor describes an executable operation.
type OperationDescriptor struct {
	// Name is the operation name, e.g., "provision", "deprovision"
	Name string `json:"name"`
	// Params maps parameter names to type descriptions
	Params map[string]string `json:"params"`
	// RequiresApproval indicates if this operation requires approval
	RequiresApproval bool `json:"requires_approval"`
}

// QueryDescriptor describes a query a module can answer.
type QueryDescriptor struct {
	// Name is the query name, e.g., "list-resources", "resource-health"
	Name string `json:"name"`
	// Description is a human-readable description of the query
	Description string `json:"description,omitempty"`
	// Params maps parameter names to type descriptions
	Params map[string]string `json:"params"`
}

// EntityTypeDescriptor describes an entity type managed by a module.
type EntityTypeDescriptor struct {
	// Kind is the entity kind, e.g., "rds-instance", "s3-bucket"
	Kind string `json:"kind"`
	// Attributes maps attribute names to type descriptions
	Attributes map[string]string `json:"attributes"`
}
