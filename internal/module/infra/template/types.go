// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 gatblau

package template

// ProvisionParams is the canonical input model for infrastructure rendering.
// Phase 1 includes core/global fields and capability switches used by the
// engine and always-on policy fragments.
type ProvisionParams struct {
	ContractVersion    string            `yaml:"contract_version"`
	RequestName        string            `yaml:"request_name"`
	Tenant             string            `yaml:"tenant"`
	Owner              string            `yaml:"owner"`
	Environment        string            `yaml:"environment"`
	WorkloadProfile    string            `yaml:"workload_profile"`
	TemplateName       string            `yaml:"template_name"`
	CompositionRef     string            `yaml:"composition_ref"`
	Namespace          string            `yaml:"namespace"`
	Residency          string            `yaml:"residency"`
	PrimaryRegion      string            `yaml:"primary_region"`
	SecondaryRegion    string            `yaml:"secondary_region"`
	Availability       string            `yaml:"availability"`
	DataClassification string            `yaml:"data_classification"`
	Compliance         []string          `yaml:"compliance"`
	WorkloadType       string            `yaml:"workload_type"`
	WorkloadExposure   string            `yaml:"workload_exposure"`
	IngressMode        string            `yaml:"ingress_mode"`
	EgressMode         string            `yaml:"egress_mode"`
	DRRequired         bool              `yaml:"dr_required"`
	BackupRequired     bool              `yaml:"backup_required"`
	ExtraTags          map[string]string `yaml:"extra_tags"`

	// Capability switches (used in later phases and partial validation now)
	Kubernetes    KubernetesParams    `yaml:"kubernetes"`
	VM            VMParams            `yaml:"vm"`
	Database      DatabaseParams      `yaml:"database"`
	ObjectStore   ObjectStoreParams   `yaml:"object_store"`
	BlockStore    BlockStoreParams    `yaml:"block_store"`
	FileStore     FileStoreParams     `yaml:"file_store"`
	VPC           VPCParams           `yaml:"vpc"`
	Messaging     MessagingParams     `yaml:"messaging"`
	Cache         CacheParams         `yaml:"cache"`
	DNS           DNSParams           `yaml:"dns"`
	LoadBalancer  LoadBalancerParams  `yaml:"load_balancer"`
	CDN           CDNParams           `yaml:"cdn"`
	Identity      IdentityParams      `yaml:"identity"`
	Secrets       SecretsParams       `yaml:"secrets"`
	Observability ObservabilityParams `yaml:"observability"`
}

type KubernetesParams struct {
	Enabled       bool   `yaml:"enabled"`
	Tier          string `yaml:"tier"`
	Size          string `yaml:"size"`
	MultiAZ       bool   `yaml:"multi_az"`
	Version       string `yaml:"version"`
	NodePoolCount int    `yaml:"node_pool_count"`
}

type VMParams struct {
	Enabled         bool        `yaml:"enabled"`
	Count           int         `yaml:"count"`
	InstanceFamily  string      `yaml:"instance_family"`
	Size            string      `yaml:"size"`
	OSFamily        string      `yaml:"os_family"`
	OSImage         string      `yaml:"os_image"`
	Arch            string      `yaml:"arch"`
	SpotEnabled     bool        `yaml:"spot_enabled"`
	SpotMaxPrice    float64     `yaml:"spot_max_price"`
	PlacementGroup  string      `yaml:"placement_group"`
	SSHKeyName      string      `yaml:"ssh_key_name"`
	UserData        string      `yaml:"user_data"`
	BootDiskGiB     int         `yaml:"boot_disk_gib"`
	BootDiskType    string      `yaml:"boot_disk_type"`
	AdditionalDisks []VMDisk    `yaml:"additional_disks"`
	MultiAZ         bool        `yaml:"multi_az"`
	AutoScaling     VMAutoScale `yaml:"auto_scaling"`
}

