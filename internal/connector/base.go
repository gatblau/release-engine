// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package connector

import (
	"fmt"
	"strings"
)

// BaseConnector provides common functionality for all connector implementations.
// Embed this in concrete connector structs.
type BaseConnector struct {
	connectorType ConnectorType
	technology    string
}

// NewBaseConnector creates a new BaseConnector with validation.
func NewBaseConnector(ctype ConnectorType, tech string) (BaseConnector, error) {
	if !ValidConnectorTypes[ctype] {
		return BaseConnector{}, fmt.Errorf("unknown connector type: %s", ctype)
	}
	if tech == "" {
		return BaseConnector{}, fmt.Errorf("technology must not be empty")
	}
	if strings.Contains(tech, "-") {
		return BaseConnector{}, fmt.Errorf("technology must not contain hyphens: %s", tech)
	}
	return BaseConnector{connectorType: ctype, technology: tech}, nil
}

func (b *BaseConnector) Type() ConnectorType { return b.connectorType }
func (b *BaseConnector) Technology() string  { return b.technology }
func (b *BaseConnector) Key() string         { return fmt.Sprintf("%s-%s", b.connectorType, b.technology) }
