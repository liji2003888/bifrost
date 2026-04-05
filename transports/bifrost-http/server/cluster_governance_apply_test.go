package server

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	governanceplugin "github.com/maximhq/bifrost/plugins/governance"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

type governanceClusterTestPlugin struct {
	store governanceplugin.GovernanceStore
}

func (p *governanceClusterTestPlugin) GetName() string { return governanceplugin.PluginName }
func (p *governanceClusterTestPlugin) Cleanup() error  { return nil }
func (p *governanceClusterTestPlugin) GetGovernanceStore() governanceplugin.GovernanceStore {
	return p.store
}
func (p *governanceClusterTestPlugin) HTTPTransportPreHook(ctx *schemas.BifrostContext, req *schemas.HTTPRequest) (*schemas.HTTPResponse, error) {
	return nil, nil
}
func (p *governanceClusterTestPlugin) HTTPTransportPostHook(ctx *schemas.BifrostContext, req *schemas.HTTPRequest, resp *schemas.HTTPResponse) error {
	return nil
}
func (p *governanceClusterTestPlugin) PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	return req, nil, nil
}
func (p *governanceClusterTestPlugin) PostLLMHook(ctx *schemas.BifrostContext, result *schemas.BifrostResponse, err *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	return result, err, nil
}
func (p *governanceClusterTestPlugin) PreMCPHook(ctx *schemas.BifrostContext, req *schemas.BifrostMCPRequest) (*schemas.BifrostMCPRequest, *schemas.MCPPluginShortCircuit, error) {
	return req, nil, nil
}
func (p *governanceClusterTestPlugin) PostMCPHook(ctx *schemas.BifrostContext, resp *schemas.BifrostMCPResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostMCPResponse, *schemas.BifrostError, error) {
	return resp, bifrostErr, nil
}

func newGovernanceClusterTestServer(t *testing.T) (*BifrostHTTPServer, configstore.ConfigStore, func()) {
	t.Helper()

	ctx := context.Background()
	logger := bifrost.NewNoOpLogger()
	SetLogger(logger)
	handlers.SetLogger(logger)

	store, err := configstore.NewConfigStore(ctx, &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config: &configstore.SQLiteConfig{
			Path: filepath.Join(t.TempDir(), "cluster-governance.db"),
		},
	}, logger)
	if err != nil {
		t.Fatalf("NewConfigStore() error = %v", err)
	}

	governanceStore, err := governanceplugin.NewLocalGovernanceStore(ctx, logger, store, nil, nil)
	if err != nil {
		t.Fatalf("NewLocalGovernanceStore() error = %v", err)
	}

	cfg := &lib.Config{ConfigStore: store}
	if err := cfg.ReloadPlugin(&governanceClusterTestPlugin{store: governanceStore}); err != nil {
		t.Fatalf("ReloadPlugin(governance) error = %v", err)
	}

	server := &BifrostHTTPServer{
		Ctx:    schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		Config: cfg,
	}

	cleanup := func() {
		_ = store.Close(context.Background())
	}

	return server, store, cleanup
}

