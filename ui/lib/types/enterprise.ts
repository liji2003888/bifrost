export interface ClusterConfigSyncStatus {
	store_connected: boolean;
	store_kind?: string;
	runtime_hash?: string;
	store_hash?: string;
	in_sync?: boolean;
	drift_domains?: string[];
	provider_count?: number;
	virtual_key_count?: number;
	mcp_client_count?: number;
	plugin_count?: number;
	last_error?: string;
}

export interface ClusterPeerStatus {
	address: string;
	healthy: boolean;
	reported_healthy?: boolean;
	node_id?: string;
	started_at?: string;
	kv_keys?: number;
	discovery_peer_count?: number;
	config_sync?: ClusterConfigSyncStatus;
	last_seen?: string;
	last_error?: string;
	consecutive_successes: number;
	consecutive_failures: number;
}

export interface ClusterDiscoveryStatus {
	enabled: boolean;
	type?: string;
	last_refresh?: string;
	last_error?: string;
	peer_count: number;
}

export interface ClusterStatus {
	node_id: string;
	started_at: string;
	healthy: boolean;
	kv_keys: number;
	config_sync?: ClusterConfigSyncStatus;
	peers: ClusterPeerStatus[];
	discovery?: ClusterDiscoveryStatus;
}

export interface AdaptiveRouteStatus {
	node_id?: string;
	address?: string;
	source?: string;
	provider: string;
	model: string;
	key_id: string;
	samples: number;
	successes: number;
	failures: number;
	consecutive_failures: number;
	error_ewma: number;
	latency_ewma: number;
	last_updated: string;
}

export interface AdaptiveDirectionStatus {
	node_id?: string;
	address?: string;
	source?: string;
	provider: string;
	model: string;
	score: number;
	samples: number;
	successes: number;
	failures: number;
	consecutive_failures: number;
	error_ewma: number;
	latency_ewma: number;
	last_updated: string;
}

export interface AdaptiveRoutingStatusResponse {
	cluster: boolean;
	node_id?: string;
	routes: AdaptiveRouteStatus[];
	directions: AdaptiveDirectionStatus[];
	warnings?: ClusterAggregationWarning[];
}

export type AuditCategory = "authentication" | "configuration_change" | "data_access" | "export" | "cluster" | "security_event" | "system";

export interface AuditEvent {
	id: string;
	timestamp: string;
	category: AuditCategory;
	action: string;
	node_id?: string;
	address?: string;
	source?: string;
	resource_type?: string;
	resource_id?: string;
	actor_id?: string;
	method?: string;
	path?: string;
	status_code?: number;
	remote_addr?: string;
	request_id?: string;
	message?: string;
	metadata?: Record<string, unknown>;
	previous_hash?: string;
	integrity_hash?: string;
}

export interface AuditSearchResult {
	cluster: boolean;
	node_id?: string;
	events: AuditEvent[];
	total: number;
	warnings?: ClusterAggregationWarning[];
}

export type AlertSeverity = "info" | "warning" | "critical";

export interface AlertRecord {
	id: string;
	key: string;
	type: string;
	node_id?: string;
	address?: string;
	source?: string;
	severity: AlertSeverity;
	title: string;
	message: string;
	triggered_at: string;
	metadata?: Record<string, unknown>;
}

export interface AlertsResponse {
	cluster: boolean;
	node_id?: string;
	alerts: AlertRecord[];
	warnings?: ClusterAggregationWarning[];
}

export interface ClusterAggregationWarning {
	address: string;
	error: string;
}

export type ExportScope = "logs" | "mcp_logs";
export type ExportJobStatus = "pending" | "running" | "completed" | "failed";

export interface ExportJob {
	id: string;
	status: ExportJobStatus;
	scope: ExportScope;
	node_id?: string;
	address?: string;
	source?: string;
	format: string;
	compression?: string;
	file_path?: string;
	rows_exported: number;
	created_at: string;
	completed_at?: string;
	error?: string;
}

export interface LogExportsResponse {
	cluster: boolean;
	node_id?: string;
	jobs: ExportJob[];
	warnings?: ClusterAggregationWarning[];
}

export interface CreateLogExportRequest {
	format?: "jsonl" | "csv";
	compression?: "" | "gzip";
	max_rows?: number;
}

export interface VaultStatus {
	enabled: boolean;
	type?: string;
	last_sync?: string;
	last_error?: string;
	managed_secrets: number;
}
