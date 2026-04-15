package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	bifrost "github.com/maximhq/bifrost/core"
	coremcp "github.com/maximhq/bifrost/core/mcp"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type fakeMCPClusterManager struct{}

func (m *fakeMCPClusterManager) AddMCPClient(_ context.Context, _ *schemas.MCPClientConfig) error {
	return nil
}
func (m *fakeMCPClusterManager) RemoveMCPClient(_ context.Context, _ string) error { return nil }
func (m *fakeMCPClusterManager) UpdateMCPClient(_ context.Context, _ string, _ *schemas.MCPClientConfig) error {
	return nil
}
func (m *fakeMCPClusterManager) ReconnectMCPClient(_ context.Context, _ string) error { return nil }
func (m *fakeMCPClusterManager) AddMCPHostedTool(_ context.Context, _ *configstoreTables.TableMCPHostedTool) error {
	return nil
}
func (m *fakeMCPClusterManager) UpdateMCPHostedTool(_ context.Context, _ string, _ *configstoreTables.TableMCPHostedTool) error {
	return nil
}
func (m *fakeMCPClusterManager) RemoveMCPHostedTool(_ context.Context, _ string) error { return nil }
func (m *fakeMCPClusterManager) PreviewMCPHostedTool(_ context.Context, _ string, _ map[string]any) (*configstoreTables.MCPHostedToolExecutionResult, error) {
	return &configstoreTables.MCPHostedToolExecutionResult{
		Output:        "preview output",
		StatusCode:    200,
		LatencyMS:     12,
		ResponseBytes: 128,
		ContentType:   "application/json",
		ResolvedURL:   "https://api.company.com/users/42",
		ResponseSchema: map[string]any{
			"type": "object",
		},
	}, nil
}

type fakeMCPClusterStore struct {
	configstore.ConfigStore
	clientConfig *configstore.ClientConfig
	clients      []configstoreTables.TableMCPClient
	hostedTools  []configstoreTables.TableMCPHostedTool
}

func (s *fakeMCPClusterStore) CreateMCPClientConfig(_ context.Context, _ *schemas.MCPClientConfig) error {
	return nil
}

func (s *fakeMCPClusterStore) GetClientConfig(_ context.Context) (*configstore.ClientConfig, error) {
	return s.clientConfig, nil
}

func (s *fakeMCPClusterStore) GetMCPClientsPaginated(_ context.Context, _ configstore.MCPClientsQueryParams) ([]configstoreTables.TableMCPClient, int64, error) {
	return s.clients, int64(len(s.clients)), nil
}

func (s *fakeMCPClusterStore) GetMCPHostedTools(_ context.Context) ([]configstoreTables.TableMCPHostedTool, error) {
	result := make([]configstoreTables.TableMCPHostedTool, len(s.hostedTools))
	copy(result, s.hostedTools)
	return result, nil
}

func (s *fakeMCPClusterStore) GetMCPHostedToolByID(_ context.Context, id string) (*configstoreTables.TableMCPHostedTool, error) {
	for i := range s.hostedTools {
		if s.hostedTools[i].ToolID == id {
			cloned := s.hostedTools[i]
			return &cloned, nil
		}
	}
	return nil, configstore.ErrNotFound
}

type fakeMCPClientRuntimeManager struct {
	coremcp.MCPManagerInterface
	clients []schemas.MCPClientState
}

func (m *fakeMCPClientRuntimeManager) GetClients() []schemas.MCPClientState {
	return m.clients
}

type fakeMCPClusterPropagator struct {
	changes []*ClusterConfigChange
}

func (p *fakeMCPClusterPropagator) PropagateClusterConfigChange(_ context.Context, change *ClusterConfigChange) error {
	if p == nil || change == nil {
		return nil
	}
	cloned := *change
	p.changes = append(p.changes, &cloned)
	return nil
}

