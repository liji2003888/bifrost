package enterprise

import (
	"slices"
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

