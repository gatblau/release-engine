# RFC-INFRA-005: JSON Schema Pack

These schemas are designed to be:

- **strict**
- **machine-usable**
- **validation-friendly**
- **aligned with RFC-INFRA-002 / 003 / 004**

They are written for **JSON Schema Draft 2020-12**.

---

## 1. `InfrastructureRequest` JSON Schema

**File:** `schemas/infrastructure-request-v1alpha1.schema.json`

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://platform.gatblau.io/schemas/infrastructure-request-v1alpha1.schema.json",
  "title": "InfrastructureRequest",
  "type": "object",
  "additionalProperties": false,
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "const": "platform.gatblau.io/v1alpha1"
    },
    "kind": {
      "type": "string",
      "const": "InfrastructureRequest"
    },
    "metadata": {
      "$ref": "#/$defs/metadata"
    },
    "spec": {
      "$ref": "#/$defs/spec"
    }
  },
  "$defs": {
    "dnsLabel": {
      "type": "string",
      "pattern": "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
      "minLength": 3,
      "maxLength": 63
    },
    "nonEmptyString": {
      "type": "string",
      "minLength": 1
    },
    "metadata": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "tenant"],
      "properties": {
        "name": {
          "$ref": "#/$defs/dnsLabel"
        },
        "tenant": {
          "type": "string",
          "minLength": 1,
          "maxLength": 64
        },
        "labels": {
          "type": "object",
          "default": {},
          "propertyNames": {
            "type": "string",
            "minLength": 1,
            "maxLength": 128
          },
          "additionalProperties": {
            "type": "string",
            "maxLength": 256
          }
        }
      }
    },
    "spec": {
      "type": "object",
      "additionalProperties": false,
      "required": ["owner", "environment", "workload"],
      "properties": {
        "owner": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128
        },
        "environment": {
          "type": "string",
          "enum": ["development", "test", "staging", "production"]
        },
        "workload": {
          "$ref": "#/$defs/workload"
        },
        "capabilities": {
          "$ref": "#/$defs/capabilities"
        },
        "location": {
          "$ref": "#/$defs/location"
        },
        "security": {
          "$ref": "#/$defs/security"
        },
        "operations": {
          "$ref": "#/$defs/operations"
        },
        "cost": {
          "$ref": "#/$defs/cost"
        },
        "delivery": {
          "$ref": "#/$defs/delivery"
        }
      }
    },
    "workload": {
      "type": "object",
      "additionalProperties": false,
      "required": ["type"],
      "properties": {
        "type": {
          "type": "string",
          "enum": [
            "web-service",
            "api-service",
            "worker-service",
            "data-platform",
            "analytics-platform",
            "database-service",
            "object-storage-service"
          ]
        },
        "profile": {
          "type": "string",
          "enum": ["small", "medium", "large"]
        },
        "exposure": {
          "type": "string",
          "enum": ["private", "internal", "public"]
        }
      }
    },
    "capabilities": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "kubernetes": {
          "$ref": "#/$defs/kubernetesCapability"
        },
        "database": {
          "$ref": "#/$defs/databaseCapability"
        },
        "objectStorage": {
          "$ref": "#/$defs/objectStorageCapability"
        },
        "messaging": {
          "$ref": "#/$defs/messagingCapability"
        }
      }
    },
    "kubernetesCapability": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "tier": {
          "type": "string",
          "enum": ["standard", "hardened"]
        },
        "size": {
          "type": "string",
          "enum": ["small", "medium", "large"]
        },
        "multiAz": {
          "type": "boolean"
        }
      }
    },
    "databaseCapability": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "engine": {
          "type": "string",
          "enum": ["postgres", "mysql"]
        },
        "tier": {
          "type": "string",
          "enum": ["dev", "standard", "highly-available"]
        },
        "storageGiB": {
          "type": "integer",
          "minimum": 10,
          "maximum": 65536
        },
        "backup": {
          "$ref": "#/$defs/databaseBackup"
        }
      }
    },
    "databaseBackup": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "retentionDays": {
          "type": "integer",
          "minimum": 1,
          "maximum": 365
        }
      }
    },
    "objectStorageCapability": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "class": {
          "type": "string",
          "enum": ["standard", "infrequent-access"]
        },
        "versioning": {
          "type": "boolean"
        }
      }
    },
    "messagingCapability": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "enabled": {
          "type": "boolean"
        },
        "tier": {
          "type": "string",
          "enum": ["standard", "high-throughput"]
        }
      }
    },
    "location": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "residency": {
          "type": "string",
          "enum": ["eu", "uk", "us", "global"]
        },
        "primaryRegion": {
          "type": "string",
          "minLength": 1,
          "maxLength": 64
        },
        "secondaryRegion": {
          "type": "string",
          "minLength": 1,
          "maxLength": 64
        }
      }
    },
    "security": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "compliance": {
          "type": "array",
          "uniqueItems": true,
          "items": {
            "type": "string",
            "enum": ["none", "gdpr", "pci", "sox", "hipaa"]
          }
        },
        "ingress": {
          "type": "string",
          "enum": ["private", "internal", "public"]
        },
        "egress": {
          "type": "string",
          "enum": ["open", "controlled", "restricted"]
        },
        "dataClassification": {
          "type": "string",
          "enum": ["public", "internal", "confidential", "restricted"]
        }
      }
    },
    "operations": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "availability": {
          "type": "string",
          "enum": ["best-effort", "standard", "high"]
        },
        "backupRequired": {
          "type": "boolean"
        },
        "drRequired": {
          "type": "boolean"
        },
        "monitoring": {
          "type": "string",
          "enum": ["basic", "standard", "enhanced"]
        }
      }
    },
    "cost": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "maxMonthly": {
          "type": "number",
          "minimum": 0
        },
        "currency": {
          "type": "string",
          "enum": ["GBP", "USD", "EUR"]
        },
        "approvalRequiredAbove": {
          "type": "number",
          "minimum": 0
        }
      }
    },
    "delivery": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "strategy": {
          "type": "string",
          "enum": ["direct-commit", "pull-request"]
        },
        "verifyCloud": {
          "type": "boolean"
        },
        "callbackUrl": {
          "type": "string",
          "format": "uri"
        }
      }
    }
  }
}
```

---

## 2. `CapabilityCatalog` JSON Schema

**File:** `schemas/capability-catalog-v1.schema.json`

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://platform.gatblau.io/schemas/capability-catalog-v1.schema.json",
  "title": "CapabilityCatalog",
  "type": "object",
  "additionalProperties": false,
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "const": "platform.gatblau.io/v1"
    },
    "kind": {
      "type": "string",
      "const": "CapabilityCatalog"
    },
    "metadata": {
      "$ref": "#/$defs/metadata"
    },
    "spec": {
      "$ref": "#/$defs/spec"
    }
  },
  "$defs": {
    "metadata": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "version"],
      "properties": {
        "name": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128
        },
        "version": {
          "type": "string",
          "minLength": 1,
          "maxLength": 64
        }
      }
    },
    "spec": {
      "type": "object",
      "additionalProperties": false,
      "required": ["templates"],
      "properties": {
        "templates": {
          "type": "array",
          "minItems": 1,
          "items": {
            "$ref": "#/$defs/template"
          }
        },
        "policies": {
          "type": "array",
          "items": {
            "type": "object"
          }
        }
      }
    },
    "template": {
      "type": "object",
      "additionalProperties": false,
      "required": ["id", "match", "resolvesTo"],
      "properties": {
        "id": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128
        },
        "displayName": {
          "type": "string",
          "minLength": 1,
          "maxLength": 256
        },
        "match": {
          "$ref": "#/$defs/match"
        },
        "requires": {
          "$ref": "#/$defs/requires"
        },
        "allows": {
          "$ref": "#/$defs/allows"
        },
        "resolvesTo": {
          "$ref": "#/$defs/resolvesTo"
        },
        "defaults": {
          "$ref": "#/$defs/defaults"
        },
        "policyTags": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "uniqueItems": true
        },
        "costModel": {
          "type": "object",
          "propertyNames": {
            "type": "string",
            "minLength": 1
          },
          "additionalProperties": {
            "type": "number",
            "minimum": 0
          }
        }
      }
    },
    "match": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "workloadType": {
          "type": "string"
        },
        "environment": {
          "type": "string"
        },
        "exposure": {
          "type": "string"
        }
      }
    },
    "requires": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "capabilities": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "kubernetes": { "type": "boolean" },
            "database": { "type": "boolean" },
            "objectStorage": { "type": "boolean" },
            "messaging": { "type": "boolean" }
          }
        }
      }
    },
    "allows": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "regions": {
          "type": "array",
          "items": { "type": "string" },
          "uniqueItems": true
        },
        "databaseEngines": {
          "type": "array",
          "items": {
            "type": "string",
            "enum": ["postgres", "mysql"]
          },
          "uniqueItems": true
        },
        "compliance": {
          "type": "array",
          "items": {
            "type": "string",
            "enum": ["none", "gdpr", "pci", "sox", "hipaa"]
          },
          "uniqueItems": true
        }
      }
    },
    "resolvesTo": {
      "type": "object",
      "additionalProperties": false,
      "required": ["templateName", "compositionRef"],
      "properties": {
        "templateName": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128
        },
        "compositionRef": {
          "type": "string",
          "minLength": 1,
          "maxLength": 256
        },
        "namespaceStrategy": {
          "type": "string",
          "enum": ["owner", "fixed", "tenant"]
        },
        "fixedNamespace": {
          "type": "string",
          "minLength": 1,
          "maxLength": 63
        }
      },
      "allOf": [
        {
          "if": {
            "properties": {
              "namespaceStrategy": { "const": "fixed" }
            },
            "required": ["namespaceStrategy"]
          },
          "then": {
            "required": ["fixedNamespace"]
          }
        }
      ]
    },
    "defaults": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "gitStrategy": {
          "type": "string",
          "enum": ["direct-commit", "pull-request"]
        },
        "verifyCloud": {
          "type": "boolean"
        },
        "approvalClass": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128
        }
      }
    }
  }
}
```

