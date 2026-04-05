package server

import (
	"context"
	"slices"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	governanceplugin "github.com/maximhq/bifrost/plugins/governance"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"gorm.io/gorm"
)

type clusterConfigStatusStore struct {
	configstore.ConfigStore

	authConfig   *configstore.AuthConfig
	clientConfig *configstore.ClientConfig
}

func (s *clusterConfigStatusStore) GetAuthConfig(_ context.Context) (*configstore.AuthConfig, error) {
	return s.authConfig, nil
}

func (s *clusterConfigStatusStore) GetClientConfig(_ context.Context) (*configstore.ClientConfig, error) {
	return s.clientConfig, nil
}

func (s *clusterConfigStatusStore) GetFrameworkConfig(_ context.Context) (*configstoreTables.TableFrameworkConfig, error) {
	return nil, nil
}

func (s *clusterConfigStatusStore) GetProvidersConfig(_ context.Context) (map[schemas.ModelProvider]configstore.ProviderConfig, error) {
	return nil, nil
}

func (s *clusterConfigStatusStore) GetGovernanceConfig(_ context.Context) (*configstore.GovernanceConfig, error) {
	return nil, nil
}

func (s *clusterConfigStatusStore) GetMCPConfig(_ context.Context) (*schemas.MCPConfig, error) {
	return nil, nil
}

func (s *clusterConfigStatusStore) GetProxyConfig(_ context.Context) (*configstoreTables.GlobalProxyConfig, error) {
	return nil, nil
}

func (s *clusterConfigStatusStore) GetPlugins(_ context.Context) ([]*configstoreTables.TablePlugin, error) {
	return nil, nil
}

func (s *clusterConfigStatusStore) DB() *gorm.DB {
	return nil
}

func TestClusterConfigSyncReporterIncludesAuthDrift(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	handlers.SetLogger(bifrost.NewNoOpLogger())

	store := &clusterConfigStatusStore{
		authConfig: &configstore.AuthConfig{
			AdminUserName:          schemas.NewEnvVar("admin"),
			AdminPassword:          schemas.NewEnvVar("store-password"),
			IsEnabled:              false,
			DisableAuthOnInference: true,
		},
		clientConfig: &configstore.ClientConfig{},
	}

	authMiddleware, err := handlers.InitAuthMiddleware(store, nil)
	if err != nil {
		t.Fatalf("InitAuthMiddleware() error = %v", err)
	}
	authMiddleware.UpdateAuthConfig(&configstore.AuthConfig{
		AdminUserName:          schemas.NewEnvVar("admin"),
		AdminPassword:          schemas.NewEnvVar("runtime-password"),
		IsEnabled:              true,
		DisableAuthOnInference: false,
	})

	server := &BifrostHTTPServer{
		Ctx: schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		Config: &lib.Config{
			ClientConfig:     store.clientConfig,
			ConfigStore:      store,
			GovernanceConfig: &configstore.GovernanceConfig{},
		},
		AuthMiddleware: authMiddleware,
	}

	status := newClusterConfigSyncReporter(server).compute()
	if status.InSync == nil || *status.InSync {
		t.Fatalf("expected auth drift to mark runtime/store as out of sync, got %+v", status)
	}
	if !slices.Contains(status.DriftDomains, "auth") {
		t.Fatalf("expected auth drift domain, got %+v", status.DriftDomains)
	}
	if status.RuntimeHash == "" || status.StoreHash == "" {
		t.Fatalf("expected runtime/store hashes to be populated, got %+v", status)
	}
}

func TestClusterConfigSyncReporterUsesAuthMiddlewareRuntimeSnapshot(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	handlers.SetLogger(bifrost.NewNoOpLogger())

	authConfig := &configstore.AuthConfig{
		AdminUserName:          schemas.NewEnvVar("admin"),
		AdminPassword:          schemas.NewEnvVar("shared-password"),
		IsEnabled:              true,
		DisableAuthOnInference: false,
	}
	store := &clusterConfigStatusStore{
		authConfig:   authConfig,
		clientConfig: &configstore.ClientConfig{},
	}

	authMiddleware, err := handlers.InitAuthMiddleware(store, nil)
	if err != nil {
		t.Fatalf("InitAuthMiddleware() error = %v", err)
	}

	server := &BifrostHTTPServer{
		Ctx: schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		Config: &lib.Config{
			ClientConfig:     store.clientConfig,
			ConfigStore:      store,
			GovernanceConfig: &configstore.GovernanceConfig{},
		},
		AuthMiddleware: authMiddleware,
	}

	status := newClusterConfigSyncReporter(server).compute()
	if status.InSync == nil || !*status.InSync {
		t.Fatalf("expected auth config to be in sync, got %+v", status)
	}
	if slices.Contains(status.DriftDomains, "auth") {
		t.Fatalf("expected no auth drift domain, got %+v", status.DriftDomains)
	}
}

func TestHashRuntimeGovernanceDataCountsRoutingRulesAndModelConfigs(t *testing.T) {
	hash, counts, err := hashRuntimeGovernanceData(&governanceplugin.GovernanceData{
		RoutingRules: map[string]*configstoreTables.TableRoutingRule{
			"rule-1": {
				ID:            "rule-1",
				Name:          "Rule",
				Enabled:       true,
				CelExpression: "true",
				Scope:         "global",
				Priority:      1,
				Targets: []configstoreTables.TableRoutingTarget{
					{Weight: 1},
				},
			},
		},
		ModelConfigs: []*configstoreTables.TableModelConfig{
			{
				ID:        "model-config-1",
				ModelName: "gpt-4.1",
			},
		},
	})
	if err != nil {
		t.Fatalf("hashRuntimeGovernanceData() error = %v", err)
	}
	if hash == "" {
		t.Fatal("expected governance hash to be populated")
	}
	if counts.ModelConfigCount != 1 || counts.RoutingRuleCount != 1 {
		t.Fatalf("expected model/routing counts to be tracked, got %+v", counts)
	}
}
