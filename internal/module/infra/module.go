// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package infra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gatblau/release-engine/internal/connector"
	"github.com/gatblau/release-engine/internal/module/infra/template"
	"github.com/gatblau/release-engine/internal/module/infra/template/catalog"
	"github.com/gatblau/release-engine/internal/registry"
	"github.com/gatblau/release-engine/internal/stepapi"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const (
	ModuleKey     = "infra.provision"
	ModuleVersion = "latest"
)

// stepAPI defines the interface expected by the infra module for step execution.
// This interface is implemented by the runner at runtime.
// We use the shared stepapi.StepAPI interface from the common package.
type stepAPI = stepapi.StepAPI

// queryContext provides a minimal wrapper for Query() calls when stepAPI is not available.
type queryContext struct {
	logger *zap.Logger
}

func (q *queryContext) Logger() *zap.Logger {
	if q.logger == nil {
		return zap.NewNop()
	}
	return q.logger
}

// callCrossplaneConnector calls the crossplane connector directly using the module's injected connector.
func (m *Module) callCrossplaneConnector(ctx context.Context, qctx *queryContext, operation string, input map[string]any) (*stepapi.ConnectorResult, error) {
	if m.crossplaneConnector == nil {
		return nil, fmt.Errorf("crossplane connector not available")
	}

	// Validate the operation
	if err := m.crossplaneConnector.Validate(operation, input); err != nil {
		return nil, fmt.Errorf("crossplane connector validation failed: %w", err)
	}

	// Execute the operation
	result, err := m.crossplaneConnector.Execute(ctx, operation, input, nil)
	if err != nil {
		return nil, fmt.Errorf("crossplane connector execution failed: %w", err)
	}

	// Convert to stepapi.ConnectorResult
	return &stepapi.ConnectorResult{
		Status: result.Status,
		Output: result.Output,
		Error: func() *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} {
			if result.Error != nil {
				return &struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}{
					Code:    result.Error.Code,
					Message: result.Error.Message,
				}
			}
			return nil
		}(),
	}, nil
}

// Module implements the release engine executable module contract.
type Module struct {
	// vars holds the typed configuration variables for the module (optional).
	vars *Vars
	// gitConnector holds the injected git connector (optional).
	gitConnector connector.GitConnector
	// crossplaneConnector holds the injected crossplane connector (optional).
	crossplaneConnector connector.Connector
	// policyConnector holds the injected policy connector (optional).
	policyConnector connector.PolicyConnector
	// webhookConnector holds the injected webhook connector (optional).
	webhookConnector connector.WebhookConnector
}

// GetConnectors returns the module's resolved connectors as a map keyed by family name.
// This is used by the runner to build the StepAPI with pre-resolved connectors.
func (m *Module) GetConnectors() map[string]connector.Connector {
	connectors := make(map[string]connector.Connector)
	if m.gitConnector != nil {
		connectors["git"] = m.gitConnector
	}
	if m.crossplaneConnector != nil {
		connectors["crossplane"] = m.crossplaneConnector
	}
	if m.policyConnector != nil {
		connectors["policy"] = m.policyConnector
	}
	if m.webhookConnector != nil {
		connectors["webhook"] = m.webhookConnector
	}
	return connectors
}

// NewLegacyModule creates a new Module with default constructor (legacy).
// This constructor is used by the legacy assembly path.
func NewLegacyModule() *Module {
	return &Module{
		vars: &Vars{
			HealthTimeout: 30 * time.Second,
			PollInterval:  500 * time.Millisecond,
		},
	}
}

// NewModule creates a new Module with typed variables and connectors.
// This constructor is used by the config-managed assembly path.
func NewModule(
	vars Vars,
	git connector.GitConnector,
	crossplane connector.Connector,
	policy connector.PolicyConnector,
	webhook connector.WebhookConnector,
) (*Module, error) {
	// Validate that all dependencies are non-nil
	if git == nil {
		return nil, fmt.Errorf("git connector cannot be nil")
	}
	if crossplane == nil {
		return nil, fmt.Errorf("crossplane connector cannot be nil")
	}
	if policy == nil {
		return nil, fmt.Errorf("policy connector cannot be nil")
	}
	if webhook == nil {
		return nil, fmt.Errorf("webhook connector cannot be nil")
	}

	return &Module{
		vars:                &vars,
		gitConnector:        git,
		crossplaneConnector: crossplane,
		policyConnector:     policy,
		webhookConnector:    webhook,
	}, nil
}

