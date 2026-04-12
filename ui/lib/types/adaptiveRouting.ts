/**
 * Adaptive Routing (Load Balancer) Type Definitions
 * Global system-level configuration for adaptive load balancing
 */

export interface AdaptiveRoutingTrackerConfig {
	ewma_alpha?: number;
	error_penalty?: number;
	latency_penalty?: number;
	consecutive_failure_penalty?: number;
	minimum_samples?: number;
	exploration_ratio?: number;
	jitter_ratio?: number;
	recompute_interval_seconds?: number;
	degraded_error_threshold?: number;
	failed_error_threshold?: number;
	weight_floor?: number;
	weight_ceiling?: number;
}

export interface AdaptiveRoutingConfig {
	enabled: boolean;
	key_balancing_enabled?: boolean;
	direction_routing_enabled?: boolean;
	direction_routing_for_virtual_keys?: boolean;
	provider_allowlist?: string[];
	model_allowlist?: string[];
	tracker_config?: AdaptiveRoutingTrackerConfig;
}

export const DEFAULT_ADAPTIVE_ROUTING_CONFIG: AdaptiveRoutingConfig = {
	enabled: false,
	key_balancing_enabled: true,
	direction_routing_enabled: false,
	direction_routing_for_virtual_keys: false,
	provider_allowlist: [],
	model_allowlist: [],
};