func TestAddMCPClientPropagatesClusterConfigChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := &fakeMCPClusterStore{
		clientConfig: &configstore.ClientConfig{},
	}
	propagator := &fakeMCPClusterPropagator{}
	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{
		ConfigStore: store,
	}, nil, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/mcp/client")
	ctx.Request.SetBodyString(`{
		"client_id":"client-1",
		"name":"docs_client",
		"connection_type":"http",
		"connection_string":{"value":"http://mcp.internal","env_var":"","from_env":false},
		"auth_type":"none",
		"tools_to_execute":["search"],
		"tools_to_auto_execute":["search"],
		"is_ping_available":true,
		"tool_sync_interval":5
	}`)

	handler.addMCPClient(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated cluster change, got %+v", propagator.changes)
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopeMCPClient || change.MCPClientID != "client-1" {
		t.Fatalf("unexpected propagated change: %+v", change)
	}
	if change.MCPClientConfig == nil || change.MCPClientConfig.Name != "docs_client" {
		t.Fatalf("expected mcp client config to be propagated, got %+v", change.MCPClientConfig)
	}
	if change.MCPClientConfig.ToolSyncInterval != 5*time.Minute {
		t.Fatalf("expected tool sync interval to be normalized to minutes, got %+v", change.MCPClientConfig.ToolSyncInterval)
	}
}

func TestGetMCPClientsPaginatedFallsBackToPersistedDiscoveredTools(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	description := "Search internal docs"
	store := &fakeMCPClusterStore{
		clients: []configstoreTables.TableMCPClient{
			{
				ID:               1,
				ClientID:         "client-1",
				Name:             "docs_client",
				ConnectionType:   "http",
				ConnectionString: schemas.NewEnvVar("http://mcp.internal"),
				AuthType:         "none",
				ToolsToExecute:   []string{"*"},
				DiscoveredTools: map[string]schemas.ChatTool{
					"docs_client-search": {
						Type: schemas.ChatToolTypeFunction,
						Function: &schemas.ChatToolFunction{
							Name:        "docs_client-search",
							Description: &description,
						},
					},
				},
				ToolNameMapping: map[string]string{"docs_client_search": "docs-client-search"},
			},
		},
	}
	runtimeManager := &fakeMCPClientRuntimeManager{}
	bfClient := &bifrost.Bifrost{}
	bfClient.SetMCPManager(runtimeManager)

	handler := NewMCPHandler(&fakeMCPClusterManager{}, bfClient, &lib.Config{
		ConfigStore: store,
	}, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/api/mcp/clients?limit=25")

	handler.getMCPClients(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}

	var payload struct {
		Clients []MCPClientResponse `json:"clients"`
	}
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Clients) != 1 {
		t.Fatalf("expected one client, got %+v", payload.Clients)
	}
	if payload.Clients[0].State != schemas.MCPConnectionStateError {
		t.Fatalf("expected disconnected client to be marked error, got %s", payload.Clients[0].State)
	}
	if len(payload.Clients[0].Tools) != 1 || payload.Clients[0].Tools[0].Name != "search" {
		t.Fatalf("expected persisted discovered tools to be returned, got %+v", payload.Clients[0].Tools)
	}
	if payload.Clients[0].ToolSnapshotSource != "persisted" {
		t.Fatalf("expected persisted tool snapshot source, got %+v", payload.Clients[0].ToolSnapshotSource)
	}
	if payload.Clients[0].ToolNameMapping["docs_client_search"] != "docs-client-search" {
		t.Fatalf("expected persisted tool name mapping, got %+v", payload.Clients[0].ToolNameMapping)
	}
}

