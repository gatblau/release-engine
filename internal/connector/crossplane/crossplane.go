// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package crossplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gatblau/release-engine/internal/connector"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type CrossplaneConnector struct {
	connector.BaseConnector
	client dynamic.Interface
	config connector.ConnectorConfig
	mu     sync.RWMutex
	closed bool
}

func GetDynamicClient(kubeconfig []byte) (dynamic.Interface, error) {
	var cfg *rest.Config
	var err error

	if len(kubeconfig) > 0 {
		cfg, err = clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to resolve kubernetes config: %w", err)
	}
	return dynamic.NewForConfig(cfg)
}

func NewCrossplaneConnector(cfg connector.ConnectorConfig, client dynamic.Interface) (*CrossplaneConnector, error) {
	base, err := connector.NewBaseConnector(connector.ConnectorTypeInfra, "crossplane")
	if err != nil {
		return nil, err
	}
	return &CrossplaneConnector{
		BaseConnector: base,
		client:        client,
		config:        cfg,
	}, nil
}

func (c *CrossplaneConnector) Validate(operation string, input map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return fmt.Errorf("connector is closed")
	}

	requiredFields := map[string][]string{
		"create_composite_resource": {"kind", "name", "manifest"},
		"get_resource_status":       {"kind", "name"},
		"delete_resource":           {"kind", "name"},
	}

	fields, ok := requiredFields[operation]
	if !ok {
		return fmt.Errorf("unknown operation: %s", operation)
	}

	for _, field := range fields {
		if _, ok := input[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	return nil
}

func (c *CrossplaneConnector) Execute(ctx context.Context, operation string, input map[string]interface{}) (*connector.ConnectorResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("connector is closed")
	}
	c.mu.RUnlock()

	switch operation {
	case "create_composite_resource":
		return c.createCompositeResource(ctx, input)
	case "get_resource_status":
		return c.getResourceStatus(ctx, input)
	case "delete_resource":
		return c.deleteResource(ctx, input)
	default:
		return nil, fmt.Errorf("operation not implemented: %s", operation)
	}
}

func (c *CrossplaneConnector) parseGVR(kind string) schema.GroupVersionResource {
	// this is simplified for stub purposes
	// real implementation would hit discovery client
	return schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: kind,
	}
}

func (c *CrossplaneConnector) createCompositeResource(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	kind := input["kind"].(string)
	name := input["name"].(string)

	gvr := c.parseGVR(kind)

	var obj map[string]interface{}
	switch v := input["manifest"].(type) {
	case string:
		if err := json.NewDecoder(bytes.NewReader([]byte(v))).Decode(&obj); err != nil {
			return nil, fmt.Errorf("invalid json manifest: %w", err)
		}
	case map[string]interface{}:
		// deep copy map using json to avoid concurrent map mutation by fake client
		by, _ := json.Marshal(v)
		_ = json.Unmarshal(by, &obj)
	default:
		return nil, fmt.Errorf("manifest must be a string or map[string]interface{}")
	}

	unstr := &unstructured.Unstructured{Object: obj}
	unstr.SetName(name)

	created, err := c.client.Resource(gvr).Create(ctx, unstr, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{"name": created.GetName(), "uid": created.GetUID()},
	}, nil
}

func (c *CrossplaneConnector) getResourceStatus(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	kind := input["kind"].(string)
	name := input["name"].(string)

	gvr := c.parseGVR(kind)

	obj, err := c.client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return &connector.ConnectorResult{Status: connector.StatusSuccess, Output: map[string]interface{}{"status": "Unknown"}}, nil
	}

	return &connector.ConnectorResult{
		Status: connector.StatusSuccess,
		Output: map[string]interface{}{"status": status},
	}, nil
}

func (c *CrossplaneConnector) deleteResource(ctx context.Context, input map[string]interface{}) (*connector.ConnectorResult, error) {
	kind := input["kind"].(string)
	name := input["name"].(string)

	gvr := c.parseGVR(kind)

	err := c.client.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return nil, err
	}

	return &connector.ConnectorResult{Status: connector.StatusSuccess}, nil
}

func (c *CrossplaneConnector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *CrossplaneConnector) Operations() []connector.OperationMeta {
	return []connector.OperationMeta{
		{Name: "create_composite_resource", IsAsync: true},
		{Name: "get_resource_status", IsAsync: false},
		{Name: "delete_resource", IsAsync: false},
	}
}