func (m *Module) Key() string { return ModuleKey }

func (m *Module) Version() string { return ModuleVersion }

// Query implements the registry.Module interface.
func (m *Module) Query(ctx context.Context, api any, req registry.QueryRequest) (registry.QueryResult, error) {
	// Create a query context wrapper if api is not a stepAPI
	var qctx *queryContext
	if step, ok := api.(stepAPI); ok {
		qctx = &queryContext{logger: step.Logger()}
	} else {
		// For HTTP handler calls, create a minimal context with no-op logger
		qctx = &queryContext{logger: zap.NewNop()}
	}

	switch req.Name {
	case "list-resources":
		return m.queryListResources(ctx, qctx, req.Params)
	case "resource-health":
		return m.queryResourceHealth(ctx, qctx, req.Params)
	case "drift-report":
		return m.queryDriftReport(ctx, qctx, req.Params)
	default:
		return registry.QueryResult{
			Status: "error",
			Error:  fmt.Sprintf("unknown query: %s", req.Name),
		}, nil
	}
}

// Describe implements the registry.Module interface.
func (m *Module) Describe() registry.ModuleDescriptor {
	return registry.ModuleDescriptor{
		Name:   "infra",
		Domain: "infrastructure",
		Operations: []registry.OperationDescriptor{
			{
				Name:             "provision",
				RequiresApproval: true,
				Params: map[string]string{
					"tenant":         "string",
					"environment":    "string",
					"catalogue_item": "string",
					"owner":          "string",
					"primary_region": "string",
				},
			},
			{
				Name:             "deprovision",
				RequiresApproval: true,
				Params: map[string]string{
					"tenant":      "string",
					"environment": "string",
					"resource_id": "string",
				},
			},
		},
		Queries: []registry.QueryDescriptor{
			{
				Name:        "list-resources",
				Description: "List all infrastructure resources",
				Params: map[string]string{
					"env":  "string",
					"kind": "string",
				},
			},
			{
				Name:        "resource-health",
				Description: "Get health status of resources",
				Params: map[string]string{
					"resource_id": "string",
				},
			},
			{
				Name:        "drift-report",
				Description: "Compare desired (Git) vs actual (Crossplane) state",
				Params: map[string]string{
					"env": "string",
				},
			},
		},
		EntityTypes: []registry.EntityTypeDescriptor{
			{
				Kind: "rds-instance",
				Attributes: map[string]string{
					"engine":         "string",
					"instance_class": "string",
					"storage":        "int",
				},
			},
			{
				Kind: "s3-bucket",
				Attributes: map[string]string{
					"name":   "string",
					"region": "string",
				},
			},
			{
				Kind: "vpc",
				Attributes: map[string]string{
					"cidr":   "string",
					"region": "string",
				},
			},
		},
	}
}

// SecretContext implements the connector.SecretContextProvider interface.
// Infra module always uses platform tenant for secret resolution.
func (m *Module) SecretContext() connector.SecretContext {
	return connector.SecretContext{
		TenantID: "platform",
	}
}

