## FinOps Tag Strategy

### Mandatory Tags (enforced by Go controller)

| Tag Key | Source | Example | Purpose |
|---|---|---|---|
| `cost-centre` | `cost_centre` field | `CC-4521` | Financial allocation |
| `service` | `request_name` field | `order-service-prod` | Group all resources for one service |
| `environment` | `environment` field | `production` | Separate prod/staging/dev spend |
| `owner` | `owner` field | `team-checkout` | Who to ask about cost anomalies |
| `managed-by` | Hardcoded | `release-engine` | Distinguish from manual resources |

### Recommended Tags (optional in manifest, added if not empty)

| Tag Key | Source | Example | Purpose |
|---|---|---|---|
| `business-unit` | `business_unit` field | `retail` | Executive-level reporting |
| `data-classification` | `data_classification` field | `confidential` | Security + compliance filtering |
| `ttl` | `ttl` field (ISO 8601) | `2025-12-31` | Identify forgotten non-prod resources |
| `project` | `project` field | `checkout-replatform` | Temporary initiative tracking |
| `tenant` | `tenant` field | `ecommerce` | Multi-tenant isolation |
| `catalogue-item` | `catalogue_item` field | `k8s-app` | Template/catalog tracking |

## Why Cost Centre Alone Fails

| Question | `cost-centre` answers it? |
|---|---|
| "How much does order-service cost?" | ❌ Multiple services share a cost centre |
| "What's our staging spend vs prod?" | ❌ |
| "Which capability costs most — DB or messaging?" | ❌ |
| "Who owns this orphaned resource?" | ❌ |
| "Is this resource managed or manually created?" | ❌ |
| "Can we delete this non-prod resource?" | ❌ No TTL |

## Implementation in the Go Controller

### Payload fields (add to ProvisionParams)

```yaml
contract_version: v1
request_name: order-service-prod
tenant: ecommerce
owner: checkout-team
environment: production
catalogue_item: k8s-app
cost_centre: CC-4521           # required for FinOps
business_unit: retail           # optional
project: checkout-replatform    # optional
data_classification: confidential # optional
ttl: ""                         # optional, ISO 8601 date format
extra_tags:                     # optional
  application: order-service
  team: checkout-team
```

### Tags fragment in Go

```go
func (f *TagsFragment) Render(params *template.ProvisionParams) (map[string]any, error) {
	tags := map[string]string{
		// Mandatory FinOps tags
		"cost-centre": params.CostCentre,
		"service":     params.RequestName, // Use RequestName as service identifier
		"environment": params.Environment,
		"owner":       params.Owner,
		"managed-by":  "release-engine",
		
		// Existing tags for backward compatibility
		"tenant":         params.Tenant,
		"catalogue-item": params.CatalogueItem,
	}
	
	// Optional FinOps tags (add only if not empty)
	if params.BusinessUnit != "" {
		tags["business-unit"] = params.BusinessUnit
	}
	if params.Project != "" {
		tags["project"] = params.Project
	}
	if params.DataClassification != "" {
		tags["data-classification"] = params.DataClassification
	}
	if params.TTL != "" {
		tags["ttl"] = params.TTL
	}

	// Merge user-defined extra tags (extra_tags overrides any automatic tags)
	for k, v := range params.ExtraTags {
		tags[k] = v
	}

	return map[string]any{"tags": tags}, nil
}
```

### Passed through XR parameters

```yaml
apiVersion: infrastructure.platform.io/v1alpha1
kind: XDatabase
metadata:
  name: order-service-prod-database
  namespace: platform-system
  labels:
    app.kubernetes.io/managed-by: release-engine
    platform.io/tenant: ecommerce
    platform.io/environment: production
    platform.io/catalogue-item: k8s-app
spec:
  compositionRef:
    name: database-aws
  parameters:
    region: eu-west-1
    engine: postgres
    instanceClass: db.t3.medium
    storageGiB: 100
    tags:
      cost-centre: CC-4521
      service: order-service-prod
      environment: production
      owner: checkout-team
      managed-by: release-engine
      tenant: ecommerce
      catalogue-item: k8s-app
      business-unit: retail
      project: checkout-replatform
      data-classification: confidential
      application: order-service
      team: checkout-team
```

### XRD schema for tags

```yaml
tags:
  type: object
  additionalProperties:
    type: string
  description: "FinOps tags applied to all resources in this capability"
```

### Composition applies tags to every resource

All AWS compositions (e.g., `internal/module/infra/xplane/composition/aws/*.yaml`) automatically propagate tags from XR parameters to cloud resources. For example, in the Kubernetes composition:

```yaml
# Inside function-go-templating template in kubernetes.yaml
tags:
{{- range $k, $v := $tags }}
  {{ $k }}: {{ $v | quote }}
{{- end }}
  managed-by: crossplane
  cluster: {{ $clusterName }}
```

## AWS Cost Explorer / CUR Reporting Dimensions

These tags enable:

| Report | Group by |
|---|---|
| Team chargeback | `cost-centre` |
| Service unit economics | `service` + `environment` |
| Capability cost breakdown | `service` + `capability` |
| Non-prod waste detection | `environment` != `production` + `ttl` < today |
| Orphan detection | `managed-by` != `platform-controller` |
| Project ROI | `project` |

Activate these as **AWS Cost Allocation Tags** in the Billing console. They appear in Cost Explorer and Cost & Usage Reports within 24 hours.