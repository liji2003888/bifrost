package server

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	created    []*schemas.MCPClientConfig
	updated    []*configstoreTables.TableMCPClient
	deleted    []string
	rows       map[string]*configstoreTables.TableMCPClient
	hostedRows map[string]*configstoreTables.TableMCPHostedTool
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

func (s *fakeMCPClusterApplyStore) GetMCPHostedTools(_ context.Context) ([]configstoreTables.TableMCPHostedTool, error) {
	result := make([]configstoreTables.TableMCPHostedTool, 0, len(s.hostedRows))
	for _, tool := range s.hostedRows {
		cloned := *tool
		result = append(result, cloned)
	}
	return result, nil
}

func (s *fakeMCPClusterApplyStore) GetMCPHostedToolByID(_ context.Context, id string) (*configstoreTables.TableMCPHostedTool, error) {
	if s.hostedRows == nil {
		return nil, configstore.ErrNotFound
	}
	tool, ok := s.hostedRows[id]
	if !ok {
		return nil, configstore.ErrNotFound
	}
	cloned := *tool
	return &cloned, nil
}

func (s *fakeMCPClusterApplyStore) GetMCPHostedToolByName(_ context.Context, name string) (*configstoreTables.TableMCPHostedTool, error) {
	if s.hostedRows == nil {
		return nil, configstore.ErrNotFound
	}
	for _, tool := range s.hostedRows {
		if tool.Name == name {
			cloned := *tool
			return &cloned, nil
		}
	}
	return nil, configstore.ErrNotFound
}

func (s *fakeMCPClusterApplyStore) CreateMCPHostedTool(_ context.Context, tool *configstoreTables.TableMCPHostedTool) error {
	if s.hostedRows == nil {
		s.hostedRows = map[string]*configstoreTables.TableMCPHostedTool{}
	}
	cloned := *tool
	s.hostedRows[tool.ToolID] = &cloned
	return nil
}

func (s *fakeMCPClusterApplyStore) UpdateMCPHostedTool(_ context.Context, tool *configstoreTables.TableMCPHostedTool) error {
	if s.hostedRows == nil {
		s.hostedRows = map[string]*configstoreTables.TableMCPHostedTool{}
	}
	cloned := *tool
	s.hostedRows[tool.ToolID] = &cloned
	return nil
}

func (s *fakeMCPClusterApplyStore) DeleteMCPHostedTool(_ context.Context, id string) error {
	if s.hostedRows == nil {
		return configstore.ErrNotFound
	}
	if _, ok := s.hostedRows[id]; !ok {
		return configstore.ErrNotFound
	}
	delete(s.hostedRows, id)
	return nil
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

func TestPersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := &fakeMCPClusterApplyStore{
		rows: map[string]*configstoreTables.TableMCPClient{
			"client-1": {
				ClientID:       "client-1",
				Name:           "docs-client",
				ConnectionType: string(schemas.MCPConnectionTypeHTTP),
				AuthType:       string(schemas.MCPAuthTypeNone),
			},
		},
	}
	cfg := &lib.Config{
		MCPConfig: &schemas.MCPConfig{
			ClientConfigs: []*schemas.MCPClientConfig{
				{
					ID:             "client-1",
					Name:           "docs-client",
					ConnectionType: schemas.MCPConnectionTypeHTTP,
					AuthType:       schemas.MCPAuthTypeNone,
				},
			},
		},
		ConfigStore: store,
	}
	cfg.SetBifrostClient(&bifrost.Bifrost{})

	server := &BifrostHTTPServer{Config: cfg}
	description := "Search enterprise docs"
	tools := map[string]schemas.ChatTool{
		"docs-client-search": {
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name:        "docs-client-search",
				Description: &description,
			},
		},
	}
	mapping := map[string]string{"docs_client_search": "docs-client-search"}

	if err := server.persistMCPDiscoveredTools(context.Background(), "client-1", tools, mapping); err != nil {
		t.Fatalf("persistMCPDiscoveredTools error = %v", err)
	}
	if len(store.updated) != 1 {
		t.Fatalf("expected discovered tools update to be persisted, got %+v", store.updated)
	}
	if len(store.updated[0].DiscoveredTools) != 1 {
		t.Fatalf("expected persisted discovered tools, got %+v", store.updated[0].DiscoveredTools)
	}

	got, err := cfg.GetMCPClient("client-1")
	if err != nil {
		t.Fatalf("GetMCPClient error = %v", err)
	}
	if len(got.DiscoveredTools) != 1 || got.DiscoveredToolNameMapping["docs_client_search"] != "docs-client-search" {
		t.Fatalf("expected runtime discovered tools snapshot, got %+v %+v", got.DiscoveredTools, got.DiscoveredToolNameMapping)
	}
}