func TestGetMCPClientsPaginatedMarksLiveToolsWhenConnected(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := &fakeMCPClusterStore{
		clients: []configstoreTables.TableMCPClient{
			{
				ID:               1,
				ClientID:         "client-1",
				Name:             "docs_client",
				ConnectionType:   "http",
				ConnectionString: schemas.NewEnvVar("http://mcp.internal"),
				AuthType:         "none",
				ToolsToExecute:   []string{"*"},
				DiscoveredTools:  map[string]schemas.ChatTool{},
				ToolNameMapping:  map[string]string{"docs_client_search": "docs-client-search"},
			},
		},
	}
	runtimeManager := &fakeMCPClientRuntimeManager{
		clients: []schemas.MCPClientState{
			{
				ExecutionConfig: &schemas.MCPClientConfig{ID: "client-1"},
				State:           schemas.MCPConnectionStateConnected,
				ToolMap: map[string]schemas.ChatTool{
					"docs_client-search": {
						Type: schemas.ChatToolTypeFunction,
						Function: &schemas.ChatToolFunction{
							Name: "search",
						},
					},
				},
			},
		},
	}
	bfClient := &bifrost.Bifrost{}
	bfClient.SetMCPManager(runtimeManager)

	handler := NewMCPHandler(&fakeMCPClusterManager{}, bfClient, &lib.Config{
		ConfigStore: store,
	}, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/api/mcp/clients?limit=25")

	handler.getMCPClients(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}

	var payload struct {
		Clients []MCPClientResponse `json:"clients"`
	}
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Clients) != 1 {
		t.Fatalf("expected one client, got %+v", payload.Clients)
	}
	if payload.Clients[0].State != schemas.MCPConnectionStateConnected {
		t.Fatalf("expected connected client, got %s", payload.Clients[0].State)
	}
	if payload.Clients[0].ToolSnapshotSource != "live" {
		t.Fatalf("expected live tool snapshot source, got %+v", payload.Clients[0].ToolSnapshotSource)
	}
}

func TestValidateMCPClientReturnsUnverifiedForRuntimeTemplates(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{}, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/mcp/client/validate")
	ctx.Request.SetBodyString(`{
		"name":"docs_client",
		"connection_type":"http",
		"connection_string":{"value":"https://mcp.internal/mcp","env_var":"","from_env":false},
		"auth_type":"headers",
		"headers":{"Authorization":{"value":"{{req.header.authorization}}","env_var":"","from_env":false}}
	}`)

	handler.validateMCPClient(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}

	var payload MCPClientValidationResponse
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Status != "unverified" {
		t.Fatalf("expected unverified, got %+v", payload)
	}
	if payload.Reason != "runtime_templates_require_request_context" {
		t.Fatalf("expected runtime template reason, got %+v", payload)
	}
}

func TestValidateMCPClientDetectsCompatibleEndpoint(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	serverInstance := mcpserver.NewMCPServer(
		"test-server",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)
	serverInstance.AddTool(
		mcpproto.NewTool("search_docs"),
		func(ctx context.Context, request mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
			return mcpproto.NewToolResultText("ok"), nil
		},
	)
	testServer := mcpserver.NewTestStreamableHTTPServer(serverInstance)
	defer testServer.Close()

	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{}, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/mcp/client/validate")
	ctx.Request.SetBodyString(`{
		"name":"docs_client",
		"connection_type":"http",
		"connection_string":{"value":"` + testServer.URL + `","env_var":"","from_env":false},
		"auth_type":"none"
	}`)

	handler.validateMCPClient(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}

	var payload MCPClientValidationResponse
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Status != "compatible" {
		t.Fatalf("expected compatible, got %+v", payload)
	}
	if len(payload.DiscoveredTools) != 1 || payload.DiscoveredTools[0] != "search_docs" {
		t.Fatalf("expected discovered tool list, got %+v", payload.DiscoveredTools)
	}
}

func TestGetMCPHostedToolsReturnsPersistedDefinitions(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	description := "Get user profile"
	store := &fakeMCPClusterStore{
		hostedTools: []configstoreTables.TableMCPHostedTool{
			{
				ToolID:      "tool-1",
				Name:        "get-user-profile",
				Description: &description,
				Method:      "GET",
				URL:         "https://api.company.com/users/profile",
				Headers: map[string]string{
					"Authorization": "{{req.header.authorization}}",
				},
			},
		},
	}

	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{
		ConfigStore: store,
	}, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/api/mcp/hosted-tools")

	handler.getMCPHostedTools(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}

	var payload MCPHostedToolsResponse
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Count != 1 || len(payload.Tools) != 1 {
		t.Fatalf("expected one hosted tool, got %+v", payload)
	}
	if payload.Tools[0].Name != "get-user-profile" || payload.Tools[0].Method != "GET" {
		t.Fatalf("unexpected hosted tool payload: %+v", payload.Tools[0])
	}
}

