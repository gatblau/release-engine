// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package integration

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files with generated output")

// CrossplaneValidator validates Crossplane definitions emitted by modules.
type CrossplaneValidator struct {
	rules []ValidatorRule
}

var xrdParameterContracts = map[string]struct {
	Required []string
	Allowed  map[string]struct{}
}{
	"XCache": {
		Required: []string{"engine", "region"},
		Allowed:  toSet([]string{"region", "providerConfigRef", "engine", "nodeType", "engineVersion", "replicaCount", "snapshotRetentionDays", "subnetGroupName", "securityGroupIds"}),
	},
	"XDatabase": {
		Required: []string{"engine", "region"},
		Allowed:  toSet([]string{"region", "providerConfigRef", "engine", "engineVersion", "instanceClass", "allocatedStorage", "storageType", "iops", "multiAZ", "readReplicas", "backupRetentionDays", "pointInTimeRecovery", "parameterGroup", "maintenanceWindow", "subnetGroupName", "securityGroupIds"}),
	},
	"XDNSZone": {
		Required: []string{"region", "zoneName"},
		Allowed:  toSet([]string{"providerConfigRef", "region", "zoneName", "private", "vpcId", "records"}),
	},
	"XKubernetesCluster": {
		Required: []string{"region", "version"},
		Allowed:  toSet([]string{"region", "version", "providerConfigRef", "clusterRoleArn", "nodeRoleArn", "subnetIds", "securityGroupIds", "nodePool"}),
	},
	"XLoadBalancer": {
		Required: []string{"region", "vpcId", "subnetIds"},
		Allowed:  toSet([]string{"providerConfigRef", "region", "type", "scheme", "idleTimeout", "vpcId", "subnetIds", "securityGroupIds", "https", "certificateArn", "waf", "wafAclArn", "healthCheck"}),
	},
	"XMessaging": {
		Required: []string{"region"},
		Allowed:  toSet([]string{"providerConfigRef", "region", "queueCount", "topicCount", "fifo", "encryption", "dlqEnabled", "dlqMaxRetry"}),
	},
	"XObjectStore": {
		Required: []string{"region"},
		Allowed:  toSet([]string{"region", "providerConfigRef", "versioning", "retentionDays", "encryption"}),
	},
	"XObservability": {
		Required: []string{"region"},
		Allowed:  toSet([]string{"region", "providerConfigRef", "metricsRetentionDays", "logRetentionDays", "logSinkType", "tracingEnabled", "tracingSampleRate", "dashboardEnabled"}),
	},
	"XSecretsStore": {
		Required: []string{"region"},
		Allowed:  toSet([]string{"region", "providerConfigRef", "kmsKeyType", "kmsRotationDays", "secretCount", "autoRotation", "rotationIntervalDays"}),
	},
	"XVPCNetwork": {
		Required: []string{"cidr", "region"},
		Allowed:  toSet([]string{"region", "providerConfigRef", "cidr", "privateSubnets", "publicSubnets", "natGateways", "flowLogs", "availabilityZones"}),
	},
}

// ValidatorRule performs validation on a decoded object.
type ValidatorRule interface {
	Name() string
	Validate(map[string]any) error
}

// NewCrossplaneValidator creates a validator with the default rule set.
func NewCrossplaneValidator(rules ...ValidatorRule) *CrossplaneValidator {
	if len(rules) == 0 {
		rules = []ValidatorRule{
			FieldPresenceRule{fields: []string{"apiVersion", "kind", "metadata", "spec"}},
			MetadataRule{},
			SpecRule{},
			XRDContractRule{},
		}
	}
	return &CrossplaneValidator{rules: rules}
}

// ValidateYAML parses the provided YAML (supports multi-doc) and runs each rule.
func (v *CrossplaneValidator) ValidateYAML(data []byte) error {
	if len(data) == 0 {
		return errors.New("manifest is empty")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var errs []string
	idx := 0
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) || err == nil {
				break
			}
			errs = append(errs, fmt.Sprintf("document[%d]: decode: %v", idx, err))
			break
		}
		if len(doc) == 0 {
			idx++
			continue
		}
		for _, rule := range v.rules {
			if err := rule.Validate(doc); err != nil {
				errs = append(errs, fmt.Sprintf("doc[%d] rule %s: %v", idx, rule.Name(), err))
			}
		}
		idx++
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// FieldPresenceRule ensures required top-level fields are present.
type FieldPresenceRule struct {
	fields []string
}

