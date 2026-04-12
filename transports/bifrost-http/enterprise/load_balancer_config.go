package enterprise

import (
	"slices"
	"strings"

	"github.com/bytedance/sonic"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
)

func NormalizeLoadBalancerConfig(cfg *LoadBalancerConfig) *LoadBalancerConfig {
	normalized := &LoadBalancerConfig{
		Enabled:                        false,
		KeyBalancingEnabled:            boolPtr(true),
		DirectionRoutingEnabled:        boolPtr(false),
		DirectionRoutingForVirtualKeys: boolPtr(false),
	}
	if cfg == nil {
		return normalized
	}

	normalized.Enabled = cfg.Enabled
	if cfg.KeyBalancingEnabled != nil {
		normalized.KeyBalancingEnabled = boolPtr(*cfg.KeyBalancingEnabled)
	}
	if cfg.DirectionRoutingEnabled != nil {
		normalized.DirectionRoutingEnabled = boolPtr(*cfg.DirectionRoutingEnabled)
	}
	if cfg.DirectionRoutingForVirtualKeys != nil {
		normalized.DirectionRoutingForVirtualKeys = boolPtr(*cfg.DirectionRoutingForVirtualKeys)
	}
	if len(cfg.ProviderAllowlist) > 0 {
		normalized.ProviderAllowlist = slices.Clone(cfg.ProviderAllowlist)
	}
	if len(cfg.ModelAllowlist) > 0 {
		normalized.ModelAllowlist = slices.Clone(cfg.ModelAllowlist)
	}
	if cfg.TrackerConfig != nil {
		tracker := *cfg.TrackerConfig
		normalized.TrackerConfig = &tracker
	}
	if cfg.Bootstrap != nil {
		normalized.Bootstrap = cloneLoadBalancerBootstrap(cfg.Bootstrap)
	}
	return normalized
}

func CloneLoadBalancerConfig(cfg *LoadBalancerConfig) *LoadBalancerConfig {
	if cfg == nil {
		return nil
	}
	return NormalizeLoadBalancerConfig(cfg)
}

func cloneLoadBalancerBootstrap(bootstrap *LoadBalancerBootstrap) *LoadBalancerBootstrap {
	if bootstrap == nil {
		return nil
	}
	cloned := &LoadBalancerBootstrap{}
	if len(bootstrap.RouteMetrics) > 0 {
		cloned.RouteMetrics = make(map[string]LoadBalancerRouteMetrics, len(bootstrap.RouteMetrics))
		for key, value := range bootstrap.RouteMetrics {
			cloned.RouteMetrics[key] = value
		}
	}
	if len(bootstrap.DirectionMetrics) > 0 {
		cloned.DirectionMetrics = make(map[string]map[string]any, len(bootstrap.DirectionMetrics))
		for key, value := range bootstrap.DirectionMetrics {
			if value == nil {
				cloned.DirectionMetrics[key] = nil
				continue
			}
			nested := make(map[string]any, len(value))
			for nestedKey, nestedValue := range value {
				nested[nestedKey] = nestedValue
			}
			cloned.DirectionMetrics[key] = nested
		}
	}
	if len(bootstrap.Routes) > 0 {
		cloned.Routes = make(map[string]map[string]any, len(bootstrap.Routes))
		for key, value := range bootstrap.Routes {
			if value == nil {
				cloned.Routes[key] = nil
				continue
			}
			nested := make(map[string]any, len(value))
			for nestedKey, nestedValue := range value {
				nested[nestedKey] = nestedValue
			}
			cloned.Routes[key] = nested
		}
	}
	return cloned
}

func boolPtr(value bool) *bool {
	return &value
}

// RuleAdaptiveConfig defines the per-rule adaptive load balancing configuration
// stored in the routing rule's adaptive_config JSON field.
type RuleAdaptiveConfig struct {
	Enabled                        bool                       `json:"enabled"`
	KeyBalancingEnabled            *bool                      `json:"key_balancing_enabled,omitempty"`
	DirectionRoutingEnabled        *bool                      `json:"direction_routing_enabled,omitempty"`
	DirectionRoutingForVirtualKeys *bool                      `json:"direction_routing_for_virtual_keys,omitempty"`
	ProviderAllowlist              []string                   `json:"provider_allowlist,omitempty"`
	ModelAllowlist                 []string                   `json:"model_allowlist,omitempty"`
	TrackerConfig                  *LoadBalancerTrackerConfig `json:"tracker_config,omitempty"`
}