func TestApplyClusterConfigChangeGovernanceLifecycle(t *testing.T) {
	server, store, cleanup := newGovernanceClusterTestServer(t)
	defer cleanup()

	ctx := context.Background()
	customerBudgetID := "budget-customer-1"
	customer := &configstoreTables.TableCustomer{
		ID:       "customer-1",
		Name:     "Acme Corp",
		BudgetID: bifrost.Ptr(customerBudgetID),
		Budget: &configstoreTables.TableBudget{
			ID:            customerBudgetID,
			MaxLimit:      150,
			ResetDuration: "1h",
			LastReset:     time.Unix(1700000000, 0).UTC(),
			CurrentUsage:  12.5,
		},
	}
	if err := server.ApplyClusterConfigChange(ctx, &handlers.ClusterConfigChange{
		Scope:          handlers.ClusterConfigScopeCustomer,
		CustomerID:     customer.ID,
		CustomerConfig: customer,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(customer create) error = %v", err)
	}

	storedCustomer, err := store.GetCustomer(ctx, customer.ID)
	if err != nil {
		t.Fatalf("GetCustomer() error = %v", err)
	}
	if storedCustomer.Name != customer.Name || storedCustomer.BudgetID == nil || *storedCustomer.BudgetID != customerBudgetID {
		t.Fatalf("unexpected stored customer: %+v", storedCustomer)
	}

	teamRateLimitID := "ratelimit-team-1"
	team := &configstoreTables.TableTeam{
		ID:          "team-1",
		Name:        "Platform",
		CustomerID:  bifrost.Ptr(customer.ID),
		RateLimitID: bifrost.Ptr(teamRateLimitID),
		RateLimit: &configstoreTables.TableRateLimit{
			ID:                   teamRateLimitID,
			RequestMaxLimit:      bifrost.Ptr(int64(50)),
			RequestResetDuration: bifrost.Ptr("1m"),
			RequestLastReset:     time.Unix(1700000000, 0).UTC(),
		},
	}
	if err := server.ApplyClusterConfigChange(ctx, &handlers.ClusterConfigChange{
		Scope:      handlers.ClusterConfigScopeTeam,
		TeamID:     team.ID,
		TeamConfig: team,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(team create) error = %v", err)
	}

	storedTeam, err := store.GetTeam(ctx, team.ID)
	if err != nil {
		t.Fatalf("GetTeam() error = %v", err)
	}
	if storedTeam.CustomerID == nil || *storedTeam.CustomerID != customer.ID {
		t.Fatalf("expected team to reference customer %s, got %+v", customer.ID, storedTeam.CustomerID)
	}

	virtualKeyBudgetID := "budget-vk-1"
	virtualKey := &configstoreTables.TableVirtualKey{
		ID:       "vk-1",
		Name:     "Ops Gateway",
		Value:    "sk-bf-cluster-test",
		IsActive: true,
		TeamID:   bifrost.Ptr(team.ID),
		BudgetID: bifrost.Ptr(virtualKeyBudgetID),
		Budget: &configstoreTables.TableBudget{
			ID:            virtualKeyBudgetID,
			MaxLimit:      25,
			ResetDuration: "30m",
			LastReset:     time.Unix(1700000000, 0).UTC(),
			CurrentUsage:  2.5,
		},
	}
	if err := server.ApplyClusterConfigChange(ctx, &handlers.ClusterConfigChange{
		Scope:            handlers.ClusterConfigScopeVirtualKey,
		VirtualKeyID:     virtualKey.ID,
		VirtualKeyConfig: virtualKey,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(virtual key create) error = %v", err)
	}

	storedVirtualKey, err := store.GetVirtualKey(ctx, virtualKey.ID)
	if err != nil {
		t.Fatalf("GetVirtualKey() error = %v", err)
	}
	if storedVirtualKey.TeamID == nil || *storedVirtualKey.TeamID != team.ID || storedVirtualKey.BudgetID == nil || *storedVirtualKey.BudgetID != virtualKeyBudgetID {
		t.Fatalf("unexpected stored virtual key: %+v", storedVirtualKey)
	}

	updatedVirtualKey := &configstoreTables.TableVirtualKey{
		ID:       virtualKey.ID,
		Name:     "Ops Gateway v2",
		Value:    virtualKey.Value,
		IsActive: false,
		TeamID:   bifrost.Ptr(team.ID),
	}
	if err := server.ApplyClusterConfigChange(ctx, &handlers.ClusterConfigChange{
		Scope:            handlers.ClusterConfigScopeVirtualKey,
		VirtualKeyID:     updatedVirtualKey.ID,
		VirtualKeyConfig: updatedVirtualKey,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(virtual key update) error = %v", err)
	}

	storedUpdatedVirtualKey, err := store.GetVirtualKey(ctx, updatedVirtualKey.ID)
	if err != nil {
		t.Fatalf("GetVirtualKey(updated) error = %v", err)
	}
	if storedUpdatedVirtualKey.Name != updatedVirtualKey.Name || storedUpdatedVirtualKey.IsActive || storedUpdatedVirtualKey.BudgetID != nil {
		t.Fatalf("unexpected updated virtual key: %+v", storedUpdatedVirtualKey)
	}
	if _, err := store.GetBudget(ctx, virtualKeyBudgetID); !errors.Is(err, configstore.ErrNotFound) {
		t.Fatalf("expected virtual key budget %s to be deleted, got err=%v", virtualKeyBudgetID, err)
	}

	if err := server.ApplyClusterConfigChange(ctx, &handlers.ClusterConfigChange{
		Scope:        handlers.ClusterConfigScopeVirtualKey,
		VirtualKeyID: virtualKey.ID,
		Delete:       true,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(virtual key delete) error = %v", err)
	}
	if _, err := store.GetVirtualKey(ctx, virtualKey.ID); !errors.Is(err, configstore.ErrNotFound) {
		t.Fatalf("expected virtual key to be deleted, got err=%v", err)
	}

	if err := server.ApplyClusterConfigChange(ctx, &handlers.ClusterConfigChange{
		Scope:  handlers.ClusterConfigScopeTeam,
		TeamID: team.ID,
		Delete: true,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(team delete) error = %v", err)
	}
	if _, err := store.GetTeam(ctx, team.ID); !errors.Is(err, configstore.ErrNotFound) {
		t.Fatalf("expected team to be deleted, got err=%v", err)
	}

	if err := server.ApplyClusterConfigChange(ctx, &handlers.ClusterConfigChange{
		Scope:      handlers.ClusterConfigScopeCustomer,
		CustomerID: customer.ID,
		Delete:     true,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(customer delete) error = %v", err)
	}
	if _, err := store.GetCustomer(ctx, customer.ID); !errors.Is(err, configstore.ErrNotFound) {
		t.Fatalf("expected customer to be deleted, got err=%v", err)
	}
}