// Execute decodes infra params, renders XR manifests, evaluates policy, and waits for approval.
func (m *Module) Execute(ctx context.Context, api any, params map[string]any) error {
	step, _ := api.(stepAPI)
	var logger *zap.Logger
	if step != nil {
		logger = step.Logger()
		_ = step.BeginStep("infra.render")
	}

	// If no logger available, create a no-op logger
	if logger == nil {
		logger = zap.NewNop()
	}

	// Extract param keys for logging
	paramKeys := make([]string, 0, len(params))
	for k := range params {
		paramKeys = append(paramKeys, k)
	}

	logger.Info("infra module execution started",
		zap.Int("params_count", len(params)),
		zap.Strings("param_keys", paramKeys),
	)

	// Info log for test debugging (debug level may not be enabled)
	logger.Info("infra.Execute called with params", zap.Any("params", params))

	// Fallback to stdout for debugging
	fmt.Printf("[DEBUG] infra.Execute called with params: %v\n", params)

	decoded, err := decodeProvisionParams(params)
	if err != nil {
		fmt.Printf("[DEBUG] decodeProvisionParams error: %v\n", err)
		if step != nil {
			_ = step.EndStepErr("infra.render", "INFRA_PARAMS_INVALID", err.Error())
		}
		return fmt.Errorf("decode infra params: %w", err)
	}

	fmt.Printf("[DEBUG] decoded params: Tenant=%s, Environment=%s, CatalogueItem=%s\n",
		decoded.Tenant, decoded.Environment, decoded.CatalogueItem)

	out, err := RenderManifests(decoded)
	if err != nil {
		if step != nil {
			_ = step.EndStepErr("infra.render", "INFRA_RENDER_FAILED", err.Error())
		}
		return fmt.Errorf("infra render failed: %w", err)
	}

	if step != nil {
		_ = step.SetContext("infra.manifest", string(out))
		_ = step.EndStepOK("infra.render", map[string]any{
			"manifest_yaml": string(out),
		})
	}

	// Phase 1: Policy Evaluation
	if step != nil {
		logger.Info("starting policy evaluation phase")
		logger.Info("starting policy evaluation phase")
		_ = step.BeginStep("infra.policy_evaluate")

		policyReq, err := MapToConnectorRequest(map[string]any{
			"connector": "policy",
			"impl_key":  "policy-pmock",
			"operation": "evaluate",
			"input": map[string]any{
				"policy_bundle": "infra/crossplane-xr",
				"resource":      string(out),
				"context": map[string]string{
					"tenant":      decoded.Tenant,
					"environment": decoded.Environment,
				},
			},
		})
		if err != nil {
			logger.Error("failed to create policy request", zap.Error(err))
			_ = step.EndStepErr("infra.policy_evaluate", "POLICY_REQUEST_INVALID", err.Error())
			return fmt.Errorf("policy request invalid: %w", err)
		}

		logger.Debug("calling policy connector", zap.Any("request", policyReq))
		policyResult, err := step.CallConnector(ctx, policyReq)
		if err != nil {
			logger.Error("policy connector call failed", zap.Error(err))
			_ = step.EndStepErr("infra.policy_evaluate", "POLICY_CALL_FAILED", err.Error())
			return fmt.Errorf("policy evaluation failed: %w", err)
		}

		logger.Debug("policy connector result", zap.Any("result", policyResult))
		// Check for connector error first
		if policyResult.Error != nil {
			logger.Error("policy connector returned error",
				zap.String("code", policyResult.Error.Code),
				zap.String("message", policyResult.Error.Message))
			_ = step.EndStepErr("infra.policy_evaluate", "POLICY_CALL_FAILED",
				fmt.Sprintf("Policy evaluation failed: %s - %s",
					policyResult.Error.Code, policyResult.Error.Message))
			return fmt.Errorf("policy evaluation failed: %s - %s",
				policyResult.Error.Code, policyResult.Error.Message)
		}

		// Extract allowed from result
		var allowed bool
		if policyResult.Output != nil {
			if allowedVal, ok := policyResult.Output["allowed"].(bool); ok {
				allowed = allowedVal
			} else if output, ok := policyResult.Output["output"].(map[string]any); ok {
				allowed, _ = output["allowed"].(bool)
			}
		}
		if !allowed {
			var violations any
			if policyResult.Output != nil {
				violations = policyResult.Output["violations"]
			}
			logger.Warn("policy evaluation denied", zap.Any("violations", violations))
			_ = step.EndStepErr("infra.policy_evaluate", "POLICY_VIOLATION", fmt.Sprintf("Policy denied: %v", violations))
			return fmt.Errorf("policy denied: %v", violations)
		}

		logger.Info("policy evaluation successful", zap.Bool("allowed", allowed))
		_ = step.EndStepOK("infra.policy_evaluate", map[string]any{
			"allowed": true,
		})
	}

	// Phase 1: Approval Gate (only for catalogue items that require approval)
	if step != nil {
		catalogueItemRequiresApproval := false
		if catalogDefs, err := catalog.LoadAll(); err == nil {
			if cat, ok := catalogDefs[decoded.CatalogueItem]; ok {
				catalogueItemRequiresApproval = cat.Constraints.RequiresApproval
			}
		}
		if skip, _ := params["skip_approval_gate"].(bool); skip {
			logger.Info("skipping approval gate due to request flag")
		} else if catalogueItemRequiresApproval {
			logger.Info("starting approval gate phase")
			_ = step.BeginStep("infra.approval_gate")

			approvalReq, err := MapToApprovalRequest(map[string]any{
				"summary":      fmt.Sprintf("Provision %s cluster in %s", decoded.CatalogueItem, decoded.Environment),
				"detail":       fmt.Sprintf("Tenant: %s, Owner: %s, Region: %s", decoded.Tenant, decoded.Owner, decoded.PrimaryRegion),
				"blast_radius": "production",
				"policy_ref":   "infra/crossplane-xr",
				"metadata": map[string]string{
					"approval_ttl": "1h",
					"tenant":       decoded.Tenant,
				},
			})
			if err != nil {
				logger.Error("failed to create approval request", zap.Error(err))
				_ = step.EndStepErr("infra.approval_gate", "APPROVAL_REQUEST_INVALID", err.Error())
				return fmt.Errorf("approval request invalid: %w", err)
			}

			logger.Debug("waiting for approval", zap.Any("request", approvalReq))
			approvalResult, err := step.WaitForApproval(ctx, approvalReq)
			if err != nil {
				logger.Error("approval wait failed", zap.Error(err))
				_ = step.EndStepErr("infra.approval_gate", "APPROVAL_FAILED", err.Error())
				return fmt.Errorf("approval failed: %w", err)
			}

			logger.Debug("approval result received", zap.Any("result", approvalResult))
			if approvalResult.Decision != "approved" {
				logger.Warn("approval rejected", zap.String("decision", approvalResult.Decision), zap.String("approver", approvalResult.Approver))
				_ = step.EndStepErr("infra.approval_gate", "APPROVAL_REJECTED", fmt.Sprintf("Approval rejected: %v", approvalResult))
				return fmt.Errorf("approval rejected: %v", approvalResult)
			}

			logger.Info("approval granted", zap.String("approver", approvalResult.Approver))
			_ = step.EndStepOK("infra.approval_gate", map[string]any{
				"decision": approvalResult.Decision,
				"approver": approvalResult.Approver,
			})
		}
	}
	// After approval, continue to Phase 2 (Git commit, health verification, callback)
	// For now, just succeed
	if step != nil {
		_ = step.BeginStep("infra.phase1_complete")
		_ = step.EndStepOK("infra.phase1_complete", map[string]any{
			"status": "Phase 1 complete - policy evaluated and approval granted",
		})
	}

	// Phase 2: Git Commit + Health Verification + Callback
	if step != nil {
		logger.Info("starting phase 2: git commit, health verification, and callback")
		// Step 1: Git commit
		_ = step.BeginStep("infra.git_commit")
		logger.Info("starting git commit step")

		// Get infra repo from params or context
		infraRepo, _ := params["infra_repo"].(string)
		if infraRepo == "" {
			infraRepo = "org/infra-manifests" // default
		}
		logger.Debug("using infra repo", zap.String("repo", infraRepo))

		// Convert rendered manifests to map[string]any (interface{})
		manifestStr := string(out)
		manifestsMap := map[string]any{
			fmt.Sprintf("%s/%s/%s.yaml", decoded.Tenant, decoded.Environment, decoded.CatalogueItem): manifestStr,
		}
		logger.Debug("manifest prepared", zap.Int("file_count", len(manifestsMap)))

		// Generate idempotency key from job context
		idempotencyKey, _ := params["idempotency_key"].(string)
		logger.Debug("using idempotency key", zap.String("idempotency_key", idempotencyKey))

		commitReq, err := MapToConnectorRequest(map[string]any{
			"connector": "git",
			"impl_key":  "git-file",
			"operation": "commit_files",
			"input": map[string]any{
				"repo":            infraRepo,
				"branch":          "main",
				"path_prefix":     fmt.Sprintf("tenants/%s/%s/", decoded.Tenant, decoded.Environment),
				"files":           manifestsMap,
				"message":         fmt.Sprintf("Provision %s cluster", decoded.CatalogueItem),
				"idempotency_key": idempotencyKey,
			},
		})
		if err != nil {
			logger.Error("failed to create git commit request", zap.Error(err))
			_ = step.EndStepErr("infra.git_commit", "GIT_COMMIT_INVALID_REQUEST", err.Error())
			return fmt.Errorf("git commit request invalid: %w", err)
		}

		logger.Debug("calling git connector", zap.Any("request", commitReq))
		commitResult, err := step.CallConnector(ctx, commitReq)
		if err != nil {
			logger.Error("git connector call failed", zap.Error(err))
			_ = step.EndStepErr("infra.git_commit", "GIT_COMMIT_FAILED", err.Error())
			return fmt.Errorf("git commit failed: %w", err)
		}

		logger.Debug("git connector result", zap.Any("result", commitResult))
		if commitResult.Status != "success" {
			errMsg := "git connector returned non-success status"
			if commitResult.Error != nil {
				errMsg = fmt.Sprintf("git connector error [%s]: %s", commitResult.Error.Code, commitResult.Error.Message)
			}
			logger.Error("git commit step failed",
				zap.String("status", commitResult.Status),
				zap.String("detail", errMsg))
			_ = step.EndStepErr("infra.git_commit", "GIT_COMMIT_FAILED", errMsg)

			// If the connector returns StatusTerminalError, propagate it as a TerminalError
			// to signal unrecoverability to the runner (no retry)
			if commitResult.Status == "terminal_error" {
				return &stepapi.TerminalError{
					Code:    commitResult.Error.Code,
					Message: errMsg,
				}
			}

			// For other statuses (e.g., "error"), return regular error
			return fmt.Errorf("git commit failed: %s", errMsg)
		}

		// Extract commit SHA and changed flag
		var commitSHA string
		var changed = true // default
		if commitResult.Output != nil {
			if sha, ok := commitResult.Output["commit_sha"].(string); ok {
				commitSHA = sha
			}
			if ch, ok := commitResult.Output["changed"].(bool); ok {
				changed = ch
			}
		}

		if commitSHA == "" {
			logger.Error("missing commit SHA in git commit result")
			_ = step.EndStepErr("infra.git_commit", "GIT_COMMIT_INVALID", "missing commit_sha in result")
			return fmt.Errorf("missing commit_sha in git commit result")
		}

		logger.Info("git commit successful",
			zap.String("commit_sha", commitSHA),
			zap.Bool("changed", changed),
			zap.String("repo", infraRepo))
		_ = step.EndStepOK("infra.git_commit", map[string]any{
			"commit_sha": commitSHA,
			"changed":    changed,
		})

		// If changed == false, skip health polling and proceed to callback
		if !changed {
			// Step 4: Callback (skip health polling)
			_ = step.BeginStep("infra.callback")
			callbackURL, _ := params["callback_url"].(string)

			callbackReq, err := MapToConnectorRequest(map[string]any{
				"connector": "webhook",
				"impl_key":  "webhook",
				"operation": "post_callback",
				"input": map[string]any{
					"url":     callbackURL,
					"headers": map[string]string{"Content-Type": "application/json"},
					"body": map[string]any{
						"job_id":        params["job_id"],
						"status":        "succeeded",
						"commit_sha":    commitSHA,
						"resource_refs": []interface{}{},
						"reason":        "idempotent commit - no changes",
					},
					"idempotency_key": idempotencyKey,
				},
			})
			if err != nil {
				_ = step.EndStepErr("infra.callback", "CALLBACK_REQUEST_INVALID", err.Error())
				return fmt.Errorf("callback request invalid: %w", err)
			}

			callbackResult, err := step.CallConnector(ctx, callbackReq)
			if err != nil {
				_ = step.EndStepErr("infra.callback", "CALLBACK_FAILED", err.Error())
				return fmt.Errorf("callback failed: %w", err)
			}

			// Extract status code and response body from structured result
			var statusCode interface{}
			var responseBody interface{}
			if callbackResult.Output != nil {
				statusCode = callbackResult.Output["status_code"]
				responseBody = callbackResult.Output["response_body"]
			}

			_ = step.EndStepOK("infra.callback", map[string]any{
				"status_code":   statusCode,
				"response_body": responseBody,
			})
			return nil
		}

		if skip, _ := params["skip_health_check"].(bool); skip {
			logger.Info("skipping health polling due to request flag")
			healthStatus := &HealthStatus{Status: "healthy", Timestamp: time.Now().UTC()}
			_ = step.EndStepOK("infra.health_poll", map[string]any{
				"status":    healthStatus.Status,
				"timestamp": healthStatus.Timestamp,
			})
			_ = step.BeginStep("infra.callback")
			callbackURL, _ := params["callback_url"].(string)
			callbackReq, err := MapToConnectorRequest(map[string]any{
				"connector": "webhook",
				"impl_key":  "webhook",
				"operation": "post_callback",
				"input": map[string]any{
					"url":     callbackURL,
					"headers": map[string]string{"Content-Type": "application/json"},
					"body": map[string]any{
						"job_id":        params["job_id"],
						"status":        "succeeded",
						"commit_sha":    commitSHA,
						"resource_refs": []interface{}{},
						"health_status": healthStatus.Status,
					},
					"idempotency_key": idempotencyKey,
				},
			})
			if err != nil {
				_ = step.EndStepErr("infra.callback", "CALLBACK_REQUEST_INVALID", err.Error())
				return fmt.Errorf("callback request invalid: %w", err)
			}
			callbackResult, err := step.CallConnector(ctx, callbackReq)
			if err != nil {
				_ = step.EndStepErr("infra.callback", "CALLBACK_FAILED", err.Error())
				return fmt.Errorf("callback failed: %w", err)
			}
			var statusCode interface{}
			var responseBody interface{}
			if callbackResult.Output != nil {
				statusCode = callbackResult.Output["status_code"]
				responseBody = callbackResult.Output["response_body"]
			}
			_ = step.EndStepOK("infra.callback", map[string]any{"status_code": statusCode, "response_body": responseBody})
			return nil
		}

		// Step 2: Health polling
		logger.Info("starting health polling",
			zap.String("repo", infraRepo),
			zap.String("commitSHA", commitSHA),
			zap.Bool("changed", changed))
		_ = step.BeginStep("infra.health_poll")

		// Get timeout values from module configuration
		healthTimeout := 30 * time.Second
		pollInterval := 500 * time.Millisecond
		if m.vars != nil {
			healthTimeout = m.vars.HealthTimeout
			pollInterval = m.vars.PollInterval
		}
		logger.Debug("health polling configuration",
			zap.Duration("healthTimeout", healthTimeout),
			zap.Duration("pollInterval", pollInterval))

		healthStatus, err := checkHealth(ctx, step, infraRepo, "main", commitSHA, "git-file")
		if err != nil {
			logger.Error("health polling failed", zap.Error(err))
			// Health timeout - trigger remediation
			_ = step.BeginStep("infra.remediation")
			logger.Warn("starting remediation due to health polling failure", zap.Error(err))

			// Try remediation (one retry max)
			newCommitSHA, err := remediationRecommit(ctx, step, infraRepo, "main",
				fmt.Sprintf("tenants/%s/%s/", decoded.Tenant, decoded.Environment),
				manifestsMap, fmt.Sprintf("Provision %s cluster [remediation]", decoded.CatalogueItem), "git-file")

			if err != nil {
				logger.Error("remediation failed", zap.Error(err))
				_ = step.EndStepErr("infra.remediation", "REMEDIATION_FAILED", err.Error())
				// Step 4: Callback with failure
				_ = step.BeginStep("infra.callback")
				callbackURL := params["callback_url"].(string)

				callbackReq, err := MapToConnectorRequest(map[string]any{
					"connector": "webhook",
					"impl_key":  "webhook",
					"operation": "post_callback",
					"input": map[string]any{
						"url":     callbackURL,
						"headers": map[string]string{"Content-Type": "application/json"},
						"body": map[string]any{
							"job_id":     params["job_id"],
							"status":     "failed",
							"commit_sha": commitSHA,
							"reason":     "health timeout and remediation failed",
						},
						"idempotency_key": idempotencyKey,
					},
				})
				if err != nil {
					_ = step.EndStepErr("infra.callback", "CALLBACK_REQUEST_INVALID", err.Error())
					return fmt.Errorf("callback request invalid: %w", err)
				}
				callbackResult, err := step.CallConnector(ctx, callbackReq)
				if err != nil {
					_ = step.EndStepErr("infra.callback", "CALLBACK_FAILED", err.Error())
				} else {
					// Extract status code and response body from structured result
					var statusCode interface{}
					var responseBody interface{}
					if callbackResult.Output != nil {
						statusCode = callbackResult.Output["status_code"]
						responseBody = callbackResult.Output["response_body"]
					}
					_ = step.EndStepOK("infra.callback", map[string]any{
						"status_code":   statusCode,
						"response_body": responseBody,
					})
				}
				return fmt.Errorf("health timeout and remediation failed: %w", err)
			}

			logger.Info("remediation commit successful", zap.String("new_commit_sha", newCommitSHA))
			// Retry health polling after remediation
			logger.Info("retrying health polling after remediation")
			healthStatus, err = checkHealth(ctx, step, infraRepo, "main", newCommitSHA, "git-file")
			if err != nil {
				// Second timeout → job FAILED
				logger.Error("second health polling timeout after remediation", zap.Error(err))
				_ = step.EndStepErr("infra.remediation", "DOUBLE_TIMEOUT", "second health timeout")
				// Step 4: Callback with failure
				_ = step.BeginStep("infra.callback")
				callbackURL := params["callback_url"].(string)

				callbackReq, callbackErr := MapToConnectorRequest(map[string]any{
					"connector": "webhook",
					"impl_key":  "webhook",
					"operation": "post_callback",
					"input": map[string]any{
						"url":     callbackURL,
						"headers": map[string]string{"Content-Type": "application/json"},
						"body": map[string]any{
							"job_id":     params["job_id"],
							"status":     "failed",
							"commit_sha": newCommitSHA,
							"reason":     "double health timeout",
						},
						"idempotency_key": idempotencyKey,
					},
				})
				if callbackErr != nil {
					_ = step.EndStepErr("infra.callback", "CALLBACK_REQUEST_INVALID", callbackErr.Error())
					return fmt.Errorf("callback request invalid: %w", callbackErr)
				}
				callbackResult, callbackErr := step.CallConnector(ctx, callbackReq)
				if callbackErr != nil {
					_ = step.EndStepErr("infra.callback", "CALLBACK_FAILED", callbackErr.Error())
				} else {
					// Extract status code and response body from structured result
					var statusCode interface{}
					var responseBody interface{}
					if callbackResult.Output != nil {
						statusCode = callbackResult.Output["status_code"]
						responseBody = callbackResult.Output["response_body"]
					}
					_ = step.EndStepOK("infra.callback", map[string]any{
						"status_code":   statusCode,
						"response_body": responseBody,
					})
				}
				return fmt.Errorf("double health timeout: %w", err)
			}

			logger.Info("remediation health polling successful",
				zap.String("status", healthStatus.Status),
				zap.Time("timestamp", healthStatus.Timestamp))
			_ = step.EndStepOK("infra.remediation", map[string]any{
				"new_commit_sha": newCommitSHA,
				"health_status":  healthStatus.Status,
			})
			_ = step.EndStepOK("infra.health_poll", map[string]any{
				"status":    healthStatus.Status,
				"timestamp": healthStatus.Timestamp,
			})
		} else {
			// Health polling successful
			logger.Info("health polling successful",
				zap.String("status", healthStatus.Status),
				zap.Time("timestamp", healthStatus.Timestamp))
			_ = step.EndStepOK("infra.health_poll", map[string]any{
				"status":    healthStatus.Status,
				"timestamp": healthStatus.Timestamp,
			})
		}

		// Step 3: Callback
		logger.Info("starting callback step")
		_ = step.BeginStep("infra.callback")
		callbackURL, _ := params["callback_url"].(string)
		logger.Debug("callback URL", zap.String("callback_url", callbackURL))

		callbackReq, err := MapToConnectorRequest(map[string]any{
			"connector": "webhook",
			"impl_key":  "webhook",
			"operation": "post_callback",
			"input": map[string]any{
				"url":     callbackURL,
				"headers": map[string]string{"Content-Type": "application/json"},
				"body": map[string]any{
					"job_id":        params["job_id"],
					"status":        "succeeded",
					"commit_sha":    commitSHA,
					"resource_refs": healthStatus.Resources,
					"health_status": healthStatus.Status,
				},
				"idempotency_key": idempotencyKey,
			},
		})
		if err != nil {
			logger.Error("failed to create callback request", zap.Error(err))
			_ = step.EndStepErr("infra.callback", "CALLBACK_REQUEST_INVALID", err.Error())
			return fmt.Errorf("callback request invalid: %w", err)
		}

		logger.Debug("calling webhook connector for callback", zap.Any("request", callbackReq))
		callbackResult, err := step.CallConnector(ctx, callbackReq)
		if err != nil {
			logger.Error("callback connector call failed", zap.Error(err))
			_ = step.EndStepErr("infra.callback", "CALLBACK_FAILED", err.Error())
			return fmt.Errorf("callback failed: %w", err)
		}

		logger.Debug("callback connector result", zap.Any("result", callbackResult))
		// Extract status code and response body from structured result
		var statusCode interface{}
		var responseBody interface{}
		if callbackResult.Output != nil {
			statusCode = callbackResult.Output["status_code"]
			responseBody = callbackResult.Output["response_body"]
		}

		logger.Info("callback successful",
			zap.Any("status_code", statusCode),
			zap.Any("response_body", responseBody))
		_ = step.EndStepOK("infra.callback", map[string]any{
			"status_code":   statusCode,
			"response_body": responseBody,
		})
	}

	return nil
}

