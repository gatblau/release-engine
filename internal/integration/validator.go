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

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files with generated output")

// CrossplaneValidator validates Crossplane definitions emitted by modules.
type CrossplaneValidator struct {
	rules []ValidatorRule
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
	if _, ok := labels["tenant"]; !ok {
		return fmt.Errorf("metadata.labels must include tenant")
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

// AssertGoldenYAML compares output against a golden file and optionally overwrites it.
func AssertGoldenYAML(t testing.TB, name string, actual []byte) {
	t.Helper()
	base := filepath.Join("testdata", "golden")
	path := filepath.Join(base, fmt.Sprintf("%s.yaml", name))
	if *updateGolden {
		require.NoError(t, os.MkdirAll(base, 0o755))
		require.NoError(t, os.WriteFile(path, actual, 0o644))
		return
	}
	expected, err := os.ReadFile(path)
	require.NoError(t, err)
	if diff := cmp.Diff(strings.TrimSpace(string(expected)), strings.TrimSpace(string(actual))); diff != "" {
		require.Failf(t, "golden mismatch", "diff:\n%s", diff)
	}
}