func TestApplyClusterConfigChangeMCPHostedToolLifecycle(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	manager := coremcp.NewMCPManager(context.Background(), schemas.MCPConfig{}, nil, bifrost.NewNoOpLogger(), nil)
	client := &bifrost.Bifrost{}
	client.SetMCPManager(manager)

	store := &fakeMCPClusterApplyStore{
		hostedRows: map[string]*configstoreTables.TableMCPHostedTool{},
	}
	cfg := &lib.Config{ConfigStore: store}
	cfg.SetBifrostClient(client)

	server := &BifrostHTTPServer{
		Config: cfg,
		Client: client,
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	description := "Get user profile"
	timeoutSeconds := 10
	tool := &configstoreTables.TableMCPHostedTool{
		ToolID:      "tool-1",
		Name:        "get-user-profile",
		Description: &description,
		Method:      "GET",
		URL:         upstream.URL + "/users/{{args.user_id}}",
		Headers: map[string]string{
			"Authorization": "{{req.header.authorization}}",
		},
		AuthProfile: &configstoreTables.MCPHostedToolAuthProfile{
			Mode: configstoreTables.MCPHostedToolAuthModeBearerPassthrough,
		},
		ExecutionProfile: &configstoreTables.MCPHostedToolExecutionProfile{
			TimeoutSeconds: &timeoutSeconds,
		},
		ToolSchema: schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name:        "get-user-profile",
				Description: &description,
				Parameters: &schemas.ToolFunctionParameters{
					Type:     "object",
					Required: []string{"user_id"},
				},
			},
		},
	}

	if err := server.ApplyClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:           handlers.ClusterConfigScopeMCPHostedTool,
		MCPHostedToolID: tool.ToolID,
		MCPHostedTool:   tool,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(add hosted tool) error = %v", err)
	}
	if _, err := store.GetMCPHostedToolByID(context.Background(), tool.ToolID); err != nil {
		t.Fatalf("expected hosted tool persisted, got err=%v", err)
	}
	persisted, _ := store.GetMCPHostedToolByID(context.Background(), tool.ToolID)
	if persisted.AuthProfile == nil || persisted.AuthProfile.Mode != configstoreTables.MCPHostedToolAuthModeBearerPassthrough {
		t.Fatalf("expected hosted tool auth profile to persist, got %+v", persisted.AuthProfile)
	}
	if persisted.ExecutionProfile == nil || persisted.ExecutionProfile.TimeoutSeconds == nil || *persisted.ExecutionProfile.TimeoutSeconds != timeoutSeconds {
		t.Fatalf("expected hosted tool execution profile to persist, got %+v", persisted.ExecutionProfile)
	}
	if available := client.GetAvailableMCPTools(context.Background()); len(available) == 0 {
		t.Fatalf("expected hosted tool to be registered in runtime MCP manager, got %+v", available)
	}

	if err := server.ApplyClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:           handlers.ClusterConfigScopeMCPHostedTool,
		MCPHostedToolID: tool.ToolID,
		MCPHostedTool:   tool,
		Delete:          true,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(delete hosted tool) error = %v", err)
	}
	if _, err := store.GetMCPHostedToolByID(context.Background(), tool.ToolID); err == nil {
		t.Fatal("expected hosted tool to be removed from config store")
	}
	if available := client.GetAvailableMCPTools(context.Background()); len(available) != 0 {
		t.Fatalf("expected hosted tool to be removed from runtime MCP manager, got %+v", available)
	}
}