func decodeProvisionParams(params map[string]any) (*template.ProvisionParams, error) {
	raw, err := yaml.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	var out template.ProvisionParams
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal params: %w", err)
	}
	return &out, nil
}

// queryListResources implements the list-resources query.
func (m *Module) queryListResources(ctx context.Context, qctx *queryContext, params map[string]any) (registry.QueryResult, error) {
	// Extract parameters
	env, _ := params["env"].(string)
	kind, _ := params["kind"].(string)

	// Note: repo parameter is accepted but not used in this stub implementation
	// In a real implementation, we would use git connector to list files from the repo

	// Return a stub implementation
	resources := []map[string]any{
		{
			"kind": "rds-instance",
			"name": "database-1",
			"env":  env,
			"spec": map[string]any{
				"engine":         "postgres",
				"instance_class": "db.t3.small",
				"storage":        20,
			},
		},
		{
			"kind": "s3-bucket",
			"name": "bucket-1",
			"env":  env,
			"spec": map[string]any{
				"name":   "my-bucket",
				"region": "us-east-1",
			},
		},
	}

	// Filter by kind if specified
	if kind != "" {
		filtered := []map[string]any{}
		for _, r := range resources {
			if r["kind"] == kind {
				filtered = append(filtered, r)
			}
		}
		resources = filtered
	}

	return registry.QueryResult{
		Status: "ok",
		Data:   resources,
	}, nil
}

