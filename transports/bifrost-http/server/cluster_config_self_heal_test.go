package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

type authSelfHealStore struct {
	configstore.ConfigStore

	authConfig     *configstore.AuthConfig
	clientConfig   *configstore.ClientConfig
	updateAuthRuns int32
}

func (s *authSelfHealStore) GetAuthConfig(_ context.Context) (*configstore.AuthConfig, error) {
	return cloneAuthConfig(s.authConfig), nil
}

func (s *authSelfHealStore) GetClientConfig(_ context.Context) (*configstore.ClientConfig, error) {
	if s.clientConfig == nil {
		return &configstore.ClientConfig{}, nil
	}
	cloned := *s.clientConfig
	return &cloned, nil
}

func (s *authSelfHealStore) UpdateAuthConfig(_ context.Context, _ *configstore.AuthConfig) error {
	atomic.AddInt32(&s.updateAuthRuns, 1)
	return nil
}

func TestSupportedClusterConfigSelfHealDomainsOrdersAndDedupes(t *testing.T) {
	domains := supportedClusterConfigSelfHealDomains([]string{"governance", "providers", "auth", "providers", "mcp", "client"})
	expected := []string{"client", "auth", "providers", "governance"}
	if len(domains) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, domains)
	}
	for i := range expected {
		if domains[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, domains)
		}
	}
}

func TestClusterConfigSelfHealerRunOnceDeduplicatesSameSignature(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	var applyRuns int32
	status := enterprisecfg.ClusterConfigSyncStatus{
		StoreConnected: true,
		StoreHash:      "store-hash-1",
		InSync:         bifrost.Ptr(false),
		DriftDomains:   []string{"providers", "providers", "mcp"},
	}

	healer := &clusterConfigSelfHealer{
		cooldown: time.Hour,
		status: func() enterprisecfg.ClusterConfigSyncStatus {
			return status
		},
		apply: func(_ context.Context, domains []string) error {
			atomic.AddInt32(&applyRuns, 1)
			expected := []string{"providers"}
			if len(domains) != len(expected) || domains[0] != expected[0] {
				t.Fatalf("expected supported domains %v, got %v", expected, domains)
			}
			return nil
		},
	}

	healer.runOnce(context.Background())
	healer.runOnce(context.Background())
	if got := atomic.LoadInt32(&applyRuns); got != 1 {
		t.Fatalf("expected one self-heal apply for same signature, got %d", got)
	}

	status.StoreHash = "store-hash-2"
	healer.runOnce(context.Background())
	if got := atomic.LoadInt32(&applyRuns); got != 2 {
		t.Fatalf("expected a second self-heal apply after signature changed, got %d", got)
	}
}

func TestClusterConfigSelfHealerSkipsWhenInSync(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	var applyRuns int32
	healer := &clusterConfigSelfHealer{
		status: func() enterprisecfg.ClusterConfigSyncStatus {
			return enterprisecfg.ClusterConfigSyncStatus{
				StoreConnected: true,
				StoreHash:      "store-hash-1",
				InSync:         bifrost.Ptr(true),
				DriftDomains:   []string{"providers"},
			}
		},
		apply: func(_ context.Context, _ []string) error {
			atomic.AddInt32(&applyRuns, 1)
			return nil
		},
	}

	healer.runOnce(context.Background())
	if got := atomic.LoadInt32(&applyRuns); got != 0 {
		t.Fatalf("expected no self-heal apply when runtime is already in sync, got %d", got)
	}
}

func TestReloadAuthConfigFromConfigStoreUpdatesRuntimeOnlyAndIsIdempotent(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	handlers.SetLogger(bifrost.NewNoOpLogger())

	store := &authSelfHealStore{
		authConfig: &configstore.AuthConfig{
			AdminUserName:          schemas.NewEnvVar("admin"),
			AdminPassword:          schemas.NewEnvVar("store-password"),
			IsEnabled:              true,
			DisableAuthOnInference: false,
		},
		clientConfig: &configstore.ClientConfig{},
	}

	authMiddleware, err := handlers.InitAuthMiddleware(store, nil)
	if err != nil {
		t.Fatalf("InitAuthMiddleware() error = %v", err)
	}
	authMiddleware.UpdateAuthConfig(&configstore.AuthConfig{
		AdminUserName:          schemas.NewEnvVar("runtime-admin"),
		AdminPassword:          schemas.NewEnvVar("runtime-password"),
		IsEnabled:              false,
		DisableAuthOnInference: true,
	})

	server := &BifrostHTTPServer{
		Ctx: schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		Config: &lib.Config{
			ConfigStore:      store,
			GovernanceConfig: &configstore.GovernanceConfig{},
		},
		AuthMiddleware: authMiddleware,
	}

	if err := server.ReloadAuthConfigFromConfigStore(context.Background()); err != nil {
		t.Fatalf("ReloadAuthConfigFromConfigStore(first) error = %v", err)
	}
	if err := server.ReloadAuthConfigFromConfigStore(context.Background()); err != nil {
		t.Fatalf("ReloadAuthConfigFromConfigStore(second) error = %v", err)
	}

	if got := atomic.LoadInt32(&store.updateAuthRuns); got != 0 {
		t.Fatalf("expected auth self-heal reload to avoid store writes, got %d update calls", got)
	}

	current := server.AuthMiddleware.CurrentAuthConfig()
	if current == nil {
		t.Fatal("expected auth middleware to hold the store auth config after self-heal")
	}
	if !current.AdminUserName.Equals(store.authConfig.AdminUserName) || !current.AdminPassword.Equals(store.authConfig.AdminPassword) {
		t.Fatalf("expected auth middleware config to match store config, got %+v want %+v", current, store.authConfig)
	}
	if current.IsEnabled != store.authConfig.IsEnabled || current.DisableAuthOnInference != store.authConfig.DisableAuthOnInference {
		t.Fatalf("expected auth middleware flags to match store config, got %+v want %+v", current, store.authConfig)
	}

	if server.Config.GovernanceConfig == nil || server.Config.GovernanceConfig.AuthConfig == nil {
		t.Fatal("expected governance auth config snapshot to be refreshed from store")
	}
	if !server.Config.GovernanceConfig.AuthConfig.AdminUserName.Equals(store.authConfig.AdminUserName) {
		t.Fatalf("expected governance auth config username to match store")
	}
}
