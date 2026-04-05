package server

import (
	"context"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	coremcp "github.com/maximhq/bifrost/core/mcp"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

type fakeClusterMCPManager struct {
	coremcp.MCPManagerInterface
	clients     map[string]*schemas.MCPClientConfig
	addCalls    int
	updateCalls int
	removeCalls int
}

func (m *fakeClusterMCPManager) GetClients() []schemas.MCPClientState {
	if m == nil {
		return nil
	}
	result := make([]schemas.MCPClientState, 0, len(m.clients))
	for _, cfg := range m.clients {
		result = append(result, schemas.MCPClientState{
			Name:            cfg.Name,
			ExecutionConfig: cfg,
			ToolMap:         map[string]schemas.ChatTool{},
			State:           schemas.MCPConnectionStateConnected,
		})
	}
	return result
}

func (m *fakeClusterMCPManager) AddClient(config *schemas.MCPClientConfig) error {
	if m.clients == nil {
		m.clients = map[string]*schemas.MCPClientConfig{}
	}
	cloned := *config
	m.clients[config.ID] = &cloned
	m.addCalls++
	return nil
}

func (m *fakeClusterMCPManager) RemoveClient(id string) error {
	delete(m.clients, id)
	m.removeCalls++
	return nil
}

func (m *fakeClusterMCPManager) UpdateClient(id string, updatedConfig *schemas.MCPClientConfig) error {
	if m.clients == nil {
		m.clients = map[string]*schemas.MCPClientConfig{}
	}
	cloned := *updatedConfig
	m.clients[id] = &cloned
	m.updateCalls++
	return nil
}

type fakeMCPClusterApplyStore struct {
	configstore.ConfigStore
	created []*schemas.MCPClientConfig
	updated []*configstoreTables.TableMCPClient
	deleted []string
	rows    map[string]*configstoreTables.TableMCPClient
}

func (s *fakeMCPClusterApplyStore) CreateMCPClientConfig(_ context.Context, clientConfig *schemas.MCPClientConfig) error {
	if s.rows == nil {
		s.rows = map[string]*configstoreTables.TableMCPClient{}
	}
	cloned := *clientConfig
	s.created = append(s.created, &cloned)
	s.rows[clientConfig.ID] = &configstoreTables.TableMCPClient{
		ClientID:           clientConfig.ID,
		Name:               clientConfig.Name,
		IsCodeModeClient:   clientConfig.IsCodeModeClient,
		ConnectionType:     string(clientConfig.ConnectionType),
		ConnectionString:   clientConfig.ConnectionString,
		StdioConfig:        clientConfig.StdioConfig,
		ToolsToExecute:     clientConfig.ToolsToExecute,
		ToolsToAutoExecute: clientConfig.ToolsToAutoExecute,
		Headers:            clientConfig.Headers,
		IsPingAvailable:    bifrost.Ptr(clientConfig.IsPingAvailable),
		ToolPricing:        clientConfig.ToolPricing,
		ToolSyncInterval:   int(clientConfig.ToolSyncInterval.Minutes()),
		AuthType:           string(clientConfig.AuthType),
		OauthConfigID:      clientConfig.OauthConfigID,
	}
	return nil
}

func (s *fakeMCPClusterApplyStore) UpdateMCPClientConfig(_ context.Context, id string, clientConfig *configstoreTables.TableMCPClient) error {
	if s.rows == nil {
		s.rows = map[string]*configstoreTables.TableMCPClient{}
	}
	cloned := *clientConfig
	s.updated = append(s.updated, &cloned)
	s.rows[id] = &cloned
	return nil
}

func (s *fakeMCPClusterApplyStore) DeleteMCPClientConfig(_ context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	delete(s.rows, id)
	return nil
}

func (s *fakeMCPClusterApplyStore) GetMCPClientByID(_ context.Context, id string) (*configstoreTables.TableMCPClient, error) {
	if s.rows == nil {
		return nil, nil
	}
	if row, ok := s.rows[id]; ok {
		cloned := *row
		return &cloned, nil
	}
	return nil, nil
}

func TestApplyClusterConfigChangeMCPClientLifecycle(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	manager := &fakeClusterMCPManager{
		clients: map[string]*schemas.MCPClientConfig{},
	}
	client := &bifrost.Bifrost{}
	client.SetMCPManager(manager)

	store := &fakeMCPClusterApplyStore{
		rows: map[string]*configstoreTables.TableMCPClient{},
	}
	cfg := &lib.Config{}
	cfg.SetBifrostClient(client)

	server := &BifrostHTTPServer{
		Config: cfg,
	}
	server.Config.ConfigStore = store

	baseConfig := &schemas.MCPClientConfig{
		ID:                 "client-1",
		Name:               "docs-client",
		ConnectionType:     schemas.MCPConnectionTypeHTTP,
		ConnectionString:   schemas.NewEnvVar("http://mcp.internal"),
		AuthType:           schemas.MCPAuthTypeNone,
		ToolsToExecute:     []string{"search"},
		ToolsToAutoExecute: []string{"search"},
		IsPingAvailable:    true,
		ToolSyncInterval:   5 * time.Minute,
	}

	if err := server.ApplyClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:           handlers.ClusterConfigScopeMCPClient,
		MCPClientID:     baseConfig.ID,
		MCPClientConfig: baseConfig,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(add) error = %v", err)
	}
	if manager.addCalls != 1 {
		t.Fatalf("expected one runtime add call, got %d", manager.addCalls)
	}
	if len(store.created) != 1 {
		t.Fatalf("expected one persisted create, got %+v", store.created)
	}
	if got, err := server.Config.GetMCPClient(baseConfig.ID); err != nil || got.Name != "docs-client" {
		t.Fatalf("expected added mcp client in runtime config, got %+v err=%v", got, err)
	}

	updatedConfig := &schemas.MCPClientConfig{
		ID:                 baseConfig.ID,
		Name:               "docs-client-v2",
		ConnectionType:     schemas.MCPConnectionTypeHTTP,
		ConnectionString:   schemas.NewEnvVar("http://mcp.internal"),
		AuthType:           schemas.MCPAuthTypeNone,
		ToolsToExecute:     []string{"search", "list"},
		ToolsToAutoExecute: []string{"search"},
		IsPingAvailable:    false,
		ToolSyncInterval:   7 * time.Minute,
	}
	if err := server.ApplyClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:           handlers.ClusterConfigScopeMCPClient,
		MCPClientID:     updatedConfig.ID,
		MCPClientConfig: updatedConfig,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(update) error = %v", err)
	}
	if manager.updateCalls != 1 {
		t.Fatalf("expected one runtime update call, got %d", manager.updateCalls)
	}
	if len(store.updated) != 1 || store.updated[0].Name != "docs-client-v2" {
		t.Fatalf("expected persisted update, got %+v", store.updated)
	}
	if got, err := server.Config.GetMCPClient(updatedConfig.ID); err != nil || got.Name != "docs-client-v2" || got.ToolSyncInterval != 7*time.Minute {
		t.Fatalf("expected updated runtime mcp client, got %+v err=%v", got, err)
	}

	if err := server.ApplyClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:       handlers.ClusterConfigScopeMCPClient,
		MCPClientID: updatedConfig.ID,
		Delete:      true,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(delete) error = %v", err)
	}
	if manager.removeCalls != 1 {
		t.Fatalf("expected one runtime remove call, got %d", manager.removeCalls)
	}
	if len(store.deleted) != 1 || store.deleted[0] != updatedConfig.ID {
		t.Fatalf("expected persisted delete, got %+v", store.deleted)
	}
	if _, err := server.Config.GetMCPClient(updatedConfig.ID); err == nil {
		t.Fatal("expected mcp client to be removed from runtime config")
	}
}