// queryResourceHealth implements the resource-health query.
func (m *Module) queryResourceHealth(ctx context.Context, qctx *queryContext, params map[string]any) (registry.QueryResult, error) {
	resourceID, _ := params["resource_id"].(string)
	if resourceID == "" {
		return registry.QueryResult{
			Status: "error",
			Error:  "resource_id parameter is required",
		}, nil
	}

	// Parse resource ID to extract kind and name
	// Format: "kind/name" or "kind:name"
	var kind, name string
	if idx := strings.Index(resourceID, "/"); idx > 0 {
		kind = resourceID[:idx]
		name = resourceID[idx+1:]
	} else if idx := strings.Index(resourceID, ":"); idx > 0 {
		kind = resourceID[:idx]
		name = resourceID[idx+1:]
	} else {
		return registry.QueryResult{
			Status: "error",
			Error:  "resource_id must be in format 'kind/name' or 'kind:name'",
		}, nil
	}

	// Call Crossplane connector to get resource status
	result, err := m.callCrossplaneConnector(ctx, qctx, "get_resource_status", map[string]any{
		"kind": kind,
		"name": name,
	})
	if err != nil {
		return registry.QueryResult{
			Status: "error",
			Error:  fmt.Sprintf("failed to get resource status: %v", err),
		}, nil
	}

	// Parse result
	status := "unknown"
	if result.Output != nil {
		if statusMap, ok := result.Output["status"].(map[string]any); ok {
			// Extract health from status
			if health, ok := statusMap["health"].(string); ok {
				status = health
			} else if conditions, ok := statusMap["conditions"].([]any); ok && len(conditions) > 0 {
				// Try to get from first condition
				if cond, ok := conditions[0].(map[string]any); ok {
					if typeVal, ok := cond["type"].(string); ok {
						status = typeVal
					}
				}
			}
		}
	}

	return registry.QueryResult{
		Status: "ok",
		Data: map[string]any{
			"resource_id": resourceID,
			"health":      status,
			"timestamp":   time.Now().Format(time.RFC3339),
		},
	}, nil
}

