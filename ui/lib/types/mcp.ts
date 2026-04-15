import { Function as ToolFunction } from "./logs";
import { EnvVar } from "./schemas";

export type MCPConnectionType = "http" | "stdio" | "sse";

export type MCPConnectionState = "connected" | "disconnected" | "error";

export type MCPAuthType = "none" | "headers" | "oauth";

export type { EnvVar };

export interface MCPStdioConfig {
	command: string;
	args: string[];
	envs: string[];
}

export interface OAuthConfig {
	client_id: string;
	client_secret?: string; // Optional for public clients using PKCE
	authorize_url?: string; // Optional, will be discovered from server_url if not provided
	token_url?: string; // Optional, will be discovered from server_url if not provided
	registration_url?: string; // Optional, for dynamic client registration
	scopes?: string[]; // Optional, can be discovered
	server_url?: string; // MCP server URL for OAuth discovery (automatically set from connection_string)
}

export interface MCPClientConfig {
	client_id: string; // Maps to ClientID in TableMCPClient
	name: string;
	is_code_mode_client?: boolean;
	connection_type: MCPConnectionType;
	connection_string?: EnvVar;
	stdio_config?: MCPStdioConfig;
	auth_type?: MCPAuthType;
	oauth_config_id?: string;
	tools_to_execute?: string[];
	tools_to_auto_execute?: string[];
	headers?: Record<string, EnvVar>;
	is_ping_available?: boolean;
	tool_pricing?: Record<string, number>;
	tool_sync_interval?: number; // Per-client override in minutes (0 = use global, -1 = disabled)
}

export interface MCPClient {
	config: MCPClientConfig;
	tools: ToolFunction[];
	state: MCPConnectionState;
	tool_snapshot_source?: "live" | "persisted" | "none";
	tool_name_mapping?: Record<string, string>;
}

export interface MCPHostedTool {
	id: number;
	tool_id: string;
	name: string;
	description?: string;
	method: string;
	url: string;
	headers?: Record<string, string>;
	query_params?: Record<string, string>;
	auth_profile?: {
		mode: "none" | "bearer_passthrough" | "header_passthrough";
		header_mappings?: Record<string, string>;
	};
	execution_profile?: {
		timeout_seconds?: number;
		max_response_body_bytes?: number;
	};
	response_schema?: Record<string, any>;
	response_examples?: any[];
	body_template?: string;
	response_json_path?: string;
	response_template?: string;
	created_at: string;
	updated_at: string;
	tool_schema: {
		type?: string;
		function?: {
			name?: string;
			description?: string;
			parameters?: {
				type?: string;
				description?: string;
				properties?: Record<string, any>;
				required?: string[];
			};
		};
	};
}

export interface CreateMCPClientRequest {
	name: string;
	is_code_mode_client?: boolean;
	connection_type: MCPConnectionType;
	connection_string?: EnvVar;
	stdio_config?: MCPStdioConfig;
	auth_type?: MCPAuthType;
	oauth_config?: OAuthConfig;
	tools_to_execute?: string[];
	tools_to_auto_execute?: string[];
	headers?: Record<string, EnvVar>;
	is_ping_available?: boolean;
}

export interface ValidateMCPClientRequest {
	name: string;
	connection_type: MCPConnectionType;
	connection_string?: EnvVar;
	auth_type?: MCPAuthType;
	headers?: Record<string, EnvVar>;
}

export interface ValidateMCPClientResponse {
	status: "compatible" | "unverified" | "incompatible";
	message: string;
	reason?: string;
	discovered_tools?: string[];
}

export interface CreateMCPHostedToolRequest {
	name: string;
	description?: string;
	method: string;
	url: string;
	headers?: Record<string, string>;
	query_params?: Record<string, string>;
	auth_profile?: MCPHostedTool["auth_profile"];
	execution_profile?: MCPHostedTool["execution_profile"];
	body_template?: string;
	response_json_path?: string;
	response_template?: string;
	response_schema?: MCPHostedTool["response_schema"];
	response_examples?: MCPHostedTool["response_examples"];
	tool_schema?: MCPHostedTool["tool_schema"];
}

export interface UpdateMCPHostedToolRequest extends CreateMCPHostedToolRequest {}

export interface GetMCPHostedToolsResponse {
	tools: MCPHostedTool[];
	count: number;
}

export interface PreviewMCPHostedToolRequest {
	args?: Record<string, any>;
}

export interface PreviewMCPHostedToolResult {
	output: string;
	status_code: number;
	latency_ms: number;
	response_bytes: number;
	content_type?: string;
	resolved_url?: string;
	truncated?: boolean;
	response_schema?: Record<string, any>;
}

export interface PreviewMCPHostedToolResponse {
	status: "success";
	preview: PreviewMCPHostedToolResult;
}

export interface OAuthFlowResponse {
	status: "pending_oauth";
	message: string;
	oauth_config_id: string;
	authorize_url: string;
	status_url?: string;
	complete_url?: string;
	next_steps?: string[];
	expires_at: string;
	mcp_client_id: string;
}

export interface OAuthStatusResponse {
	id: string;
	status: "pending" | "authorized" | "failed" | "expired" | "revoked";
	created_at: string;
	expires_at: string;
	token_id?: string;
	token_expires_at?: string;
	token_scopes?: string;
}

export interface MCPAuthConfigClientSummary {
	client_id: string;
	name: string;
	state?: MCPConnectionState;
	auth_type?: MCPAuthType;
}

export interface MCPAuthConfigRecord {
	id: string;
	status: "pending" | "authorized" | "failed" | "expired" | "revoked" | string;
	client_id: string;
	authorize_url: string;
	token_url: string;
	registration_url?: string;
	redirect_uri: string;
	server_url: string;
	use_discovery: boolean;
	created_at: string;
	updated_at: string;
	expires_at: string;
	scopes?: string[];
	token_id?: string;
	token_expires_at?: string;
	token_scopes?: string[];
	linked_mcp_client?: MCPAuthConfigClientSummary;
	pending_mcp_client?: MCPAuthConfigClientSummary;
	status_url: string;
	complete_url: string;
	next_steps?: string[];
}

export interface GetMCPAuthConfigsParams {
	limit?: number;
	offset?: number;
	search?: string;
	status?: string;
}

export interface GetMCPAuthConfigsResponse {
	configs: MCPAuthConfigRecord[];
	count: number;
	total_count: number;
	limit: number;
	offset: number;
}

export interface UpdateMCPClientRequest {
	name?: string;
	is_code_mode_client?: boolean;
	headers?: Record<string, EnvVar>;
	tools_to_execute?: string[];
	tools_to_auto_execute?: string[];
	is_ping_available?: boolean;
	tool_pricing?: Record<string, number>;
	tool_sync_interval?: number; // Per-client override in minutes (0 = use global, -1 = disabled)
}

// Pagination params for MCP clients list
export interface GetMCPClientsParams {
	limit?: number;
	offset?: number;
	search?: string;
}

// Paginated response for MCP clients list
export interface GetMCPClientsResponse {
	clients: MCPClient[];
	count: number;
	total_count: number;
	limit: number;
	offset: number;
}

// Types for MCP Tool Selector component
export interface SelectedTool {
	mcpClientId: string;
	toolName: string;
}

// MCP Tool Spec for tool groups (matches backend schema)
export interface MCPToolSpec {
	mcp_client_id: string;
	tool_names: string[];
}
