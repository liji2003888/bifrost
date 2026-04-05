package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fasthttp/router"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/kvstore"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

func TestPropagateClusterConfigChangeBroadcastsPayloadToPeers(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	received := make(chan handlers.ClusterConfigChange, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != handlers.ClusterConfigReloadEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get(enterprisecfg.ClusterAuthHeader); got != "cluster-secret" {
			t.Fatalf("expected cluster auth header, got %q", got)
		}

		var change handlers.ClusterConfigChange
		if err := json.NewDecoder(r.Body).Decode(&change); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		received <- change
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		Peers:     []string{server.URL},
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, store, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer cluster.Close()

	s := &BifrostHTTPServer{
		ClusterService: cluster,
	}
	change := &handlers.ClusterConfigChange{
		Scope: handlers.ClusterConfigScopeProxy,
		ProxyConfig: &configstoreTables.GlobalProxyConfig{
			Enabled: true,
			Type:    "http",
			URL:     "http://proxy.internal:8080",
		},
	}

	if err := s.PropagateClusterConfigChange(context.Background(), change); err != nil {
		t.Fatalf("PropagateClusterConfigChange() error = %v", err)
	}

	select {
	case got := <-received:
		if got.Scope != handlers.ClusterConfigScopeProxy {
			t.Fatalf("expected proxy scope, got %+v", got)
		}
		if got.ProxyConfig == nil || got.ProxyConfig.URL != "http://proxy.internal:8080" || !got.ProxyConfig.Enabled {
			t.Fatalf("expected proxy config payload to be preserved, got %+v", got.ProxyConfig)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for propagated cluster config change")
	}
}

func TestPropagateClusterConfigChangeAppliesAuthConfigOnRemotePeer(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	handlers.SetLogger(bifrost.NewNoOpLogger())

	remoteStore := &authClusterConfigStore{
		clientConfig: &configstore.ClientConfig{},
	}
	remoteAuthMiddleware, err := handlers.InitAuthMiddleware(remoteStore, nil)
	if err != nil {
		t.Fatalf("InitAuthMiddleware() error = %v", err)
	}

	remoteKV, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New(remote) error = %v", err)
	}
	defer remoteKV.Close()

	remoteCluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, remoteKV, "remote-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService(remote) error = %v", err)
	}
	defer remoteCluster.Close()

	remoteServer := &BifrostHTTPServer{
		Ctx:            schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		Config:         &lib.Config{ConfigStore: remoteStore, ClientConfig: remoteStore.clientConfig},
		AuthMiddleware: remoteAuthMiddleware,
		ClusterService: remoteCluster,
	}

	remoteRouter := router.New()
	handlers.NewEnterpriseHandler(remoteCluster, nil, nil, nil, nil, nil, remoteServer).RegisterRoutes(remoteRouter)
	remoteHTTPServer := &fasthttp.Server{Handler: remoteRouter.Handler}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()
	defer remoteHTTPServer.Shutdown()

	go func() {
		_ = remoteHTTPServer.Serve(listener)
	}()

	localKV, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New(local) error = %v", err)
	}
	defer localKV.Close()

	localCluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		Peers:     []string{"http://" + listener.Addr().String()},
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, localKV, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService(local) error = %v", err)
	}
	defer localCluster.Close()

	localServer := &BifrostHTTPServer{
		ClusterService: localCluster,
	}

	change := &handlers.ClusterConfigChange{
		Scope: handlers.ClusterConfigScopeAuth,
		AuthConfig: &configstore.AuthConfig{
			AdminUserName:          schemas.NewEnvVar("admin"),
			AdminPassword:          schemas.NewEnvVar("stored-hash"),
			IsEnabled:              true,
			DisableAuthOnInference: false,
		},
		FlushSessions: true,
	}

	if err := localServer.PropagateClusterConfigChange(context.Background(), change); err != nil {
		t.Fatalf("PropagateClusterConfigChange() error = %v", err)
	}

	if remoteStore.authConfig == nil || !remoteStore.authConfig.IsEnabled {
		t.Fatalf("expected remote auth config to be persisted, got %+v", remoteStore.authConfig)
	}
	if !remoteStore.flushSessionsCalled {
		t.Fatal("expected remote peer to flush sessions after propagated auth change")
	}
	assertAPIMiddlewareRejectsWithoutAuth(t, remoteAuthMiddleware)
}