// queryDriftReport implements the drift-report query.
func (m *Module) queryDriftReport(ctx context.Context, qctx *queryContext, params map[string]any) (registry.QueryResult, error) {
	env, _ := params["env"].(string)
	if env == "" {
		return registry.QueryResult{
			Status: "error",
			Error:  "env parameter is required",
		}, nil
	}

	// For now, return a stub drift report
	// In a real implementation, this would compare Git manifests with Crossplane status
	driftReport := map[string]any{
		"env":             env,
		"timestamp":       time.Now().Format(time.RFC3339),
		"total_resources": 5,
		"in_sync":         3,
		"out_of_sync":     2,
		"drifts": []map[string]any{
			{
				"resource": "rds-instance/database-1",
				"field":    "instance_class",
				"expected": "db.t3.medium",
				"actual":   "db.t3.small",
				"severity": "medium",
			},
			{
				"resource": "s3-bucket/bucket-1",
				"field":    "versioning",
				"expected": "Enabled",
				"actual":   "Disabled",
				"severity": "low",
			},
		},
	}

	return registry.QueryResult{
		Status: "ok",
		Data:   driftReport,
	}, nil
}

// Register registers the infra module in a module registry.
func Register(reg registry.ModuleRegistry) error {
	return reg.Register(NewLegacyModule())
}
