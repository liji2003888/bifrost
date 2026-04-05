export interface ClusterPeerStatus {
	address: string;
	healthy: boolean;
	reported_healthy?: boolean;
	node_id?: string;
	started_at?: string;
	kv_keys?: number;
	discovery_peer_count?: number;
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
	peers: ClusterPeerStatus[];
	discovery?: ClusterDiscoveryStatus;
}

export interface AdaptiveRouteStatus {
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
	routes: AdaptiveRouteStatus[];
	directions: AdaptiveDirectionStatus[];
}

export type AuditCategory = "authentication" | "configuration_change" | "data_access" | "export" | "cluster" | "security_event" | "system";

export interface AuditEvent {
	id: string;
	timestamp: string;
	category: AuditCategory;
	action: string;
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
	events: AuditEvent[];
	total: number;
}

export type AlertSeverity = "info" | "warning" | "critical";

export interface AlertRecord {
	id: string;
	key: string;
	type: string;
	severity: AlertSeverity;
	title: string;
	message: string;
	triggered_at: string;
	metadata?: Record<string, unknown>;
}

export interface AlertsResponse {
	alerts: AlertRecord[];
}

export type ExportScope = "logs" | "mcp_logs";
export type ExportJobStatus = "pending" | "running" | "completed" | "failed";

export interface ExportJob {
	id: string;
	status: ExportJobStatus;
	scope: ExportScope;
	format: string;
	compression?: string;
	file_path?: string;
	rows_exported: number;
	created_at: string;
	completed_at?: string;
	error?: string;
}

export interface LogExportsResponse {
	jobs: ExportJob[];
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
