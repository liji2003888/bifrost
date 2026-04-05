package enterprise

import "github.com/maximhq/bifrost/core/schemas"

type ClusterConfig struct {
	Enabled   bool                    `json:"enabled"`
	Region    string                  `json:"region,omitempty"`
	Peers     []string                `json:"peers,omitempty"`
	AuthToken *schemas.EnvVar         `json:"auth_token,omitempty"`
	Gossip    *ClusterGossipConfig    `json:"gossip,omitempty"`
	Discovery *ClusterDiscoveryConfig `json:"discovery,omitempty"`
}

type ClusterGossipConfig struct {
	Port   int                  `json:"port,omitempty"`
	Config *ClusterHealthConfig `json:"config,omitempty"`
}

type ClusterHealthConfig struct {
	TimeoutSeconds   int `json:"timeout_seconds,omitempty"`
	SuccessThreshold int `json:"success_threshold,omitempty"`
	FailureThreshold int `json:"failure_threshold,omitempty"`
}

type ClusterDiscoveryType string

const (
	ClusterDiscoveryKubernetes ClusterDiscoveryType = "kubernetes"
	ClusterDiscoveryDNS        ClusterDiscoveryType = "dns"
	ClusterDiscoveryUDP        ClusterDiscoveryType = "udp"
	ClusterDiscoveryConsul     ClusterDiscoveryType = "consul"
	ClusterDiscoveryEtcd       ClusterDiscoveryType = "etcd"
	ClusterDiscoveryMDNS       ClusterDiscoveryType = "mdns"
)

type ClusterDiscoveryConfig struct {
	Enabled             bool                 `json:"enabled,omitempty"`
	Type                ClusterDiscoveryType `json:"type,omitempty"`
	ServiceName         string               `json:"service_name,omitempty"`
	BindPort            int                  `json:"bind_port,omitempty"`
	DialTimeout         int64                `json:"dial_timeout,omitempty"`
	AllowedAddressSpace []string             `json:"allowed_address_space,omitempty"`
	K8sNamespace        string               `json:"k8s_namespace,omitempty"`
	K8sLabelSelector    string               `json:"k8s_label_selector,omitempty"`
	DNSNames            []string             `json:"dns_names,omitempty"`
	UDPBroadcastPort    int                  `json:"udp_broadcast_port,omitempty"`
	ConsulAddress       string               `json:"consul_address,omitempty"`
	EtcdEndpoints       []string             `json:"etcd_endpoints,omitempty"`
	MDNSService         string               `json:"mdns_service,omitempty"`
}

type LoadBalancerConfig struct {
	Enabled       bool                       `json:"enabled"`
	TrackerConfig *LoadBalancerTrackerConfig `json:"tracker_config,omitempty"`
	Bootstrap     *LoadBalancerBootstrap     `json:"bootstrap,omitempty"`
}

type LoadBalancerTrackerConfig struct {
	EWMAAlpha                 float64 `json:"ewma_alpha,omitempty"`
	ErrorPenalty              float64 `json:"error_penalty,omitempty"`
	LatencyPenalty            float64 `json:"latency_penalty,omitempty"`
	ConsecutiveFailurePenalty float64 `json:"consecutive_failure_penalty,omitempty"`
	MinimumSamples            int     `json:"minimum_samples,omitempty"`
	ExplorationRatio          float64 `json:"exploration_ratio,omitempty"`
	JitterRatio               float64 `json:"jitter_ratio,omitempty"`
	MinWeightMultiplier       float64 `json:"min_weight_multiplier,omitempty"`
	MaxWeightMultiplier       float64 `json:"max_weight_multiplier,omitempty"`
}

type LoadBalancerBootstrap struct {
	RouteMetrics     map[string]LoadBalancerRouteMetrics `json:"route_metrics,omitempty"`
	DirectionMetrics map[string]map[string]any           `json:"direction_metrics,omitempty"`
	Routes           map[string]map[string]any           `json:"routes,omitempty"`
}

type LoadBalancerRouteMetrics struct {
	ErrorRate           float64 `json:"error_rate,omitempty"`
	LatencyMs           float64 `json:"latency_ms,omitempty"`
	ConsecutiveFailures int64   `json:"consecutive_failures,omitempty"`
	SampleCount         int64   `json:"sample_count,omitempty"`
}

type AuditLogsConfig struct {
	Disabled      bool            `json:"disabled,omitempty"`
	HMACKey       *schemas.EnvVar `json:"hmac_key,omitempty"`
	RetentionDays int             `json:"retention_days,omitempty"`
}

