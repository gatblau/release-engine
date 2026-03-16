// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package resolve

// VMInstanceType maps (family, size, arch) to a concrete cloud instance type.
func VMInstanceType(family, size, arch string) string {
	archPrefix := "m"
	switch family {
	case "compute":
		archPrefix = "c"
	case "memory":
		archPrefix = "r"
	case "storage":
		archPrefix = "i"
	case "gpu":
		archPrefix = "p"
	}

	generation := "7"
	archSuffix := ""
	if arch == "arm64" {
		archSuffix = "g"
	}

	sizeMap := map[string]string{
		"small":   "large",
		"medium":  "xlarge",
		"large":   "2xlarge",
		"xlarge":  "4xlarge",
		"2xlarge": "8xlarge",
		"4xlarge": "16xlarge",
	}

	resolvedSize := sizeMap[size]
	if resolvedSize == "" {
		resolvedSize = "large"
	}

	return archPrefix + generation + archSuffix + "." + resolvedSize
}

// VMAMI resolves the AMI ID for a given OS family, image, arch, and region.
func VMAMI(osFamily, image, arch, region string) string {
	if image != "" {
		return image
	}
	return "resolve://" + osFamily + "/" + arch + "/" + region + "/latest"
}

// DefaultBootDiskGiB returns the default boot disk size for an OS family.
func DefaultBootDiskGiB(osFamily string) int {
	if osFamily == "windows" {
		return 100
	}
	return 30
}

// DiskTypeToCloud maps abstract disk types to cloud-specific types.
func DiskTypeToCloud(t string) string {
	switch t {
	case "provisioned":
		return "io2"
	case "standard", "ssd":
		return "gp3"
	default:
		return "gp3"
	}
}

// DBInstanceClass resolves a database instance class.
func DBInstanceClass(engine, tier string) string {
	_ = engine
	base := "db.r6g"
	if tier == "ha" {
		return base + ".xlarge"
	}
	return base + ".large"
}

// ReadReplicas returns read replica count based on availability.
func ReadReplicas(availability string) int {
	switch availability {
	case "critical":
		return 2
	case "high":
		return 1
	default:
		return 0
	}
}

// ServerlessMinACU returns min Aurora Capacity Units for serverless.
func ServerlessMinACU(workloadProfile string) float64 {
	switch workloadProfile {
	case "medium":
		return 2
	case "large":
		return 8
	case "xlarge":
		return 16
	default:
		return 0.5
	}
}

// ServerlessMaxACU returns max Aurora Capacity Units for serverless.
func ServerlessMaxACU(workloadProfile string) float64 {
	switch workloadProfile {
	case "medium":
		return 16
	case "large":
		return 64
	case "xlarge":
		return 128
	default:
		return 4
	}
}

// DefaultBackupRetention returns retention based on data classification.
func DefaultBackupRetention(classification string) int {
	switch classification {
	case "restricted":
		return 35
	case "confidential":
		return 14
	default:
		return 7
	}
}

// CacheNodeType resolves a cache node type from engine and profile.
func CacheNodeType(engine, workloadProfile string) string {
	_ = engine
	base := "cache.r7g"
	sizeMap := map[string]string{
		"small":  "medium",
		"medium": "large",
		"large":  "xlarge",
		"xlarge": "2xlarge",
	}
	resolvedSize := sizeMap[workloadProfile]
	if resolvedSize == "" {
		resolvedSize = "medium"
	}
	return base + "." + resolvedSize
}

// CacheEngineVersion returns the pinned engine version.
func CacheEngineVersion(engine, requestedVersion string) string {
	if requestedVersion != "" {
		return requestedVersion
	}
	defaults := map[string]string{
		"redis":     "7.2",
		"valkey":    "7.2",
		"memcached": "1.6",
	}
	v := defaults[engine]
	if v == "" {
		return "latest"
	}
	return v
}

// CacheShardCount returns shard count for clustered cache.
func CacheShardCount(engine, workloadProfile string) int {
	_ = engine
	switch workloadProfile {
	case "medium":
		return 3
	case "large":
		return 6
	case "xlarge":
		return 9
	default:
		return 2
	}
}

// KubernetesNodePool resolves node pool sizing.
func KubernetesNodePool(size, tier string) map[string]any {
	sizeMap := map[string]map[string]any{
		"small":  {"instanceType": "m7.large", "minNodes": 2, "maxNodes": 5},
		"medium": {"instanceType": "m7.xlarge", "minNodes": 3, "maxNodes": 10},
		"large":  {"instanceType": "m7.2xlarge", "minNodes": 3, "maxNodes": 20},
		"xlarge": {"instanceType": "m7.4xlarge", "minNodes": 5, "maxNodes": 50},
	}
	pool := sizeMap[size]
	if pool == nil {
		pool = map[string]any{"instanceType": "m7.large", "minNodes": 2, "maxNodes": 5}
	}
	if tier == "advanced" {
		pool["maxNodes"] = pool["maxNodes"].(int) * 2
	}
	return pool
}

// KubernetesVersion resolves the K8s version.
func KubernetesVersion(tier, requested string) string {
	_ = tier
	if requested != "" {
		return requested
	}
	return "1.30"
}
