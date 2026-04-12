package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/maximhq/bifrost/transports/bifrost-http/loadbalancer"
)

func (s *BifrostHTTPServer) ReloadLoadBalancerConfig(ctx context.Context, cfg *enterprisecfg.LoadBalancerConfig) error {
	if s == nil || s.Config == nil {
		return fmt.Errorf("config not found")
	}

	normalized := enterprisecfg.NormalizeLoadBalancerConfig(cfg)
	s.Config.LoadBalancerConfig = normalized

	plugin, _ := lib.FindPluginAs[*loadbalancer.Plugin](s.Config, loadbalancer.PluginName)
	if plugin == nil {
		// Older runtimes may not have loaded the built-in wrapper yet. Instantiate it on demand.
		instantiated, err := InstantiatePlugin(ctx, loadbalancer.PluginName, nil, normalized, s.Config)
		if err != nil {
			return err
		}
		if err := s.SyncLoadedPlugin(ctx, loadbalancer.PluginName, instantiated, schemas.Ptr(schemas.PluginPlacementBuiltin), schemas.Ptr(4)); err != nil {
			return err
		}
		plugin, _ = lib.FindPluginAs[*loadbalancer.Plugin](s.Config, loadbalancer.PluginName)
	}
	if plugin == nil {
		return fmt.Errorf("adaptive routing plugin is not available")
	}
	if err := plugin.UpdateConfig(normalized); err != nil {
		return fmt.Errorf("failed to update adaptive routing config: %w", err)
	}
	if normalized.Enabled {
		if err := s.Config.UpdatePluginStatus(loadbalancer.PluginName, schemas.PluginStatusActive); err != nil && logger != nil {
			logger.Warn("failed to mark adaptive routing plugin active: %v", err)
		}
	} else {
		if err := s.markPluginDisabled(loadbalancer.PluginName); err != nil && logger != nil {
			logger.Warn("failed to mark adaptive routing plugin disabled: %v", err)
		}
	}
	return nil
}

func (s *BifrostHTTPServer) ReloadLoadBalancerConfigFromConfigStore(ctx context.Context) error {
	if s == nil || s.Config == nil {
		return fmt.Errorf("config not found")
	}
	cfg, err := loadLoadBalancerConfigFromStore(ctx, s.Config.ConfigStore)
	if err != nil {
		return err
	}
	return s.ReloadLoadBalancerConfig(ctx, cfg)
}

func (s *BifrostHTTPServer) ApplyClusterLoadBalancerConfig(ctx context.Context, cfg *enterprisecfg.LoadBalancerConfig) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config store not found")
	}
	if err := updateLoadBalancerConfigInStore(ctx, s.Config.ConfigStore, cfg); err != nil {
		return fmt.Errorf("failed to persist adaptive routing config: %w", err)
	}
	return s.ReloadLoadBalancerConfigFromConfigStore(ctx)
}

func loadLoadBalancerConfigFromStore(ctx context.Context, store configstore.ConfigStore) (*enterprisecfg.LoadBalancerConfig, error) {
	if store == nil {
		return enterprisecfg.NormalizeLoadBalancerConfig(nil), nil
	}

	row, err := store.GetConfig(ctx, configstoreTables.ConfigLoadBalancerKey)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			return enterprisecfg.NormalizeLoadBalancerConfig(nil), nil
		}
		return nil, err
	}

	var cfg enterprisecfg.LoadBalancerConfig
	if err := sonic.Unmarshal([]byte(row.Value), &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode adaptive routing config: %w", err)
	}
	return enterprisecfg.NormalizeLoadBalancerConfig(&cfg), nil
}

func updateLoadBalancerConfigInStore(ctx context.Context, store configstore.ConfigStore, cfg *enterprisecfg.LoadBalancerConfig) error {
	if store == nil {
		return nil
	}
	normalized := enterprisecfg.NormalizeLoadBalancerConfig(cfg)
	payload, err := sonic.MarshalString(normalized)
	if err != nil {
		return fmt.Errorf("failed to encode adaptive routing config: %w", err)
	}
	return store.UpdateConfig(ctx, &configstoreTables.TableGovernanceConfig{
		Key:   configstoreTables.ConfigLoadBalancerKey,
		Value: payload,
	})
}

// ReloadLoadBalancerFromAdaptiveRules aggregates all enabled adaptive routing rules
// and rebuilds the LoadBalancerConfig. Called after any routing rule create/update/delete.
func (s *BifrostHTTPServer) ReloadLoadBalancerFromAdaptiveRules(ctx context.Context) error {
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return nil
	}

	rules, err := s.Config.ConfigStore.GetRoutingRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to load routing rules: %w", err)
	}

	baseCfg, err := loadLoadBalancerConfigFromStore(ctx, s.Config.ConfigStore)
	if err != nil {
		baseCfg = enterprisecfg.NormalizeLoadBalancerConfig(nil)
	}

	merged := enterprisecfg.AggregateAdaptiveRules(baseCfg, rules)
	return s.ReloadLoadBalancerConfig(ctx, merged)
}
