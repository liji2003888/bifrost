package handlers

import (
	"context"
	"encoding/json"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type clusterConfigHandlerStore struct {
	configstore.ConfigStore

	clientConfig         *configstore.ClientConfig
	frameworkConfig      *configstoreTables.TableFrameworkConfig
	authConfig           *configstore.AuthConfig
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

type fakeConfigHandlerManager struct {
	store *clusterConfigHandlerStore
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