type VMDisk struct {
	Name       string `yaml:"name"`
	SizeGiB    int    `yaml:"size_gib"`
	Type       string `yaml:"type"`
	IOPS       int    `yaml:"iops"`
	Throughput int    `yaml:"throughput"`
	MountPath  string `yaml:"mount_path"`
	Encrypted  bool   `yaml:"encrypted"`
}

type VMAutoScale struct {
	Enabled     bool   `yaml:"enabled"`
	MinCount    int    `yaml:"min_count"`
	MaxCount    int    `yaml:"max_count"`
	TargetCPU   int    `yaml:"target_cpu"`
	ScalePolicy string `yaml:"scale_policy"`
}

type DatabaseParams struct {
	Enabled             bool   `yaml:"enabled"`
	Engine              string `yaml:"engine"`
	Tier                string `yaml:"tier"`
	StorageGiB          int    `yaml:"storage_gib"`
	StorageType         string `yaml:"storage_type"`
	IOPS                int    `yaml:"iops"`
	BackupEnabled       bool   `yaml:"backup_enabled"`
	BackupRetentionDays int    `yaml:"backup_retention_days"`
	PointInTimeRecovery bool   `yaml:"point_in_time_recovery"`
	ParameterGroup      string `yaml:"parameter_group"`
	MaintenanceWindow   string `yaml:"maintenance_window"`
}

type ObjectStoreParams struct {
	Enabled       bool   `yaml:"enabled"`
	Class         string `yaml:"class"`
	Versioning    bool   `yaml:"versioning"`
	RetentionDays int    `yaml:"retention_days"`
	BucketCount   int    `yaml:"bucket_count"`
}

type BlockStoreParams struct {
	Enabled bool          `yaml:"enabled"`
	Volumes []BlockVolume `yaml:"volumes"`
}

type BlockVolume struct {
	Name             string `yaml:"name"`
	SizeGiB          int    `yaml:"size_gib"`
	Type             string `yaml:"type"`
	IOPS             int    `yaml:"iops"`
	Throughput       int    `yaml:"throughput"`
	MultiAZ          bool   `yaml:"multi_az"`
	Encrypted        bool   `yaml:"encrypted"`
	SnapshotSchedule string `yaml:"snapshot_schedule"`
}

type FileStoreParams struct {
	Enabled         bool   `yaml:"enabled"`
	PerformanceMode string `yaml:"performance_mode"`
	ThroughputMode  string `yaml:"throughput_mode"`
	ThroughputMiBs  int    `yaml:"throughput_mibs"`
	SizeGiB         int    `yaml:"size_gib"`
	Protocol        string `yaml:"protocol"`
	MultiAZ         bool   `yaml:"multi_az"`
}

type VPCParams struct {
	Enabled         bool      `yaml:"enabled"`
	CIDR            string    `yaml:"cidr"`
	PrivateSubnets  int       `yaml:"private_subnets"`
	PublicSubnets   int       `yaml:"public_subnets"`
	NATGateways     int       `yaml:"nat_gateways"`
	FlowLogs        bool      `yaml:"flow_logs"`
	TransitGateway  bool      `yaml:"transit_gateway"`
	PeeringRequests []VPCPeer `yaml:"peering_requests"`
}

type VPCPeer struct {
	PeerVPCID   string `yaml:"peer_vpc_id"`
	PeerAccount string `yaml:"peer_account"`
	PeerRegion  string `yaml:"peer_region"`
}

type MessagingParams struct {
	Enabled     bool   `yaml:"enabled"`
	Tier        string `yaml:"tier"`
	QueueCount  int    `yaml:"queue_count"`
	TopicCount  int    `yaml:"topic_count"`
	FIFO        bool   `yaml:"fifo"`
	Encryption  bool   `yaml:"encryption"`
	DLQEnabled  bool   `yaml:"dlq_enabled"`
	DLQMaxRetry int    `yaml:"dlq_max_retry"`
}

