package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

type clusterConfigHandlerStore struct {
	configstore.ConfigStore

	clientConfig         *configstore.ClientConfig
	frameworkConfig      *configstoreTables.TableFrameworkConfig
	authConfig           *configstore.AuthConfig
	configs              map[string]string
	flushSessionsCalled  bool
	restartRequiredValue *configstoreTables.RestartRequiredConfig
}

func (m *clusterConfigHandlerStore) UpdateClientConfig(_ context.Context, config *configstore.ClientConfig) error {
	if config == nil {
		m.clientConfig = nil
		return nil
	}
	cloned := *config
	m.clientConfig = &cloned
	return nil
}

func (m *clusterConfigHandlerStore) GetFrameworkConfig(_ context.Context) (*configstoreTables.TableFrameworkConfig, error) {
	return m.frameworkConfig, nil
}

func (m *clusterConfigHandlerStore) GetAuthConfig(_ context.Context) (*configstore.AuthConfig, error) {
	return m.authConfig, nil
}

func (m *clusterConfigHandlerStore) FlushSessions(_ context.Context) error {
	m.flushSessionsCalled = true
	return nil
}

func (m *clusterConfigHandlerStore) SetRestartRequiredConfig(_ context.Context, config *configstoreTables.RestartRequiredConfig) error {
	m.restartRequiredValue = config
	return nil
}

func (m *clusterConfigHandlerStore) UpdateConfig(_ context.Context, row *configstoreTables.TableGovernanceConfig, _ ...*gorm.DB) error {
	if row == nil {
		return nil
	}
	if m.configs == nil {
		m.configs = make(map[string]string)
	}
	m.configs[row.Key] = row.Value
	return nil
}

func (m *clusterConfigHandlerStore) GetConfig(_ context.Context, key string) (*configstoreTables.TableGovernanceConfig, error) {
	if m == nil || len(m.configs) == 0 {
		return nil, configstore.ErrNotFound
	}
	value, ok := m.configs[key]
	if !ok {
		return nil, configstore.ErrNotFound
	}
	return &configstoreTables.TableGovernanceConfig{Key: key, Value: value}, nil
}

type fakeConfigHandlerManager struct {
	store                  *clusterConfigHandlerStore
	lastLoadBalancerConfig *enterprisecfg.LoadBalancerConfig
}

func (m *fakeConfigHandlerManager) UpdateAuthConfig(_ context.Context, authConfig *configstore.AuthConfig) error {
	if authConfig == nil {
		m.store.authConfig = nil
		return nil
	}
	cloned := *authConfig
	m.store.authConfig = &cloned
	return nil
}

func (m *fakeConfigHandlerManager) ReloadClientConfigFromConfigStore(_ context.Context) error {
	return nil
}
func (m *fakeConfigHandlerManager) ReloadPricingManager(_ context.Context) error       { return nil }
func (m *fakeConfigHandlerManager) ForceReloadPricing(_ context.Context) error         { return nil }
func (m *fakeConfigHandlerManager) UpdateDropExcessRequests(_ context.Context, _ bool) {}
func (m *fakeConfigHandlerManager) UpdateMCPToolManagerConfig(_ context.Context, _ int, _ int, _ string) error {
	return nil
}
func (m *fakeConfigHandlerManager) ReloadPlugin(_ context.Context, _ string, _ *string, _ any, _ *schemas.PluginPlacement, _ *int) error {
	return nil
}
func (m *fakeConfigHandlerManager) RemovePlugin(_ context.Context, _ string) error { return nil }
func (m *fakeConfigHandlerManager) ReloadProxyConfig(_ context.Context, _ *configstoreTables.GlobalProxyConfig) error {
	return nil
}
func (m *fakeConfigHandlerManager) ReloadHeaderFilterConfig(_ context.Context, _ *configstoreTables.GlobalHeaderFilterConfig) error {
	return nil
}
func (m *fakeConfigHandlerManager) ReloadLoadBalancerConfig(_ context.Context, cfg *enterprisecfg.LoadBalancerConfig) error {
	if m == nil {
		return nil
	}
	m.lastLoadBalancerConfig = enterprisecfg.CloneLoadBalancerConfig(cfg)
	return nil
}

type fakeClusterConfigPropagator struct {
	changes []*ClusterConfigChange
}

func (p *fakeClusterConfigPropagator) PropagateClusterConfigChange(_ context.Context, change *ClusterConfigChange) error {
	if p == nil || change == nil {
		return nil
	}
	cloned := *change
	p.changes = append(p.changes, &cloned)
	return nil
}