func TestAddMCPHostedToolPropagatesClusterConfigChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := &fakeMCPClusterStore{}
	propagator := &fakeMCPClusterPropagator{}
	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{
		ConfigStore: store,
	}, nil, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/mcp/hosted-tool")
	ctx.Request.SetBodyString(`{
		"name":"get-user-profile",
		"method":"GET",
		"url":"https://api.company.com/users/{{args.user_id}}",
		"headers":{"Authorization":"{{req.header.authorization}}"},
		"query_params":{"tenant_id":"{{args.tenant_id}}"},
		"auth_profile":{"mode":"bearer_passthrough"},
		"execution_profile":{"timeout_seconds":20,"max_response_body_bytes":8192},
		"response_examples":[{"summary":"First example"},{"summary":"Second example"}],
		"response_json_path":"data.summary"
	}`)

	handler.addMCPHostedTool(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated cluster change, got %+v", propagator.changes)
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopeMCPHostedTool || change.MCPHostedTool == nil {
		t.Fatalf("unexpected propagated change: %+v", change)
	}
	if change.MCPHostedTool.Name != "get_user_profile" || change.MCPHostedTool.ToolID == "" {
		t.Fatalf("expected hosted tool payload to be propagated, got %+v", change.MCPHostedTool)
	}
	requiredArgs := change.MCPHostedTool.ToolSchema.Function.Parameters.Required
	if len(requiredArgs) != 2 || requiredArgs[0] != "tenant_id" || requiredArgs[1] != "user_id" {
		t.Fatalf("expected tool schema to capture args from templates, got %+v", requiredArgs)
	}
	if change.MCPHostedTool.QueryParams["tenant_id"] != "{{args.tenant_id}}" {
		t.Fatalf("expected query param mapping to be propagated, got %+v", change.MCPHostedTool.QueryParams)
	}
	if change.MCPHostedTool.ResponseJSONPath == nil || *change.MCPHostedTool.ResponseJSONPath != "data.summary" {
		t.Fatalf("expected response json path to be propagated, got %+v", change.MCPHostedTool.ResponseJSONPath)
	}
	if len(change.MCPHostedTool.ResponseExamples) != 2 {
		t.Fatalf("expected response examples to be propagated, got %+v", change.MCPHostedTool.ResponseExamples)
	}
	if change.MCPHostedTool.AuthProfile == nil || change.MCPHostedTool.AuthProfile.Mode != configstoreTables.MCPHostedToolAuthModeBearerPassthrough {
		t.Fatalf("expected auth profile to be propagated, got %+v", change.MCPHostedTool.AuthProfile)
	}
	if change.MCPHostedTool.ExecutionProfile == nil || change.MCPHostedTool.ExecutionProfile.TimeoutSeconds == nil || *change.MCPHostedTool.ExecutionProfile.TimeoutSeconds != 20 {
		t.Fatalf("expected execution profile to be propagated, got %+v", change.MCPHostedTool.ExecutionProfile)
	}
}

