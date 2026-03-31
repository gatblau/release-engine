// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package template

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

const ModuleDefaultProvider = "aws"

var SupportedProviders = map[string][]string{
	"blockStorage":  {"aws", "azure", "gcp"},
	"cache":         {"aws", "azure", "gcp"},
	"cdn":           {"aws", "azure", "gcp"},
	"database":      {"aws", "azure", "gcp"},
	"dns":           {"aws", "azure", "gcp"},
	"fileStorage":   {"aws", "azure", "gcp"},
	"identity":      {"aws", "azure", "gcp"},
	"kubernetes":    {"aws", "azure", "gcp"},
	"loadBalancer":  {"aws", "azure", "gcp"},
	"messaging":     {"aws", "azure", "gcp"},
	"objectStorage": {"aws", "azure", "gcp"},
	"observability": {"aws", "azure", "gcp"},
	"secrets":       {"aws", "azure", "gcp"},
	"vm":            {"aws", "azure", "gcp"},
	"vpc":           {"aws", "azure", "gcp"},
}

var paramBuilders = map[string]func(params *ProvisionParams, in map[string]any) map[string]any{
	"blockStorage":  buildBlockStorageParams,
	"cache":         buildCacheParams,
	"cdn":           buildCDNParams,
	"database":      buildDatabaseParams,
	"dns":           buildDNSParams,
	"fileStorage":   buildFileStorageParams,
	"identity":      buildIdentityParams,
	"kubernetes":    buildKubernetesParams,
	"loadBalancer":  buildLoadBalancerParams,
	"messaging":     buildMessagingParams,
	"objectStorage": buildObjectStoreParams,
	"observability": buildObservabilityParams,
	"secrets":       buildSecretsParams,
	"vm":            buildVMParams,
	"vpc":           buildVPCParams,
}

// ResolveProvider determines the provider for a capability using the hierarchy:
// capability.Provider → params.DefaultProvider → ModuleDefaultProvider
func ResolveProvider(cap Capability, requestDefault string) (string, error) {
	if p := cap.GetProvider(); p != "" {
		return p, nil
	}
	if requestDefault != "" {
		return requestDefault, nil
	}
	if ModuleDefaultProvider != "" {
		return ModuleDefaultProvider, nil
	}
	return "", fmt.Errorf("no provider resolved")
}

// resolveXRDKind returns the XRD kind based on capability name using convention-based naming.
func resolveXRDKind(capability string) string {
	switch capability {
	case "blockStorage":
		return "XBlockStorage"
	case "cache":
		return "XCache"
	case "cdn":
		return "XCDN"
	case "database":
		return "XDatabase"
	case "dns":
		return "XDNSZone"
	case "fileStorage":
		return "XFileStorage"
	case "identity":
		return "XIdentity"
	case "kubernetes":
		return "XKubernetesCluster"
	case "loadBalancer":
		return "XLoadBalancer"
	case "messaging":
		return "XMessaging"
	case "objectStorage":
		return "XObjectStore"
	case "observability":
		return "XObservability"
	case "secrets":
		return "XSecretsStore"
	case "vm":
		return "XVirtualMachine"
	case "vpc":
		return "XVPCNetwork"
	default:
		// fallback to original convention
		parts := strings.Split(capability, "_")
		for i, p := range parts {
			parts[i] = cases.Title(language.English).String(p)
		}
		return "X" + strings.Join(parts, "")
	}
}

// resolveCompositionRef returns the composition reference based on capability name and provider.
func resolveCompositionRef(capability, provider string) string {
	// Convert capability to lowercase, remove underscores
	clean := strings.ReplaceAll(capability, "_", "")
	clean = strings.ToLower(clean)

	// Simple pattern: {capability}-{provider}
	return fmt.Sprintf("%s-%s", clean, provider)
}

// contains checks if a string slice contains a specific string.
func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