func TestUpdateConfigPropagatesAuthConfigWithoutSessionFlushWhenOnlyInferenceFlagChanges(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := &clusterConfigHandlerStore{
		authConfig: &configstore.AuthConfig{
			AdminUserName:          schemas.NewEnvVar("admin"),
			AdminPassword:          schemas.NewEnvVar("stored-hash"),
			IsEnabled:              true,
			DisableAuthOnInference: false,
		},
	}
	manager := &fakeConfigHandlerManager{store: store}
	propagator := &fakeClusterConfigPropagator{}
	handler := NewConfigHandler(manager, &lib.Config{
		ConfigStore: store,
		ClientConfig: &configstore.ClientConfig{
			LogRetentionDays: 30,
		},
	}, propagator)

	body, err := json.Marshal(map[string]any{
		"client_config": map[string]any{
			"log_retention_days": 30,
		},
		"framework_config": map[string]any{},
		"auth_config": map[string]any{
			"admin_username": map[string]any{
				"value":    "admin",
				"env_var":  "",
				"from_env": false,
			},
			"admin_password": map[string]any{
				"value":    "<redacted>",
				"env_var":  "",
				"from_env": false,
			},
			"is_enabled":                true,
			"disable_auth_on_inference": true,
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPut)
	ctx.Request.SetRequestURI("/api/config")
	ctx.Request.SetBody(body)

	handler.updateConfig(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if store.flushSessionsCalled {
		t.Fatal("expected existing sessions to remain when only disable_auth_on_inference changes")
	}

	var authChange *ClusterConfigChange
	for _, change := range propagator.changes {
		if change.Scope == ClusterConfigScopeAuth {
			authChange = change
			break
		}
	}
	if authChange == nil {
		t.Fatalf("expected auth config propagation, got %+v", propagator.changes)
	}
	if authChange.FlushSessions {
		t.Fatalf("expected propagated auth change not to request session flush, got %+v", authChange)
	}
	if authChange.AuthConfig == nil || !authChange.AuthConfig.IsEnabled || !authChange.AuthConfig.DisableAuthOnInference {
		t.Fatalf("expected propagated auth config to include latest flags, got %+v", authChange.AuthConfig)
	}
	if authChange.AuthConfig.AdminPassword == nil || authChange.AuthConfig.AdminPassword.GetValue() != "stored-hash" {
		t.Fatalf("expected redacted password to resolve to stored hash, got %+v", authChange.AuthConfig.AdminPassword)
	}
}

func TestUpdateConfigReloadsAndPropagatesLoadBalancerConfig(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := &clusterConfigHandlerStore{}
	manager := &fakeConfigHandlerManager{store: store}
	propagator := &fakeClusterConfigPropagator{}
	handler := NewConfigHandler(manager, &lib.Config{
		ConfigStore: store,
		ClientConfig: &configstore.ClientConfig{
			LogRetentionDays: 30,
		},
	}, propagator)

	body, err := json.Marshal(map[string]any{
		"client_config": map[string]any{
			"log_retention_days": 30,
		},
		"framework_config": map[string]any{},
		"load_balancer_config": map[string]any{
			"enabled":                            true,
			"key_balancing_enabled":              true,
			"direction_routing_enabled":          false,
			"direction_routing_for_virtual_keys": false,
			"provider_allowlist":                 []string{"openai", "vllm"},
			"model_allowlist":                    []string{"gpt-4o", "qwen-max"},
			"tracker_config": map[string]any{
				"minimum_samples":            20,
				"recompute_interval_seconds": 5,
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPut)
	ctx.Request.SetRequestURI("/api/config")
	ctx.Request.SetBody(body)

	handler.updateConfig(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if manager.lastLoadBalancerConfig == nil {
		t.Fatal("expected adaptive routing config reload to be invoked")
	}
	if !manager.lastLoadBalancerConfig.Enabled {
		t.Fatalf("expected adaptive routing to remain enabled, got %+v", manager.lastLoadBalancerConfig)
	}
	if manager.lastLoadBalancerConfig.DirectionRoutingEnabled == nil || *manager.lastLoadBalancerConfig.DirectionRoutingEnabled {
		t.Fatalf("expected direction routing to stay disabled by default for enterprise safety, got %+v", manager.lastLoadBalancerConfig)
	}
	if manager.lastLoadBalancerConfig.KeyBalancingEnabled == nil || !*manager.lastLoadBalancerConfig.KeyBalancingEnabled {
		t.Fatalf("expected key balancing to stay enabled, got %+v", manager.lastLoadBalancerConfig)
	}
	if len(manager.lastLoadBalancerConfig.ProviderAllowlist) != 2 || manager.lastLoadBalancerConfig.ProviderAllowlist[0] != "openai" {
		t.Fatalf("expected provider allowlist to be preserved, got %+v", manager.lastLoadBalancerConfig.ProviderAllowlist)
	}

	storedConfigRow, err := store.GetConfig(context.Background(), configstoreTables.ConfigLoadBalancerKey)
	if err != nil {
		t.Fatalf("GetConfig(load_balancer_config) error = %v", err)
	}
	if !strings.Contains(storedConfigRow.Value, "\"enabled\":true") {
		t.Fatalf("expected stored adaptive routing config payload, got %s", storedConfigRow.Value)
	}

	var loadBalancerChange *ClusterConfigChange
	for _, change := range propagator.changes {
		if change.Scope != ClusterConfigScopeLoadBalancer {
			continue
		}
		loadBalancerChange = change
		break
	}
	if loadBalancerChange == nil {
		t.Fatalf("expected adaptive routing config propagation, got %+v", propagator.changes)
	}
	if loadBalancerChange.LoadBalancerConfig == nil || !loadBalancerChange.LoadBalancerConfig.Enabled {
		t.Fatalf("expected propagated adaptive routing config, got %+v", loadBalancerChange.LoadBalancerConfig)
	}
	if loadBalancerChange.LoadBalancerConfig.DirectionRoutingEnabled == nil || *loadBalancerChange.LoadBalancerConfig.DirectionRoutingEnabled {
		t.Fatalf("expected propagated direction routing to remain disabled, got %+v", loadBalancerChange.LoadBalancerConfig)
	}
}