func TestUpdateMCPHostedToolPropagatesClusterConfigChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	description := "Get user profile"
	store := &fakeMCPClusterStore{
		hostedTools: []configstoreTables.TableMCPHostedTool{
			{
				ToolID:      "tool-1",
				Name:        "get-user-profile",
				Description: &description,
				Method:      "GET",
				URL:         "https://api.company.com/users/{{args.user_id}}",
				Headers: map[string]string{
					"Authorization": "{{req.header.authorization}}",
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
			},
		},
	}
	propagator := &fakeMCPClusterPropagator{}
	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{
		ConfigStore: store,
	}, nil, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("id", "tool-1")
	ctx.Request.Header.SetMethod(fasthttp.MethodPut)
	ctx.Request.SetRequestURI("/api/mcp/hosted-tool/tool-1")
	ctx.Request.SetBodyString(`{
		"name":"update-user-profile",
		"method":"POST",
		"url":"https://api.company.com/users/{{args.user_id}}/sync",
		"headers":{"Authorization":"{{req.header.authorization}}","X-Tenant-ID":"{{req.header.x-tenant-id}}"},
		"query_params":{"tenant_id":"{{req.query.tenant_id}}"},
		"auth_profile":{"mode":"header_passthrough","header_mappings":{"X-Tenant-ID":"x-tenant-id"}},
		"execution_profile":{"timeout_seconds":15},
		"body_template":"{\"tenant_id\":\"{{req.query.tenant_id}}\",\"user_id\":\"{{args.user_id}}\"}",
		"response_examples":[{"summary":"Updated example"}],
		"response_template":"User {{response.user.name}} synced for {{response.team}}"
	}`)

	handler.updateMCPHostedTool(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated cluster change, got %+v", propagator.changes)
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopeMCPHostedTool || change.MCPHostedTool == nil {
		t.Fatalf("unexpected propagated change: %+v", change)
	}
	if change.MCPHostedToolID != "tool-1" || change.MCPHostedTool.Name != "update_user_profile" {
		t.Fatalf("expected normalized hosted tool update to be propagated, got %+v", change)
	}
	if change.MCPHostedTool.Method != fasthttp.MethodPost {
		t.Fatalf("expected updated method to be propagated, got %+v", change.MCPHostedTool.Method)
	}
	requiredArgs := change.MCPHostedTool.ToolSchema.Function.Parameters.Required
	if len(requiredArgs) != 2 || requiredArgs[0] != "tenant_id" || requiredArgs[1] != "user_id" {
		t.Fatalf("expected merged args from body/query/url templates, got %+v", requiredArgs)
	}
	if change.MCPHostedTool.QueryParams["tenant_id"] != "{{req.query.tenant_id}}" {
		t.Fatalf("expected updated query params to be propagated, got %+v", change.MCPHostedTool.QueryParams)
	}
	if change.MCPHostedTool.ResponseTemplate == nil || *change.MCPHostedTool.ResponseTemplate != "User {{response.user.name}} synced for {{response.team}}" {
		t.Fatalf("expected response template to be propagated, got %+v", change.MCPHostedTool.ResponseTemplate)
	}
	if len(change.MCPHostedTool.ResponseExamples) != 1 {
		t.Fatalf("expected updated response examples to be propagated, got %+v", change.MCPHostedTool.ResponseExamples)
	}
	if change.MCPHostedTool.AuthProfile == nil || change.MCPHostedTool.AuthProfile.Mode != configstoreTables.MCPHostedToolAuthModeHeaderPassthrough {
		t.Fatalf("expected updated auth profile to be propagated, got %+v", change.MCPHostedTool.AuthProfile)
	}
	if change.MCPHostedTool.AuthProfile.HeaderMappings["X-Tenant-ID"] != "x-tenant-id" {
		t.Fatalf("expected auth header mappings to be propagated, got %+v", change.MCPHostedTool.AuthProfile.HeaderMappings)
	}
	if change.MCPHostedTool.ExecutionProfile == nil || change.MCPHostedTool.ExecutionProfile.TimeoutSeconds == nil || *change.MCPHostedTool.ExecutionProfile.TimeoutSeconds != 15 {
		t.Fatalf("expected updated execution profile to be propagated, got %+v", change.MCPHostedTool.ExecutionProfile)
	}
}

func TestDeleteMCPHostedToolPropagatesClusterConfigChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	description := "Get user profile"
	store := &fakeMCPClusterStore{
		hostedTools: []configstoreTables.TableMCPHostedTool{
			{
				ToolID:      "tool-1",
				Name:        "get-user-profile",
				Description: &description,
				Method:      "GET",
				URL:         "https://api.company.com/users/profile",
			},
		},
	}
	propagator := &fakeMCPClusterPropagator{}
	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{
		ConfigStore: store,
	}, nil, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("id", "tool-1")
	ctx.Request.Header.SetMethod(fasthttp.MethodDelete)
	ctx.Request.SetRequestURI("/api/mcp/hosted-tool/tool-1")

	handler.deleteMCPHostedTool(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated cluster change, got %+v", propagator.changes)
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopeMCPHostedTool || !change.Delete || change.MCPHostedToolID != "tool-1" {
		t.Fatalf("unexpected propagated change: %+v", change)
	}
	if change.MCPHostedTool == nil || change.MCPHostedTool.Name != "get-user-profile" {
		t.Fatalf("expected delete payload to include hosted tool snapshot, got %+v", change.MCPHostedTool)
	}
}

func TestAddMCPHostedToolPreservesProvidedToolSchema(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := &fakeMCPClusterStore{}
	propagator := &fakeMCPClusterPropagator{}
	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{
		ConfigStore: store,
	}, nil, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/mcp/hosted-tool")
	ctx.Request.SetBodyString(`{
		"name":"lookup-user",
		"method":"POST",
		"url":"https://api.company.com/users/lookup",
		"response_schema":{"type":"object","properties":{"user":{"type":"object"}}},
		"response_examples":[{"user":{"id":"u-1"}}],
		"tool_schema":{
			"type":"function",
			"function":{
				"name":"ignored-name",
				"description":"Lookup a user record",
				"parameters":{
					"type":"object",
					"properties":{
						"user_id":{"type":"string","description":"User identifier"},
						"limit":{"type":"number"}
					},
					"required":["user_id"]
				}
			}
		}
	}`)

	handler.addMCPHostedTool(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated cluster change, got %+v", propagator.changes)
	}
	change := propagator.changes[0]
	if change.MCPHostedTool == nil || change.MCPHostedTool.ToolSchema.Function == nil || change.MCPHostedTool.ToolSchema.Function.Parameters == nil {
		t.Fatalf("expected hosted tool schema to be present, got %+v", change.MCPHostedTool)
	}
	if change.MCPHostedTool.ToolSchema.Function.Name != "lookup_user" {
		t.Fatalf("expected normalized tool schema function name, got %+v", change.MCPHostedTool.ToolSchema.Function.Name)
	}
	if len(change.MCPHostedTool.ToolSchema.Function.Parameters.Required) != 1 || change.MCPHostedTool.ToolSchema.Function.Parameters.Required[0] != "user_id" {
		t.Fatalf("expected provided required fields to be preserved, got %+v", change.MCPHostedTool.ToolSchema.Function.Parameters.Required)
	}
	if change.MCPHostedTool.ResponseSchema["type"] != "object" {
		t.Fatalf("expected response schema to be preserved, got %+v", change.MCPHostedTool.ResponseSchema)
	}
	if len(change.MCPHostedTool.ResponseExamples) != 1 {
		t.Fatalf("expected response examples to be preserved, got %+v", change.MCPHostedTool.ResponseExamples)
	}
	properties := change.MCPHostedTool.ToolSchema.Function.Parameters.Properties
	if properties == nil {
		t.Fatal("expected tool schema properties to be preserved")
	}
	if limit, ok := properties.Get("limit"); !ok {
		t.Fatalf("expected typed property to be preserved, got %+v", properties)
	} else {
		limitMap, ok := limit.(*schemas.OrderedMap)
		if !ok {
			t.Fatalf("expected ordered map property, got %T", limit)
		}
		typeValue, _ := limitMap.Get("type")
		if typeValue != "number" {
			t.Fatalf("expected number type for limit, got %+v", typeValue)
		}
	}
}

func TestPreviewMCPHostedToolReturnsExecutionMetadata(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	handler := NewMCPHandler(&fakeMCPClusterManager{}, nil, &lib.Config{
		ConfigStore: &fakeMCPClusterStore{},
	}, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.SetUserValue("id", "tool-1")
	ctx.Request.SetRequestURI("/api/mcp/hosted-tool/tool-1/preview")
	ctx.Request.SetBodyString(`{"args":{"user_id":"42"}}`)

	handler.previewMCPHostedTool(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}

	var payload MCPHostedToolPreviewResponse
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Status != "success" || payload.Preview == nil {
		t.Fatalf("expected preview response, got %+v", payload)
	}
	if payload.Preview.LatencyMS != 12 || payload.Preview.ContentType != "application/json" {
		t.Fatalf("expected execution metadata, got %+v", payload.Preview)
	}
	if payload.Preview.ResponseSchema["type"] != "object" {
		t.Fatalf("expected response schema in preview response, got %+v", payload.Preview.ResponseSchema)
	}
}