func TestPropagateClusterConfigChangeAppliesGovernanceResourcesOnRemotePeer(t *testing.T) {
	remoteServer, remoteStore, cleanup := newGovernanceClusterTestServer(t)
	defer cleanup()
	seedGovernanceTestProvider(t, remoteStore, "openai")

	remoteKV, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New(remote) error = %v", err)
	}
	defer remoteKV.Close()

	remoteCluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, remoteKV, "remote-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService(remote) error = %v", err)
	}
	defer remoteCluster.Close()

	remoteServer.ClusterService = remoteCluster
	remoteServer.Ctx = schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)

	remoteRouter := router.New()
	handlers.NewEnterpriseHandler(remoteCluster, nil, nil, nil, nil, nil, remoteServer).RegisterRoutes(remoteRouter)
	remoteHTTPServer := &fasthttp.Server{Handler: remoteRouter.Handler}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()
	defer remoteHTTPServer.Shutdown()

	go func() {
		_ = remoteHTTPServer.Serve(listener)
	}()

	localKV, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New(local) error = %v", err)
	}
	defer localKV.Close()

	localCluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		Peers:     []string{"http://" + listener.Addr().String()},
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, localKV, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService(local) error = %v", err)
	}
	defer localCluster.Close()

	localServer := &BifrostHTTPServer{
		ClusterService: localCluster,
	}

	customerBudgetID := "budget-propagated-customer"
	customer := &configstoreTables.TableCustomer{
		ID:       "customer-propagated",
		Name:     "Cluster Customer",
		BudgetID: bifrost.Ptr(customerBudgetID),
		Budget: &configstoreTables.TableBudget{
			ID:            customerBudgetID,
			MaxLimit:      100,
			ResetDuration: "1h",
			LastReset:     time.Unix(1700000000, 0).UTC(),
			CurrentUsage:  3.5,
		},
	}
	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:          handlers.ClusterConfigScopeCustomer,
		CustomerID:     customer.ID,
		CustomerConfig: customer,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(customer) error = %v", err)
	}

	customerTeamID := "team-propagated"
	team := &configstoreTables.TableTeam{
		ID:         customerTeamID,
		Name:       "Cluster Team",
		CustomerID: bifrost.Ptr(customer.ID),
	}
	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:      handlers.ClusterConfigScopeTeam,
		TeamID:     team.ID,
		TeamConfig: team,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(team) error = %v", err)
	}

	virtualKey := &configstoreTables.TableVirtualKey{
		ID:       "vk-propagated",
		Name:     "Cluster Virtual Key",
		Value:    "sk-bf-propagated",
		IsActive: true,
		TeamID:   bifrost.Ptr(team.ID),
	}
	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:            handlers.ClusterConfigScopeVirtualKey,
		VirtualKeyID:     virtualKey.ID,
		VirtualKeyConfig: virtualKey,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(virtual key) error = %v", err)
	}

	storedCustomer, err := remoteStore.GetCustomer(context.Background(), customer.ID)
	if err != nil {
		t.Fatalf("remote GetCustomer() error = %v", err)
	}
	if storedCustomer.Name != customer.Name {
		t.Fatalf("expected remote customer name %q, got %q", customer.Name, storedCustomer.Name)
	}

	storedTeam, err := remoteStore.GetTeam(context.Background(), team.ID)
	if err != nil {
		t.Fatalf("remote GetTeam() error = %v", err)
	}
	if storedTeam.CustomerID == nil || *storedTeam.CustomerID != customer.ID {
		t.Fatalf("expected remote team to reference customer %s, got %+v", customer.ID, storedTeam.CustomerID)
	}

	storedVirtualKey, err := remoteStore.GetVirtualKey(context.Background(), virtualKey.ID)
	if err != nil {
		t.Fatalf("remote GetVirtualKey() error = %v", err)
	}
	if storedVirtualKey.TeamID == nil || *storedVirtualKey.TeamID != team.ID || storedVirtualKey.Value != virtualKey.Value {
		t.Fatalf("unexpected remote virtual key: %+v", storedVirtualKey)
	}

	modelProvider := "openai"
	modelConfig := &configstoreTables.TableModelConfig{
		ID:        "model-config-propagated",
		ModelName: "gpt-4.1",
		Provider:  &modelProvider,
		BudgetID:  bifrost.Ptr("budget-model-config-propagated"),
		Budget: &configstoreTables.TableBudget{
			ID:            "budget-model-config-propagated",
			MaxLimit:      200,
			ResetDuration: "1h",
			LastReset:     time.Unix(1700000000, 0).UTC(),
			CurrentUsage:  9.5,
		},
	}
	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:         handlers.ClusterConfigScopeModelConfig,
		ModelConfigID: modelConfig.ID,
		ModelConfig:   modelConfig,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(model config) error = %v", err)
	}

	storedModelConfig, err := remoteStore.GetModelConfigByID(context.Background(), modelConfig.ID)
	if err != nil {
		t.Fatalf("remote GetModelConfigByID() error = %v", err)
	}
	if storedModelConfig.ModelName != modelConfig.ModelName || storedModelConfig.Provider == nil || *storedModelConfig.Provider != modelProvider {
		t.Fatalf("unexpected remote model config: %+v", storedModelConfig)
	}

	scopeID := team.ID
	routingRule := &configstoreTables.TableRoutingRule{
		ID:            "routing-rule-propagated",
		Name:          "Cluster Routing Rule",
		Enabled:       true,
		CelExpression: "true",
		Scope:         "team",
		ScopeID:       &scopeID,
		Priority:      10,
		Targets: []configstoreTables.TableRoutingTarget{
			{
				Provider: &modelProvider,
				Model:    bifrost.Ptr("gpt-4.1"),
				Weight:   1,
			},
		},
	}
	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:         handlers.ClusterConfigScopeRoutingRule,
		RoutingRuleID: routingRule.ID,
		RoutingRule:   routingRule,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(routing rule) error = %v", err)
	}

	storedRoutingRule, err := remoteStore.GetRoutingRule(context.Background(), routingRule.ID)
	if err != nil {
		t.Fatalf("remote GetRoutingRule() error = %v", err)
	}
	if storedRoutingRule.Scope != routingRule.Scope || storedRoutingRule.ScopeID == nil || *storedRoutingRule.ScopeID != scopeID || len(storedRoutingRule.Targets) != 1 {
		t.Fatalf("unexpected remote routing rule: %+v", storedRoutingRule)
	}

	providerGovernance := &configstoreTables.TableProvider{
		Name:     "openai",
		BudgetID: bifrost.Ptr("budget-provider-governance-propagated"),
		Budget: &configstoreTables.TableBudget{
			ID:            "budget-provider-governance-propagated",
			MaxLimit:      150,
			ResetDuration: "1h",
			LastReset:     time.Unix(1700000000, 0).UTC(),
			CurrentUsage:  7.25,
		},
	}
	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:              handlers.ClusterConfigScopeProviderGovernance,
		Provider:           "openai",
		ProviderGovernance: providerGovernance,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(provider governance create) error = %v", err)
	}

	storedProviderGovernance, err := remoteStore.GetProvider(context.Background(), "openai")
	if err != nil {
		t.Fatalf("remote GetProvider(provider governance create) error = %v", err)
	}
	if storedProviderGovernance.BudgetID == nil || *storedProviderGovernance.BudgetID != "budget-provider-governance-propagated" {
		t.Fatalf("unexpected remote provider governance after create: %+v", storedProviderGovernance)
	}

	clearedProviderGovernance := &configstoreTables.TableProvider{Name: "openai"}
	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:              handlers.ClusterConfigScopeProviderGovernance,
		Provider:           "openai",
		ProviderGovernance: clearedProviderGovernance,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(provider governance clear) error = %v", err)
	}

	storedClearedProviderGovernance, err := remoteStore.GetProvider(context.Background(), "openai")
	if err != nil {
		t.Fatalf("remote GetProvider(provider governance clear) error = %v", err)
	}
	if storedClearedProviderGovernance.BudgetID != nil || storedClearedProviderGovernance.RateLimitID != nil {
		t.Fatalf("unexpected remote provider governance after clear: %+v", storedClearedProviderGovernance)
	}
}