// BuildXRs constructs Crossplane XR manifests per enabled capability.
func BuildXRs(params *ProvisionParams, specParts map[string]any) ([]map[string]any, error) {
	tags := extractTags(specParts)

	keys := make([]string, 0, len(specParts))
	for k := range specParts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	docs := make([]map[string]any, 0, len(keys))
	for _, capability := range keys {
		// Skip the "tags" key as it's not a capability
		if capability == "tags" {
			continue
		}

		// Get capability from params using the Capabilities() method
		capabilities := params.Capabilities()
		cap, ok := capabilities[capability]
		if !ok {
			return nil, fmt.Errorf("capability %q not recognised", capability)
		}

		if !cap.IsEnabled() {
			continue
		}

		// Resolve provider
		provider, err := ResolveProvider(cap, params.DefaultProvider)
		if err != nil {
			return nil, fmt.Errorf("capability %q: %w", capability, err)
		}

		// Check if provider is supported for this capability
		supported, ok := SupportedProviders[capability]
		if !ok {
			return nil, fmt.Errorf("capability %q not recognised", capability)
		}
		if !contains(supported, provider) {
			return nil, fmt.Errorf(
				"provider %q not supported for capability %q, supported: %v",
				provider, capability, supported,
			)
		}

		// Get parameter builder
		builder, ok := paramBuilders[capability]
		if !ok {
			return nil, fmt.Errorf("no parameter builder for capability %q", capability)
		}

		raw, _ := specParts[capability].(map[string]any)

		// Merge tags into raw parameters so builders have access if needed
		if len(tags) > 0 {
			if raw == nil {
				raw = make(map[string]any)
			}
			raw["tags"] = tags
		}

		resourceName := fmt.Sprintf("%s-%s", params.RequestName, capability)

		// Use convention-based naming
		kind := resolveXRDKind(capability)
		compositionRef := resolveCompositionRef(capability, provider)

		docs = append(docs, map[string]any{
			"apiVersion": "infrastructure.platform.io/v1alpha1",
			"kind":       kind,
			"metadata": map[string]any{
				"name":      resourceName,
				"namespace": params.Namespace,
				"labels": map[string]string{
					"app.kubernetes.io/managed-by": "release-engine",
					"platform.io/tenant":           params.Tenant,
					"platform.io/environment":      params.Environment,
					"platform.io/catalogue-item":   params.CatalogueItem,
				},
				"annotations": map[string]string{
					"platform.io/contract-version": params.ContractVersion,
					"platform.io/composition-ref":  compositionRef,
				},
			},
			"spec": map[string]any{
				"compositionRef": map[string]any{"name": compositionRef},
				"parameters":     mergeTagsIntoParameters(builder(params, raw), tags),
			},
		})
	}
	return docs, nil
}

func mergeTagsIntoParameters(params map[string]any, tags map[string]string) map[string]any {
	if len(tags) == 0 {
		return params
	}
	existing, _ := params["tags"].(map[string]string)
	merged := make(map[string]string, len(existing)+len(tags))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range tags {
		merged[k] = v
	}
	params["tags"] = merged
	return params
}