func (r FieldPresenceRule) Name() string { return "FieldPresence" }

func (r FieldPresenceRule) Validate(obj map[string]any) error {
	for _, field := range r.fields {
		if _, ok := obj[field]; !ok {
			return fmt.Errorf("field %s is required", field)
		}
	}
	return nil
}

// MetadataRule enforces metadata structure and labels.
type MetadataRule struct{}

func (MetadataRule) Name() string { return "Metadata" }

func (MetadataRule) Validate(obj map[string]any) error {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return fmt.Errorf("metadata must be an object")
	}
	if _, ok := metadata["name"].(string); !ok {
		return fmt.Errorf("metadata.name must be a string")
	}
	labels, ok := metadata["labels"].(map[string]any)
	if !ok {
		return fmt.Errorf("metadata.labels must be defined")
	}
	if _, ok := labels["platform.io/tenant"]; !ok {
		return fmt.Errorf("metadata.labels must include platform.io/tenant")
	}
	return nil
}

// SpecRule enforces spec is defined with nested maps.
type SpecRule struct{}

func (SpecRule) Name() string { return "Spec" }

func (SpecRule) Validate(obj map[string]any) error {
	spec, ok := obj["spec"].(map[string]any)
	if !ok {
		return fmt.Errorf("spec must be an object")
	}
	if len(spec) == 0 {
		return fmt.Errorf("spec must not be empty")
	}
	return nil
}

// XRDContractRule validates known kind contracts against documented XRD schemas.
type XRDContractRule struct{}

func (XRDContractRule) Name() string { return "XRDContract" }

func (XRDContractRule) Validate(obj map[string]any) error {
	kind, _ := obj["kind"].(string)
	contract, ok := xrdParameterContracts[kind]
	if !ok {
		return nil
	}
	spec, _ := obj["spec"].(map[string]any)
	params, ok := spec["parameters"].(map[string]any)
	if !ok {
		return fmt.Errorf("spec.parameters must be an object")
	}
	for _, req := range contract.Required {
		if _, ok := params[req]; !ok {
			return fmt.Errorf("spec.parameters.%s is required for kind %s", req, kind)
		}
	}
	for k := range params {
		if _, ok := contract.Allowed[k]; !ok {
			return fmt.Errorf("spec.parameters.%s is not allowed for kind %s", k, kind)
		}
	}
	return nil
}

func toSet(keys []string) map[string]struct{} {
	m := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return m
}

// AssertGoldenYAML compares output against a golden file and optionally overwrites it.
func AssertGoldenYAML(t testing.TB, name string, actual []byte) {
	t.Helper()

	// Validate name to prevent path traversal attacks.
	if !isValidGoldenName(name) {
		require.Failf(t, "invalid golden file name", "name %q must contain only alphanumeric characters, hyphens, and underscores", name)
		return
	}

	base := filepath.Join("testdata", "golden")

	// Use os.Root to scope all file access under the fixed base directory (Go >= 1.24).
	root, err := os.OpenRoot(base)
	require.NoError(t, err)
	defer func(root *os.Root) {
		if err = root.Close(); err != nil {
			t.Fatalf("failed to close root file: %v", err)
		}
	}(root)

	fileName := fmt.Sprintf("%s.yaml", name)

	if *updateGolden {
		require.NoError(t, os.MkdirAll(base, 0o750))
		f, err := root.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		require.NoError(t, err)
		defer func(f *os.File) {
			if err = f.Close(); err != nil {
				t.Fatalf("failed to close file %s: %v", fileName, err)
			}
		}(f)
		_, err = f.Write(actual)
		require.NoError(t, err)
		return
	}

	f, err := root.Open(fileName)
	require.NoError(t, err)
	defer func(f *os.File) {
		if err = f.Close(); err != nil {
			t.Fatalf("failed to close file %s: %v", fileName, err)
		}
	}(f)

	expected, err := io.ReadAll(f)
	require.NoError(t, err)

	if diff := cmp.Diff(strings.TrimSpace(string(expected)), strings.TrimSpace(string(actual))); diff != "" {
		require.Failf(t, "golden mismatch", "diff:\n%s", diff)
	}
}

// isValidGoldenName ensures the golden file name contains only safe characters,
// preventing path traversal when used in file path construction.
func isValidGoldenName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