func TestPropagateClusterConfigChangeAppliesBuiltinPluginConfigOnRemotePeer(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	handlers.SetLogger(bifrost.NewNoOpLogger())

	remoteStore := newClusterPluginApplyStore(t)

	remoteKV, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New(remote) error = %v", err)
	}
	defer remoteKV.Close()

	remoteCluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, remoteKV, "remote-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService(remote) error = %v", err)
	}
	defer remoteCluster.Close()

	remoteServer := &BifrostHTTPServer{
		Ctx:            schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		Config:         &lib.Config{ConfigStore: remoteStore},
		ClusterService: remoteCluster,
	}

	remoteRouter := router.New()
	handlers.NewEnterpriseHandler(remoteCluster, nil, nil, nil, nil, nil, remoteServer).RegisterRoutes(remoteRouter)
	remoteHTTPServer := &fasthttp.Server{Handler: remoteRouter.Handler}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()
	defer remoteHTTPServer.Shutdown()

	go func() {
		_ = remoteHTTPServer.Serve(listener)
	}()

	localKV, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New(local) error = %v", err)
	}
	defer localKV.Close()

	localCluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		Peers:     []string{"http://" + listener.Addr().String()},
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, localKV, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService(local) error = %v", err)
	}
	defer localCluster.Close()

	localServer := &BifrostHTTPServer{
		ClusterService: localCluster,
	}

	pluginConfig := &configstoreTables.TablePlugin{
		Name:     "loadbalancer",
		Enabled:  false,
		IsCustom: false,
		Config: map[string]any{
			"latency_weight": 0.6,
		},
	}
	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:        handlers.ClusterConfigScopePlugin,
		PluginName:   "loadbalancer",
		PluginConfig: pluginConfig,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(plugin create/update) error = %v", err)
	}

	remotePlugin, err := remoteStore.GetPlugin(context.Background(), "loadbalancer")
	if err != nil {
		t.Fatalf("GetPlugin(loadbalancer) error = %v", err)
	}
	if remotePlugin.Enabled || remotePlugin.IsCustom || remotePlugin.Path != nil {
		t.Fatalf("unexpected remote plugin record: %+v", remotePlugin)
	}
	config, ok := remotePlugin.Config.(map[string]any)
	if !ok || config["latency_weight"] != 0.6 {
		t.Fatalf("expected remote plugin config to be preserved, got %+v", remotePlugin.Config)
	}

	if err := localServer.PropagateClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:      handlers.ClusterConfigScopePlugin,
		PluginName: "loadbalancer",
		Delete:     true,
	}); err != nil {
		t.Fatalf("PropagateClusterConfigChange(plugin delete) error = %v", err)
	}

	if _, err := remoteStore.GetPlugin(context.Background(), "loadbalancer"); !errors.Is(err, configstore.ErrNotFound) {
		t.Fatalf("expected propagated plugin delete to remove remote config, got err=%v", err)
	}
}