// AggregateAdaptiveRules merges all enabled adaptive routing rules into a single
// LoadBalancerConfig. The base config provides tracker tuning defaults; per-rule
// overrides control which providers/models/keys have adaptive routing active.
func AggregateAdaptiveRules(base *LoadBalancerConfig, rules []configstoreTables.TableRoutingRule) *LoadBalancerConfig {
	if base == nil {
		base = NormalizeLoadBalancerConfig(nil)
	}

	merged := CloneLoadBalancerConfig(base)

	providerSet := make(map[string]struct{})
	modelSet := make(map[string]struct{})

	hasAdaptiveRule := false
	anyKeyBalancing := false
	anyDirectionRouting := false
	anyDirectionVK := false

	for _, rule := range rules {
		if !rule.Enabled || rule.RuleType != "adaptive" {
			continue
		}

		var ruleCfg RuleAdaptiveConfig
		if rule.AdaptiveConfig != nil && strings.TrimSpace(*rule.AdaptiveConfig) != "" {
			if err := sonic.Unmarshal([]byte(*rule.AdaptiveConfig), &ruleCfg); err != nil {
				continue
			}
		} else if len(rule.ParsedAdaptiveConfig) > 0 {
			data, err := sonic.Marshal(rule.ParsedAdaptiveConfig)
			if err != nil {
				continue
			}
			if err := sonic.Unmarshal(data, &ruleCfg); err != nil {
				continue
			}
		} else {
			continue
		}

		if !ruleCfg.Enabled {
			continue
		}

		hasAdaptiveRule = true

		if ruleCfg.KeyBalancingEnabled == nil || *ruleCfg.KeyBalancingEnabled {
			anyKeyBalancing = true
		}
		if ruleCfg.DirectionRoutingEnabled != nil && *ruleCfg.DirectionRoutingEnabled {
			anyDirectionRouting = true
		}
		if ruleCfg.DirectionRoutingForVirtualKeys != nil && *ruleCfg.DirectionRoutingForVirtualKeys {
			anyDirectionVK = true
		}

		for _, p := range ruleCfg.ProviderAllowlist {
			if t := strings.TrimSpace(strings.ToLower(p)); t != "" {
				providerSet[t] = struct{}{}
			}
		}
		for _, m := range ruleCfg.ModelAllowlist {
			if t := strings.TrimSpace(m); t != "" {
				modelSet[t] = struct{}{}
			}
		}

		// Rule-level targets also contribute to allowlists
		for _, target := range rule.Targets {
			if target.Provider != nil {
				if t := strings.TrimSpace(strings.ToLower(*target.Provider)); t != "" {
					providerSet[t] = struct{}{}
				}
			}
		}

		// Use the first rule's tracker config as override if present
		if ruleCfg.TrackerConfig != nil && merged.TrackerConfig == nil {
			merged.TrackerConfig = ruleCfg.TrackerConfig
		}
	}

	merged.Enabled = hasAdaptiveRule
	merged.KeyBalancingEnabled = boolPtr(anyKeyBalancing)
	merged.DirectionRoutingEnabled = boolPtr(anyDirectionRouting)
	merged.DirectionRoutingForVirtualKeys = boolPtr(anyDirectionVK)

	if len(providerSet) > 0 {
		providers := make([]string, 0, len(providerSet))
		for p := range providerSet {
			providers = append(providers, p)
		}
		merged.ProviderAllowlist = providers
	} else {
		merged.ProviderAllowlist = nil
	}

	if len(modelSet) > 0 {
		models := make([]string, 0, len(modelSet))
		for m := range modelSet {
			models = append(models, m)
		}
		merged.ModelAllowlist = models
	} else {
		merged.ModelAllowlist = nil
	}

	return merged
}
