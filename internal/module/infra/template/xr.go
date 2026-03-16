// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package template

// BuildXR constructs the top-level Crossplane XR manifest structure.
func BuildXR(params *ProvisionParams, specParts map[string]any) map[string]any {
	return map[string]any{
		"apiVersion": "infrastructure.platform.io/v1alpha1",
		"kind":       "InfrastructureRequest",
		"metadata": map[string]any{
			"name":      params.RequestName,
			"namespace": params.Namespace,
			"labels": map[string]string{
				"app.kubernetes.io/managed-by": "release-engine",
				"platform.io/tenant":           params.Tenant,
				"platform.io/environment":      params.Environment,
				"platform.io/template":         params.TemplateName,
			},
			"annotations": map[string]string{
				"platform.io/contract-version": params.ContractVersion,
				"platform.io/composition-ref":  params.CompositionRef,
			},
		},
		"spec": map[string]any{
			"compositionRef": map[string]any{
				"name": params.CompositionRef,
			},
			"parameters": specParts,
		},
	}
}