---

## 3. `CompiledProvisioningPlan` JSON Schema

**File:** `schemas/compiled-provisioning-plan-v1.schema.json`

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://platform.gatblau.io/schemas/compiled-provisioning-plan-v1.schema.json",
  "title": "CompiledProvisioningPlan",
  "type": "object",
  "additionalProperties": false,
  "required": ["apiVersion", "kind", "metadata", "summary", "source", "resolution", "job"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "const": "platform.gatblau.io/v1"
    },
    "kind": {
      "type": "string",
      "const": "CompiledProvisioningPlan"
    },
    "metadata": {
      "$ref": "#/$defs/metadata"
    },
    "summary": {
      "$ref": "#/$defs/summary"
    },
    "source": {
      "$ref": "#/$defs/source"
    },
    "resolution": {
      "$ref": "#/$defs/resolution"
    },
    "job": {
      "$ref": "#/$defs/job"
    }
  },
  "$defs": {
    "metadata": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "tenant"],
      "properties": {
        "name": {
          "type": "string",
          "minLength": 1,
          "maxLength": 63
        },
        "tenant": {
          "type": "string",
          "minLength": 1,
          "maxLength": 64
        }
      }
    },
    "summary": {
      "type": "object",
      "additionalProperties": false,
      "required": ["template", "blastRadius", "estimatedMonthlyCost", "currency"],
      "properties": {
        "template": {
          "type": "string"
        },
        "blastRadius": {
          "type": "string",
          "enum": ["low", "medium", "high"]
        },
        "estimatedMonthlyCost": {
          "type": "number",
          "minimum": 0
        },
        "currency": {
          "type": "string",
          "enum": ["GBP", "USD", "EUR"]
        }
      }
    },
    "source": {
      "type": "object",
      "additionalProperties": false,
      "required": ["requestHash", "catalogVersion", "policyVersion", "compilerVersion"],
      "properties": {
        "requestHash": {
          "type": "string",
          "pattern": "^sha256:[a-f0-9]{64}$"
        },
        "catalogVersion": {
          "type": "string"
        },
        "policyVersion": {
          "type": "string"
        },
        "compilerVersion": {
          "type": "string"
        }
      }
    },
    "resolution": {
      "type": "object",
      "additionalProperties": false,
      "required": ["namespace", "compositionRef", "gitStrategy", "verifyCloud"],
      "properties": {
        "namespace": {
          "type": "string",
          "minLength": 1,
          "maxLength": 63
        },
        "compositionRef": {
          "type": "string",
          "minLength": 1
        },
        "gitStrategy": {
          "type": "string",
          "enum": ["direct-commit", "pull-request"]
        },
        "verifyCloud": {
          "type": "boolean"
        }
      }
    },
    "job": {
      "type": "object",
      "additionalProperties": false,
      "required": ["tenant_id", "path_key", "idempotency_key", "params"],
      "properties": {
        "tenant_id": {
          "type": "string",
          "minLength": 1,
          "maxLength": 64
        },
        "path_key": {
          "type": "string",
          "minLength": 1,
          "maxLength": 256
        },
        "idempotency_key": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128
        },
        "params": {
          "type": "object"
        }
      }
    }
  }
}
```

---

## 4. `ValidationResult` JSON Schema

**File:** `schemas/validation-result-v1.schema.json`

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://platform.gatblau.io/schemas/validation-result-v1.schema.json",
  "title": "ValidationResult",
  "type": "object",
  "additionalProperties": false,
  "required": ["apiVersion", "kind", "result"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "const": "platform.gatblau.io/v1"
    },
    "kind": {
      "type": "string",
      "const": "ValidationResult"
    },
    "result": {
      "$ref": "#/$defs/result"
    }
  },
  "$defs": {
    "diagnostic": {
      "type": "object",
      "additionalProperties": false,
      "required": ["code", "message"],
      "properties": {
        "code": {
          "type": "string",
          "minLength": 1
        },
        "message": {
          "type": "string",
          "minLength": 1
        },
        "field": {
          "type": "string"
        },
        "severity": {
          "type": "string",
          "enum": ["error", "warning", "info"]
        }
      }
    },
    "derived": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "blastRadius": {
          "type": "string",
          "enum": ["low", "medium", "high"]
        },
        "approvalRequired": {
          "type": "boolean"
        },
        "matchedTemplate": {
          "type": "string"
        }
      }
    },
    "result": {
      "type": "object",
      "additionalProperties": false,
      "required": ["valid", "status", "errors", "warnings"],
      "properties": {
        "valid": {
          "type": "boolean"
        },
        "status": {
          "type": "string",
          "enum": ["allow", "allow_with_approval", "deny", "invalid"]
        },
        "errors": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/diagnostic"
          }
        },
        "warnings": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/diagnostic"
          }
        },
        "derived": {
          "$ref": "#/$defs/derived"
        }
      }
    }
  }
}
```