type CacheParams struct {
	Enabled               bool   `yaml:"enabled"`
	Engine                string `yaml:"engine"`
	Tier                  string `yaml:"tier"`
	NodeType              string `yaml:"node_type"`
	Version               string `yaml:"version"`
	ReplicaCount          int    `yaml:"replica_count"`
	SnapshotRetentionDays int    `yaml:"snapshot_retention_days"`
}

type DNSParams struct {
	Enabled  bool     `yaml:"enabled"`
	ZoneName string   `yaml:"zone_name"`
	Private  bool     `yaml:"private"`
	Records  []DNSRec `yaml:"records"`
}

type DNSRec struct {
	Name   string   `yaml:"name"`
	Type   string   `yaml:"type"`
	TTL    int      `yaml:"ttl"`
	Values []string `yaml:"values"`
}

type LoadBalancerParams struct {
	Enabled     bool          `yaml:"enabled"`
	Type        string        `yaml:"type"`
	Scheme      string        `yaml:"scheme"`
	HTTPS       bool          `yaml:"https"`
	WAF         bool          `yaml:"waf"`
	IdleTimeout int           `yaml:"idle_timeout"`
	HealthCheck LBHealthCheck `yaml:"health_check"`
}

type LBHealthCheck struct {
	Path               string `yaml:"path"`
	Port               int    `yaml:"port"`
	Protocol           string `yaml:"protocol"`
	IntervalSeconds    int    `yaml:"interval_seconds"`
	HealthyThreshold   int    `yaml:"healthy_threshold"`
	UnhealthyThreshold int    `yaml:"unhealthy_threshold"`
}

type CDNParams struct {
	Enabled        bool     `yaml:"enabled"`
	OriginType     string   `yaml:"origin_type"`
	PriceClass     string   `yaml:"price_class"`
	CachePolicyTTL int      `yaml:"cache_ttl"`
	CustomDomains  []string `yaml:"custom_domains"`
	WAF            bool     `yaml:"waf"`
}

type IdentityParams struct {
	Enabled             bool        `yaml:"enabled"`
	Type                string      `yaml:"type"`
	ServiceAccountCount int         `yaml:"service_account_count"`
	Policies            []IAMPolicy `yaml:"policies"`
	FederationProvider  string      `yaml:"federation_provider"`
	FederationAudience  string      `yaml:"federation_audience"`
}

type IAMPolicy struct {
	Effect     string            `yaml:"effect"`
	Actions    []string          `yaml:"actions"`
	Resource   string            `yaml:"resource"`
	Conditions map[string]string `yaml:"conditions"`
}

type SecretsParams struct {
	Enabled              bool   `yaml:"enabled"`
	Provider             string `yaml:"provider"`
	KMSKeyType           string `yaml:"kms_key_type"`
	KMSRotationDays      int    `yaml:"kms_rotation_days"`
	SecretCount          int    `yaml:"secret_count"`
	AutoRotation         bool   `yaml:"auto_rotation"`
	RotationIntervalDays int    `yaml:"rotation_interval_days"`
}

type ObservabilityParams struct {
	Enabled                bool     `yaml:"enabled"`
	MetricsRetentionDays   int      `yaml:"metrics_retention_days"`
	MetricsResolution      string   `yaml:"metrics_resolution"`
	CustomMetricNamespaces []string `yaml:"custom_metric_namespaces"`
	LogRetentionDays       int      `yaml:"log_retention_days"`
	LogSinkType            string   `yaml:"log_sink_type"`
	ExternalLogSink        string   `yaml:"external_log_sink"`
	TracingEnabled         bool     `yaml:"tracing_enabled"`
	TracingSampleRate      float64  `yaml:"tracing_sample_rate"`
	TracingProvider        string   `yaml:"tracing_provider"`
	DashboardEnabled       bool     `yaml:"dashboard_enabled"`
}