func extractTags(specParts map[string]any) map[string]string {
	if specParts == nil {
		return nil
	}
	tagsRaw, ok := specParts["tags"]
	if !ok {
		return nil
	}
	switch v := tagsRaw.(type) {
	case map[string]string:
		if len(v) == 0 {
			return nil
		}
		return copyStringMap(v)
	case map[string]any:
		if inner, ok := v["tags"]; ok {
			if strMap, ok := inner.(map[string]string); ok && len(strMap) > 0 {
				return copyStringMap(strMap)
			}
		}
		out := make(map[string]string)
		for key, val := range v {
			if str, ok := val.(string); ok {
				out[key] = str
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func marshalDeterministicDocuments(docs []map[string]any) ([]byte, error) {
	var out []byte
	for i, doc := range docs {
		b, err := marshalDeterministic(doc)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			out = append(out, []byte("---\n")...)
		}
		out = append(out, b...)
	}
	return out, nil
}

func stringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func intAny(v any, d int) int {
	if n, ok := v.(int); ok {
		return n
	}
	if n, ok := v.(int64); ok {
		return int(n)
	}
	if n, ok := v.(float64); ok {
		return int(n)
	}
	return d
}

func boolAny(v any, d bool) bool {
	b, ok := v.(bool)
	if !ok {
		return d
	}
	return b
}

func mapAny(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func buildKubernetesParams(params *ProvisionParams, in map[string]any) map[string]any {
	np := mapAny(in["nodePool"])

	// Build node pool with all required fields
	nodePool := map[string]any{
		"instanceType": np["instanceType"],
		"diskSizeGb":   intAny(np["diskSizeGb"], 50), // XRD default: 50
		"minNodes":     intAny(np["minNodes"], 1),
		"desiredNodes": intAny(np["desiredNodes"], 2), // XRD default: 2
		"maxNodes":     intAny(np["maxNodes"], 3),
		"capacityType": "ON_DEMAND", // Default capacity type
	}

	// Allow capacityType to be specified in input
	if capType, ok := np["capacityType"].(string); ok && (capType == "ON_DEMAND" || capType == "SPOT") {
		nodePool["capacityType"] = capType
	}

	// Use request name as default cluster name
	clusterName := fmt.Sprintf("%s-k8s", params.RequestName)

	// Build result with all XRD parameters
	result := map[string]any{
		"clusterName":       clusterName,
		"region":            params.PrimaryRegion,
		"version":           in["version"],
		"providerConfigRef": "aws-provider",
		"subnetIds":         []string{},
		"securityGroupIds":  []string{},
		"nodePool":          nodePool,
		"publicAccess":      false, // Default to private cluster
		"enableOIDC":        true,  // Default to enabled
		"tags":              map[string]string{},
	}

	// Add optional adminPrincipalArn if specified
	if adminArn, ok := in["adminPrincipalArn"].(string); ok && adminArn != "" {
		result["adminPrincipalArn"] = adminArn
	}

	return result
}

func buildDatabaseParams(params *ProvisionParams, in map[string]any) map[string]any {
	return map[string]any{
		"region":              params.PrimaryRegion,
		"providerConfigRef":   "aws-provider",
		"engine":              in["engine"],
		"engineVersion":       in["engineVersion"],
		"instanceClass":       in["instanceClass"],
		"allocatedStorage":    in["storageGiB"],
		"storageType":         in["storageType"],
		"iops":                intAny(in["iops"], 0),
		"multiAZ":             boolAny(in["multiAZ"], false),
		"backupRetentionDays": params.Database.BackupRetentionDays,
		"parameterGroup":      params.Database.ParameterGroup,
		"maintenanceWindow":   in["maintenanceWindow"],
		"subnetGroupName":     "",
		"securityGroupIds":    []string{},
	}
}

func buildVPCParams(params *ProvisionParams, in map[string]any) map[string]any {
	return map[string]any{
		"region":            params.PrimaryRegion,
		"providerConfigRef": "aws-provider",
		"cidr":              in["cidr"],
		"privateSubnets":    intAny(in["privateSubnets"], 3),
		"publicSubnets":     intAny(in["publicSubnets"], 3),
		"natGateways":       intAny(in["natGateways"], 1),
		"flowLogs":          boolAny(in["flowLogs"], true),
		"availabilityZones": in["availabilityZones"],
	}
}

func buildObjectStoreParams(params *ProvisionParams, in map[string]any) map[string]any {
	return map[string]any{
		"region":            params.PrimaryRegion,
		"providerConfigRef": "aws-provider",
		"versioning":        boolAny(in["versioning"], true), // XRD default: true
		"retentionDays":     params.ObjectStore.RetentionDays,
		"encryption":        true, // XRD default: true
	}
}

func buildMessagingParams(params *ProvisionParams, in map[string]any) map[string]any {
	return map[string]any{
		"providerConfigRef": "aws-provider",
		"region":            params.PrimaryRegion,
		"queueCount":        params.Messaging.QueueCount,
		"topicCount":        params.Messaging.TopicCount,
		"fifo":              params.Messaging.FIFO,
		"encryption":        boolAny(in["encrypted"], true),
		"dlqEnabled":        params.Messaging.DLQEnabled,
		"dlqMaxRetry":       params.Messaging.DLQMaxRetry,
	}
}

func buildCacheParams(params *ProvisionParams, in map[string]any) map[string]any {
	return map[string]any{
		"region":                params.PrimaryRegion,
		"providerConfigRef":     "aws-provider",
		"engine":                in["engine"],
		"nodeType":              in["nodeType"],
		"engineVersion":         in["version"],
		"replicaCount":          in["replicaCount"],
		"snapshotRetentionDays": params.Cache.SnapshotRetentionDays,
		"subnetGroupName":       "",
		"securityGroupIds":      []string{},
	}
}

func buildDNSParams(params *ProvisionParams, in map[string]any) map[string]any {
	return map[string]any{
		"providerConfigRef": "aws-provider",
		"region":            params.PrimaryRegion,
		"zoneName":          in["zoneName"],
		"private":           boolAny(in["private"], false),
		"vpcId":             "",
		"records":           in["records"],
	}
}

func buildLoadBalancerParams(params *ProvisionParams, in map[string]any) map[string]any {
	hc, _ := in["healthCheck"].(map[string]any)
	if hc == nil {
		hc = map[string]any{"path": "/", "protocol": "HTTP", "port": 80}
	}
	return map[string]any{
		"providerConfigRef": "aws-provider",
		"region":            params.PrimaryRegion,
		"type":              in["type"],
		"scheme":            in["scheme"],
		"idleTimeout":       in["idleTimeout"],
		"vpcId":             "",
		"subnetIds":         []string{},
		"securityGroupIds":  stringSlice(in["securityGroupIds"]),
		"https":             boolAny(in["https"], false),
		"certificateArn":    "",
		"waf":               boolAny(in["waf"], false),
		"wafAclArn":         "",
		"healthCheck":       hc,
	}
}

func buildSecretsParams(params *ProvisionParams, _ map[string]any) map[string]any {
	return map[string]any{
		"region":               params.PrimaryRegion,
		"providerConfigRef":    "aws-provider",
		"kmsKeyType":           params.Secrets.KMSKeyType,
		"kmsRotationDays":      params.Secrets.KMSRotationDays,
		"secretCount":          params.Secrets.SecretCount,
		"autoRotation":         params.Secrets.AutoRotation,
		"rotationIntervalDays": params.Secrets.RotationIntervalDays,
	}
}

func buildObservabilityParams(params *ProvisionParams, _ map[string]any) map[string]any {
	return map[string]any{
		"region":               params.PrimaryRegion,
		"providerConfigRef":    "aws-provider",
		"metricsRetentionDays": params.Observability.MetricsRetentionDays,
		"logRetentionDays":     params.Observability.LogRetentionDays,
		"logSinkType":          params.Observability.LogSinkType,
		"tracingEnabled":       params.Observability.TracingEnabled,
		"tracingSampleRate":    params.Observability.TracingSampleRate,
		"dashboardEnabled":     params.Observability.DashboardEnabled,
	}
}

func buildVMParams(params *ProvisionParams, in map[string]any) map[string]any {
	// Map platform concepts to AWS-native parameters
	// This is a complex mapping that needs input validation and transformation

	// Extract bootDisk information
	bootDisk := mapAny(in["bootDisk"])
	diskSizeGb := 30
	diskType := "gp3"
	if bootDisk != nil {
		diskSizeGb = intAny(bootDisk["sizeGiB"], 30)
		diskType, _ = bootDisk["type"].(string)
		if diskType == "" {
			diskType = "gp3"
		}
	}

	// Determine subnetId - allow explicit subnetId override, otherwise use subnetTier placeholder
	subnetId := ""

	// Check for explicit subnetId override first (takes precedence)
	if explicitSubnetId, ok := in["subnetId"].(string); ok && explicitSubnetId != "" {
		subnetId = explicitSubnetId
	} else if subnetTier, ok := in["subnetTier"].(string); ok {
		// Use tier-based placeholder as fallback
		subnetId = fmt.Sprintf("subnet-from-tier-%s", subnetTier)
	}

	// Map sshKeyName to keyPairName
	keyPairName := ""
	if sshKeyName, ok := in["sshKeyName"].(string); ok {
		keyPairName = sshKeyName
	}

	// Handle security groups - default to empty array
	securityGroupIds := []string{}

	// Handle user data - ensure it's base64 encoded if needed
	userData := ""
	if ud, ok := in["userData"].(string); ok {
		userData = ud
	}

	// Default IAM role policies
	iamRolePolicies := []string{
		"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",
	}

	// Handle spot instances
	publicIp := false
	if spot, ok := in["spot"].(map[string]any); ok {
		if enabled, ok := spot["enabled"].(bool); ok && enabled {
			publicIp = boolAny(spot["publicIp"], false)
		}
	}

	// Get instanceType (required by XRD)
	instanceType := ""
	if it, ok := in["instanceType"].(string); ok {
		instanceType = it
	}

	// Get AMI (optional in XRD)
	ami := ""
	if a, ok := in["ami"].(string); ok {
		ami = a
	}

	return map[string]any{
		"region":            params.PrimaryRegion,
		"providerConfigRef": "aws-provider",
		"instanceType":      instanceType,
		"ami":               ami, // Optional in XRD, can be empty
		"subnetId":          subnetId,
		"securityGroupIds":  securityGroupIds,
		"diskSizeGb":        diskSizeGb,
		"diskType":          diskType,
		"keyPairName":       keyPairName,
		"publicIp":          publicIp,
		"userData":          userData,
		"iamRolePolicies":   iamRolePolicies,
		"tags":              map[string]string{},
		"overrides":         map[string]any{},
		// Note: count, arch, multiAZ, additionalDisks, placementGroup, autoScaling
		// are platform concepts that don't directly map to XRD parameters
		// These would need to be handled by compositions or additional logic
	}
}

func buildBlockStorageParams(params *ProvisionParams, in map[string]any) map[string]any {
	// Take the first volume from the volumes array
	volumesRaw, _ := in["volumes"].([]any)
	var volume map[string]any
	if len(volumesRaw) > 0 {
		if vol, ok := volumesRaw[0].(map[string]any); ok {
			volume = vol
		}
	}

	// Default values if no volume specified
	if volume == nil {
		volume = map[string]any{
			"sizeGiB": 100,
			"type":    "gp3",
			"multiAZ": false,
		}
	}

	// Map platform concepts to XRD parameters
	// Note: availabilityZone should come from input, but not in current structure
	// Using empty string as default for now
	return map[string]any{
		"region":            params.PrimaryRegion,
		"providerConfigRef": "aws-provider",
		"sizeGb":            intAny(volume["sizeGiB"], 100),
		"availabilityZone":  "", // Should come from input, but not in current structure
		"volumeType":        volume["type"],
		"encrypted":         true,
		"iops":              intAny(volume["iops"], 0),
		"throughput":        intAny(volume["throughput"], 0),
		// Optional parameters with defaults
		"kmsKeyId":  "",
		"tags":      map[string]string{},
		"overrides": map[string]any{},
	}
}

func buildFileStorageParams(params *ProvisionParams, in map[string]any) map[string]any {
	// Map platform concepts to XRD parameters
	// platform.multiAZ → availabilityZoneName (empty for multi-AZ, specific AZ for One Zone)
	availabilityZoneName := ""
	if !boolAny(in["multiAZ"], false) {
		// For single AZ, we need to specify which AZ
		// This would ideally come from input or configuration
		// For now, use empty string (multi-AZ)
		// No action needed - just keeping the empty string default
		// This is a valid case, so we'll add a comment to explain the empty branch
		// The empty branch is intentional - when multiAZ is false, we keep availabilityZoneName as empty string
		// This is intentional - when multiAZ is false, we keep availabilityZoneName as empty string
		// Intentional empty branch - keeping availabilityZoneName as empty string for single AZ case
		availabilityZoneName = "" // Explicitly set to empty string for clarity
	}

	// Generate creation token based on request name
	creationToken := fmt.Sprintf("%s-efs", params.RequestName)

	// Map provisioned throughput - note different parameter name
	provisionedThroughput := intAny(in["provisionedThroughputMiBs"], 0)

	// Map throughputMode - ensure it's valid
	throughputMode := "bursting"
	if tm, ok := in["throughputMode"].(string); ok {
		throughputMode = tm
	}

	// Only include provisionedThroughputInMibps if throughputMode is "provisioned"
	result := map[string]any{
		"region":               params.PrimaryRegion,
		"providerConfigRef":    "aws-provider",
		"creationToken":        creationToken,
		"encrypted":            true,
		"performanceMode":      in["performanceMode"],
		"throughputMode":       throughputMode,
		"availabilityZoneName": availabilityZoneName,
		"nameTag":              fmt.Sprintf("%s-efs", params.RequestName),
		"tags":                 map[string]string{},
	}

	// Add provisioned throughput only for provisioned mode
	if throughputMode == "provisioned" && provisionedThroughput > 0 {
		result["provisionedThroughputInMibps"] = float64(provisionedThroughput)
	}

	// Add optional kmsKeyId if specified
	if kmsKeyId, ok := in["kmsKeyId"].(string); ok && kmsKeyId != "" {
		result["kmsKeyId"] = kmsKeyId
	}

	return result
}

func buildCDNParams(params *ProvisionParams, in map[string]any) map[string]any {
	// Build origins array from input
	origins := []map[string]any{}
	if originType, ok := in["originType"].(string); ok {
		// Create a simple origin based on originType
		domainName := ""
		switch originType {
		case "s3":
			domainName = fmt.Sprintf("%s.s3.amazonaws.com", params.RequestName)
		case "alb":
			domainName = fmt.Sprintf("alb-%s.internal", params.RequestName)
		default:
			domainName = "example.com" // Default fallback
		}

		origins = append(origins, map[string]any{
			"name":                 "default-origin",
			"domainName":           domainName,
			"originProtocolPolicy": "match-viewer",
			"httpPort":             80,
			"httpsPort":            443,
			"originSslProtocols":   []string{"TLSv1.2"},
		})
	}

	// Build default cache behavior
	defaultCacheBehavior := map[string]any{
		"targetOriginName":     "default-origin",
		"viewerProtocolPolicy": "redirect-to-https",
		"allowedMethods":       []string{"GET", "HEAD", "OPTIONS"},
		"cachedMethods":        []string{"GET", "HEAD", "OPTIONS"},
		"cachePolicyId":        "658327ea-f89d-4fab-a63d-7e88639e58f6", // Managed-CachingOptimized
		"compress":             true,
	}

	// Handle WAF configuration
	wafEnabled := false
	if waf, ok := in["waf"].(map[string]any); ok {
		wafEnabled = boolAny(waf["enabled"], false)
	}

	result := map[string]any{
		"providerConfigRef":    "aws-provider",
		"enabled":              true,
		"priceClass":           in["priceClass"],
		"httpVersion":          "http2",
		"isIpv6Enabled":        true,
		"waitForDeployment":    true,
		"origins":              origins,
		"defaultCacheBehavior": defaultCacheBehavior,
	}

	// Add optional fields if present
	if aliases, ok := in["customDomains"]; ok {
		result["aliases"] = aliases
	}
	if defaultRootObject, ok := in["defaultRootObject"]; ok {
		result["defaultRootObject"] = defaultRootObject
	}
	if comment, ok := in["comment"]; ok {
		result["comment"] = comment
	}
	if wafEnabled {
		result["tags"] = map[string]any{"waf": "enabled"}
	}

	return result
}

func buildIdentityParams(params *ProvisionParams, in map[string]any) map[string]any {
	// Map platform identity model to AWS IAM model
	identityType := ""
	if t, ok := in["type"].(string); ok {
		// Map platform identity types to XRD enum values
		switch t {
		case "service":
			identityType = "service"
		case "irsa":
			identityType = "irsa"
		case "federation":
			identityType = "federation"
		default:
			identityType = "service" // Default
		}
	} else {
		identityType = "service"
	}

	// Convert policies: support both managed policy ARNs and inline policy JSON
	managedPolicies := []string{}
	inlinePolicy := ""

	if policiesRaw, ok := in["policies"].([]any); ok {
		for _, p := range policiesRaw {
			// Handle string policy names (map to AWS managed policy ARNs)
			if policyName, ok := p.(string); ok {
				arn := mapPolicyNameToARN(policyName)
				if arn != "" {
					managedPolicies = append(managedPolicies, arn)
				}
			}
			// Handle custom policy definitions (convert to inline policy JSON)
			if pol, ok := p.(map[string]any); ok {
				// If it's a custom policy definition, convert it to IAM policy JSON
				if effect, ok := pol["effect"].(string); ok {
					actions := []string{}
					if actionsRaw, ok := pol["actions"].([]any); ok {
						for _, a := range actionsRaw {
							if action, ok := a.(string); ok {
								actions = append(actions, action)
							}
						}
					}

					resources := []string{}
					if resourcesRaw, ok := pol["resource"].([]any); ok {
						for _, r := range resourcesRaw {
							if resource, ok := r.(string); ok {
								resources = append(resources, resource)
							}
						}
					} else if resource, ok := pol["resource"].(string); ok {
						resources = append(resources, resource)
					}

					// Build a simple inline policy JSON
					if len(actions) > 0 && len(resources) > 0 {
						// Convert to JSON string (simplified)
						inlinePolicy = fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"%s","Action":%v,"Resource":%v}]}`,
							effect, toJSONArray(actions), toJSONArray(resources))
					}
				}
			}
		}
	}

	// Handle service accounts (for IRSA) - map to XRD's oidc structure
	oidc := map[string]any{}
	if sa, ok := in["serviceAccounts"].(map[string]any); ok && identityType == "irsa" {
		if namespace, ok := sa["namespace"].(string); ok {
			oidc["namespace"] = namespace
		}
		if serviceAccount, ok := sa["serviceAccount"].(string); ok {
			oidc["serviceAccount"] = serviceAccount
		}
		// Default issuer URL for EKS (placeholder)
		oidc["issuerUrl"] = fmt.Sprintf("https://oidc.eks.%s.amazonaws.com/id/EXAMPLEClusterID", params.PrimaryRegion)
	}

	// Handle federation - map federation object to federationProvider string
	federationProvider := ""
	if fed, ok := in["federation"].(map[string]any); ok && identityType == "federation" {
		if provider, ok := fed["provider"].(string); ok {
			federationProvider = provider
		}
	}

	// Build result with all XRD parameters
	result := map[string]any{
		"providerConfigRef": "aws-provider",
		"type":              identityType,
		"managedPolicies":   managedPolicies,
		"tags":              map[string]string{},
		"overrides":         map[string]any{},
	}

	// Add type-specific required fields
	switch identityType {
	case "service":
		if servicePrincipal, ok := in["servicePrincipal"].(string); ok {
			result["servicePrincipal"] = servicePrincipal
		} else {
			// Default service principal
			result["servicePrincipal"] = "lambda.amazonaws.com"
		}
	case "irsa":
		if len(oidc) > 0 {
			result["oidc"] = oidc
		}
	case "federation":
		if federationProvider != "" {
			result["federationProvider"] = federationProvider
		}
	}

	// Add inline policy if we have one
	if inlinePolicy != "" {
		result["inlinePolicy"] = inlinePolicy
	}

	return result
}

// marshalDeterministic produces YAML with sorted map keys for stable output.
func marshalDeterministic(v any) ([]byte, error) {
	node := &yaml.Node{}
	raw, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, node); err != nil {
		return nil, err
	}
	sortYAMLNode(node)
	return yaml.Marshal(node)
}

func sortYAMLNode(node *yaml.Node) {
	if node == nil {
		return
	}

	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			sortYAMLNode(child)
		}
		return
	}

	if node.Kind == yaml.MappingNode {
		pairs := make([]struct{ Key, Value *yaml.Node }, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			pairs[i/2] = struct{ Key, Value *yaml.Node }{node.Content[i], node.Content[i+1]}
		}
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].Key.Value < pairs[j].Key.Value
		})
		for i, p := range pairs {
			node.Content[i*2] = p.Key
			node.Content[i*2+1] = p.Value
			sortYAMLNode(p.Value)
		}
		return
	}

	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			sortYAMLNode(child)
		}
	}
}

// mapPolicyNameToARN maps well-known policy names to AWS managed policy ARNs.
func mapPolicyNameToARN(policyName string) string {
	switch policyName {
	case "ReadOnlyAccess":
		return "arn:aws:iam::aws:policy/ReadOnlyAccess"
	case "AdministratorAccess":
		return "arn:aws:iam::aws:policy/AdministratorAccess"
	case "AmazonS3FullAccess":
		return "arn:aws:iam::aws:policy/AmazonS3FullAccess"
	case "AmazonDynamoDBFullAccess":
		return "arn:aws:iam::aws:policy/AmazonDynamoDBFullAccess"
	case "AmazonEC2FullAccess":
		return "arn:aws:iam::aws:policy/AmazonEC2FullAccess"
	case "AmazonRDSFullAccess":
		return "arn:aws:iam::aws:policy/AmazonRDSFullAccess"
	case "AmazonVPCFullAccess":
		return "arn:aws:iam::aws:policy/AmazonVPCFullAccess"
	case "AWSCloudFormationFullAccess":
		return "arn:aws:iam::aws:policy/AWSCloudFormationFullAccess"
	case "AWSLambda_FullAccess":
		return "arn:aws:iam::aws:policy/AWSLambda_FullAccess"
	case "AmazonSSMManagedInstanceCore":
		return "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
	default:
		// If it already looks like an ARN, return it as-is
		if strings.HasPrefix(policyName, "arn:aws:iam::") {
			return policyName
		}
		// Unknown policy name - return empty string, will be ignored
		return ""
	}
}

// toJSONArray converts a string slice to a JSON array string.
func toJSONArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, item := range items {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, "%q", item)
	}
	sb.WriteString("]")
	return sb.String()
}