---

## 5. Suggested schema directory layout

```text
schemas/
├── infrastructure-request-v1alpha1.schema.json
├── capability-catalog-v1.schema.json
├── compiled-provisioning-plan-v1.schema.json
└── validation-result-v1.schema.json
```

---

## 6. Example valid `InfrastructureRequest`

```yaml
apiVersion: platform.gatblau.io/v1alpha1
kind: InfrastructureRequest
metadata:
  name: analytics-prod-eu
  tenant: acme
spec:
  owner: team-data
  environment: production
  workload:
    type: analytics-platform
    profile: medium
    exposure: internal
  capabilities:
    kubernetes:
      enabled: true
      tier: standard
      size: medium
      multiAz: true
    database:
      enabled: true
      engine: postgres
      tier: highly-available
      storageGiB: 500
      backup:
        enabled: true
        retentionDays: 30
    objectStorage:
      enabled: true
      class: standard
      versioning: true
  location:
    residency: eu
    primaryRegion: eu-west-1
    secondaryRegion: eu-central-1
  security:
    compliance:
      - gdpr
    ingress: internal
    egress: controlled
    dataClassification: confidential
  operations:
    availability: high
    backupRequired: true
    drRequired: true
    monitoring: enhanced
  cost:
    maxMonthly: 3000
    currency: GBP
    approvalRequiredAbove: 2000
  delivery:
    strategy: pull-request
    verifyCloud: true
    callbackUrl: https://internal.example/callback
```

---