type AlertsConfig struct {
	Enabled                   bool                 `json:"enabled"`
	EvaluationIntervalSeconds int                  `json:"evaluation_interval_seconds,omitempty"`
	LookbackMinutes           int                  `json:"lookback_minutes,omitempty"`
	MinimumRequests           int                  `json:"minimum_requests,omitempty"`
	ErrorRateThresholdPercent float64              `json:"error_rate_threshold_percent,omitempty"`
	AverageLatencyThresholdMs float64              `json:"average_latency_threshold_ms,omitempty"`
	BudgetThresholdsPercent   []float64            `json:"budget_thresholds_percent,omitempty"`
	Channels                  *AlertChannelsConfig `json:"channels,omitempty"`
}

type AlertChannelsConfig struct {
	Email   *EmailAlertConfig   `json:"email,omitempty"`
	Feishu  *FeishuAlertConfig  `json:"feishu,omitempty"`
	Webhook *WebhookAlertConfig `json:"webhook,omitempty"`
}

type EmailAlertConfig struct {
	Enabled  bool            `json:"enabled"`
	SMTPHost string          `json:"smtp_host,omitempty"`
	SMTPPort int             `json:"smtp_port,omitempty"`
	Username *schemas.EnvVar `json:"username,omitempty"`
	Password *schemas.EnvVar `json:"password,omitempty"`
	From     string          `json:"from,omitempty"`
	To       []string        `json:"to,omitempty"`
}

type FeishuAlertConfig struct {
	Enabled        bool            `json:"enabled"`
	WebhookURL     *schemas.EnvVar `json:"webhook_url,omitempty"`
	Secret         *schemas.EnvVar `json:"secret,omitempty"`
	MentionUserIDs []string        `json:"mention_user_ids,omitempty"`
}

type WebhookAlertConfig struct {
	Enabled        bool              `json:"enabled"`
	URL            *schemas.EnvVar   `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

type LogExportsConfig struct {
	Enabled              bool   `json:"enabled"`
	StoragePath          string `json:"storage_path,omitempty"`
	Format               string `json:"format,omitempty"`
	Compression          string `json:"compression,omitempty"`
	MaxRowsPerFile       int    `json:"max_rows_per_file,omitempty"`
	FlushIntervalSeconds int    `json:"flush_interval_seconds,omitempty"`
}

type VaultType string

const (
	VaultTypeHashicorp        VaultType = "hashicorp"
	VaultTypeAWSSecrets       VaultType = "aws_secrets_manager"
	VaultTypeGoogleSecret     VaultType = "google_secret_manager"
	VaultTypeAzureKeyVault    VaultType = "azure_key_vault"
	VaultTypeKubernetesSecret VaultType = "kubernetes"
)

type VaultConfig struct {
	Enabled              bool                       `json:"enabled"`
	Type                 VaultType                  `json:"type,omitempty"`
	SyncInterval         string                     `json:"sync_interval,omitempty"`
	SyncPaths            []string                   `json:"sync_paths,omitempty"`
	AutoDeprecate        bool                       `json:"auto_deprecate,omitempty"`
	BackupDeprecatedKeys bool                       `json:"backup_deprecated_keys,omitempty"`
	Deprecation          *VaultDeprecationConfig    `json:"deprecation,omitempty"`
	Hashicorp            *HashicorpVaultConfig      `json:"hashicorp,omitempty"`
	AWSSecretsManager    *AWSSecretsManagerConfig   `json:"aws_secrets_manager,omitempty"`
	GoogleSecretManager  *GoogleSecretManagerConfig `json:"google_secret_manager,omitempty"`
	AzureKeyVault        *AzureKeyVaultConfig       `json:"azure_key_vault,omitempty"`
	Kubernetes           *KubernetesSecretsConfig   `json:"kubernetes,omitempty"`
}

type VaultDeprecationConfig struct {
	GracePeriod    string `json:"grace_period,omitempty"`
	NotifyAdmins   bool   `json:"notify_admins,omitempty"`
	RetainArchived string `json:"retain_archived,omitempty"`
}

type HashicorpVaultConfig struct {
	Address       string          `json:"address,omitempty"`
	Token         *schemas.EnvVar `json:"token,omitempty"`
	Mount         string          `json:"mount,omitempty"`
	Namespace     string          `json:"namespace,omitempty"`
	TLSSkipVerify bool            `json:"tls_skip_verify,omitempty"`
	CACertPEM     string          `json:"ca_cert_pem,omitempty"`
}

type AWSSecretsManagerConfig struct {
	Region string `json:"region,omitempty"`
}

type GoogleSecretManagerConfig struct {
	ProjectID string `json:"project_id,omitempty"`
}

type AzureKeyVaultConfig struct {
	VaultURL     string          `json:"vault_url,omitempty"`
	TenantID     *schemas.EnvVar `json:"tenant_id,omitempty"`
	ClientID     *schemas.EnvVar `json:"client_id,omitempty"`
	ClientSecret *schemas.EnvVar `json:"client_secret,omitempty"`
}

type KubernetesSecretsConfig struct {
	Namespace     string `json:"namespace,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"`
}