func TestApplyClusterConfigChangeMCPHostedToolRenameReplacesRuntimeRegistration(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	manager := coremcp.NewMCPManager(context.Background(), schemas.MCPConfig{}, nil, bifrost.NewNoOpLogger(), nil)
	client := &bifrost.Bifrost{}
	client.SetMCPManager(manager)

	store := &fakeMCPClusterApplyStore{
		hostedRows: map[string]*configstoreTables.TableMCPHostedTool{},
	}
	cfg := &lib.Config{ConfigStore: store}
	cfg.SetBifrostClient(client)

	server := &BifrostHTTPServer{
		Config: cfg,
		Client: client,
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	initialDescription := "Get user profile"
	initialTool := &configstoreTables.TableMCPHostedTool{
		ToolID:      "tool-1",
		Name:        "get-user-profile",
		Description: &initialDescription,
		Method:      "GET",
		URL:         upstream.URL + "/users/{{args.user_id}}",
		Headers: map[string]string{
			"Authorization": "{{req.header.authorization}}",
		},
		ToolSchema: schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name:        "get-user-profile",
				Description: &initialDescription,
				Parameters: &schemas.ToolFunctionParameters{
					Type:     "object",
					Required: []string{"user_id"},
				},
			},
		},
	}

	if err := server.ApplyClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:           handlers.ClusterConfigScopeMCPHostedTool,
		MCPHostedToolID: initialTool.ToolID,
		MCPHostedTool:   initialTool,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(add initial hosted tool) error = %v", err)
	}

	updatedDescription := "Sync user profile"
	bodyTemplate := `{"user_id":"{{args.user_id}}"}`
	updatedTool := &configstoreTables.TableMCPHostedTool{
		ToolID:      "tool-1",
		Name:        "sync-user-profile",
		Description: &updatedDescription,
		Method:      "POST",
		URL:         upstream.URL + "/users/{{args.user_id}}/sync",
		Headers: map[string]string{
			"Authorization": "{{req.header.authorization}}",
		},
		BodyTemplate: &bodyTemplate,
		ToolSchema: schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name:        "sync-user-profile",
				Description: &updatedDescription,
				Parameters: &schemas.ToolFunctionParameters{
					Type:     "object",
					Required: []string{"user_id"},
				},
			},
		},
	}

	if err := server.ApplyClusterConfigChange(context.Background(), &handlers.ClusterConfigChange{
		Scope:           handlers.ClusterConfigScopeMCPHostedTool,
		MCPHostedToolID: updatedTool.ToolID,
		MCPHostedTool:   updatedTool,
	}); err != nil {
		t.Fatalf("ApplyClusterConfigChange(update hosted tool) error = %v", err)
	}

	persisted, err := store.GetMCPHostedToolByID(context.Background(), updatedTool.ToolID)
	if err != nil {
		t.Fatalf("expected renamed hosted tool persisted, got err=%v", err)
	}
	if persisted.Name != "sync_user_profile" || persisted.Method != http.MethodPost {
		t.Fatalf("expected updated hosted tool persisted, got %+v", persisted)
	}

	available := client.GetAvailableMCPTools(context.Background())
	if len(available) != 1 {
		t.Fatalf("expected exactly one runtime hosted tool after rename, got %+v", available)
	}
	if available[0].Function == nil {
		t.Fatalf("expected runtime hosted tool to keep a function schema, got %+v", available)
	}
}
