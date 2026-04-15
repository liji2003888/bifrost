// Package handlers provides HTTP request handlers for the Bifrost HTTP transport.
// This file contains MCP (Model Context Protocol) tool execution handlers.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/google/uuid"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpproto "github.com/mark3labs/mcp-go/mcp"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/mcp"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type MCPManager interface {
	AddMCPClient(ctx context.Context, clientConfig *schemas.MCPClientConfig) error
	RemoveMCPClient(ctx context.Context, id string) error
	UpdateMCPClient(ctx context.Context, id string, updatedConfig *schemas.MCPClientConfig) error
	ReconnectMCPClient(ctx context.Context, id string) error
	AddMCPHostedTool(ctx context.Context, tool *configstoreTables.TableMCPHostedTool) error
	UpdateMCPHostedTool(ctx context.Context, id string, tool *configstoreTables.TableMCPHostedTool) error
	RemoveMCPHostedTool(ctx context.Context, id string) error
	PreviewMCPHostedTool(ctx context.Context, id string, args map[string]any) (*configstoreTables.MCPHostedToolExecutionResult, error)
}

// MCPHandler manages HTTP requests for MCP tool operations
type MCPHandler struct {
	client       *bifrost.Bifrost
	store        *lib.Config
	mcpManager   MCPManager
	oauthHandler *OAuthHandler
	propagator   ClusterConfigPropagator
}

// NewMCPHandler creates a new MCP handler instance
func NewMCPHandler(mcpManager MCPManager, client *bifrost.Bifrost, store *lib.Config, oauthHandler *OAuthHandler, propagator ClusterConfigPropagator) *MCPHandler {
	return &MCPHandler{
		client:       client,
		store:        store,
		mcpManager:   mcpManager,
		oauthHandler: oauthHandler,
		propagator:   propagator,
	}
}

// RegisterRoutes registers all MCP-related routes
func (h *MCPHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/mcp/clients", lib.ChainMiddlewares(h.getMCPClients, middlewares...))
	r.GET("/api/mcp/hosted-tools", lib.ChainMiddlewares(h.getMCPHostedTools, middlewares...))
	r.POST("/api/mcp/client", lib.ChainMiddlewares(h.addMCPClient, middlewares...))
	r.POST("/api/mcp/client/validate", lib.ChainMiddlewares(h.validateMCPClient, middlewares...))
	r.POST("/api/mcp/hosted-tool", lib.ChainMiddlewares(h.addMCPHostedTool, middlewares...))
	r.POST("/api/mcp/hosted-tool/{id}/preview", lib.ChainMiddlewares(h.previewMCPHostedTool, middlewares...))
	r.PUT("/api/mcp/hosted-tool/{id}", lib.ChainMiddlewares(h.updateMCPHostedTool, middlewares...))
	r.PUT("/api/mcp/client/{id}", lib.ChainMiddlewares(h.updateMCPClient, middlewares...))
	r.DELETE("/api/mcp/client/{id}", lib.ChainMiddlewares(h.deleteMCPClient, middlewares...))
	r.DELETE("/api/mcp/hosted-tool/{id}", lib.ChainMiddlewares(h.deleteMCPHostedTool, middlewares...))
	r.POST("/api/mcp/client/{id}/reconnect", lib.ChainMiddlewares(h.reconnectMCPClient, middlewares...))
	r.POST("/api/mcp/client/{id}/complete-oauth", lib.ChainMiddlewares(h.completeMCPClientOAuth, middlewares...))
}

// MCPClientResponse represents the response structure for MCP clients
type MCPClientResponse struct {
	Config             *schemas.MCPClientConfig   `json:"config"`
	Tools              []schemas.ChatToolFunction `json:"tools"`
	State              schemas.MCPConnectionState `json:"state"`
	ToolSnapshotSource string                     `json:"tool_snapshot_source,omitempty"`
	ToolNameMapping    map[string]string          `json:"tool_name_mapping,omitempty"`
}

func sortedToolFunctionsFromMap(clientName string, toolMap map[string]schemas.ChatTool) []schemas.ChatToolFunction {
	if len(toolMap) == 0 {
		return []schemas.ChatToolFunction{}
	}
	tools := make([]schemas.ChatToolFunction, 0, len(toolMap))
	for _, tool := range toolMap {
		if tool.Function != nil {
			cloned := *tool.Function
			cloned.Name = strings.TrimPrefix(cloned.Name, clientName+"-")
			tools = append(tools, cloned)
		}
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

func resolveMCPToolSnapshot(clientConfig *schemas.MCPClientConfig, liveTools []schemas.ChatToolFunction) ([]schemas.ChatToolFunction, string) {
	if len(liveTools) > 0 {
		sortedTools := make([]schemas.ChatToolFunction, len(liveTools))
		copy(sortedTools, liveTools)
		sort.Slice(sortedTools, func(i, j int) bool {
			return sortedTools[i].Name < sortedTools[j].Name
		})
		return sortedTools, "live"
	}
	if clientConfig != nil && len(clientConfig.DiscoveredTools) > 0 {
		return sortedToolFunctionsFromMap(clientConfig.Name, clientConfig.DiscoveredTools), "persisted"
	}
	return []schemas.ChatToolFunction{}, "none"
}

// getMCPClients handles GET /api/mcp/clients - Get all MCP clients
func (h *MCPHandler) getMCPClients(ctx *fasthttp.RequestCtx) {
	emptyResponse := map[string]interface{}{
		"clients":     []MCPClientResponse{},
		"count":       0,
		"total_count": 0,
		"limit":       0,
		"offset":      0,
	}
	if h.store.ConfigStore == nil {
		SendJSON(ctx, emptyResponse)
		return
	}

	// Check if pagination params are present — if so, use paginated DB path
	limitStr := string(ctx.QueryArgs().Peek("limit"))
	offsetStr := string(ctx.QueryArgs().Peek("offset"))
	searchStr := string(ctx.QueryArgs().Peek("search"))

	if limitStr != "" || offsetStr != "" || searchStr != "" {
		h.getMCPClientsPaginated(ctx, limitStr, offsetStr, searchStr)
		return
	}

	// Non-paginated path: read from in-memory config
	configsInStore := h.store.MCPConfig
	if configsInStore == nil {
		SendJSON(ctx, emptyResponse)
		return
	}
	// Get actual connected clients from Bifrost
	clientsInBifrost, err := h.client.GetMCPClients()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to get MCP clients from Bifrost: %v", err))
		return
	}
	// Create a map of connected clients for quick lookup
	connectedClientsMap := make(map[string]schemas.MCPClient)
	for _, client := range clientsInBifrost {
		connectedClientsMap[client.Config.ID] = client
	}
	// Build the final client list, including errored clients
	clients := make([]MCPClientResponse, 0, len(configsInStore.ClientConfigs))

	for _, configClient := range configsInStore.ClientConfigs {
		// Redact sensitive fields before sending to UI
		redactedConfig := h.store.RedactMCPClientConfig(configClient)
		if connectedClient, exists := connectedClientsMap[configClient.ID]; exists {
			sortedTools, snapshotSource := resolveMCPToolSnapshot(configClient, connectedClient.Tools)

			clients = append(clients, MCPClientResponse{
				Config:             redactedConfig,
				Tools:              sortedTools,
				State:              connectedClient.State,
				ToolSnapshotSource: snapshotSource,
				ToolNameMapping:    maps.Clone(configClient.DiscoveredToolNameMapping),
			})
		} else {
			// Client is in config but not connected, mark as errored
			tools, snapshotSource := resolveMCPToolSnapshot(configClient, nil)
			clients = append(clients, MCPClientResponse{
				Config:             redactedConfig,
				Tools:              tools,
				State:              schemas.MCPConnectionStateError,
				ToolSnapshotSource: snapshotSource,
				ToolNameMapping:    maps.Clone(configClient.DiscoveredToolNameMapping),
			})
		}
	}
	SendJSON(ctx, map[string]interface{}{
		"clients":     clients,
		"count":       len(clients),
		"total_count": len(clients),
		"limit":       len(clients),
		"offset":      0,
	})
}

// getMCPClientsPaginated handles the paginated path for GET /api/mcp/clients
func (h *MCPHandler) getMCPClientsPaginated(ctx *fasthttp.RequestCtx, limitStr, offsetStr, searchStr string) {
	params := configstore.MCPClientsQueryParams{
		Search: searchStr,
	}
	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil {
			SendError(ctx, 400, "Invalid limit parameter: must be a number")
			return
		}
		if n < 0 {
			SendError(ctx, 400, "Invalid limit parameter: must be non-negative")
			return
		}
		params.Limit = n
	}
	if offsetStr != "" {
		n, err := strconv.Atoi(offsetStr)
		if err != nil {
			SendError(ctx, 400, "Invalid offset parameter: must be a number")
			return
		}
		if n < 0 {
			SendError(ctx, 400, "Invalid offset parameter: must be non-negative")
			return
		}
		params.Offset = n
	}

	dbClients, totalCount, err := h.store.ConfigStore.GetMCPClientsPaginated(ctx, params)
	if err != nil {
		logger.Error("failed to retrieve MCP clients: %v", err)
		SendError(ctx, 500, "Failed to retrieve MCP clients")
		return
	}

	// Get connected clients from Bifrost engine for state/tools merge
	clientsInBifrost, err := h.client.GetMCPClients()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to get MCP clients from Bifrost: %v", err))
		return
	}
	connectedClientsMap := make(map[string]schemas.MCPClient)
	for _, client := range clientsInBifrost {
		connectedClientsMap[client.Config.ID] = client
	}

	// Convert DB rows to MCPClientConfig and merge with engine state
	clients := make([]MCPClientResponse, 0, len(dbClients))
	for _, dbClient := range dbClients {
		isPingAvailable := true
		if dbClient.IsPingAvailable != nil {
			isPingAvailable = *dbClient.IsPingAvailable
		}
		clientConfig := &schemas.MCPClientConfig{
			ID:                        dbClient.ClientID,
			Name:                      dbClient.Name,
			IsCodeModeClient:          dbClient.IsCodeModeClient,
			ConnectionType:            schemas.MCPConnectionType(dbClient.ConnectionType),
			ConnectionString:          dbClient.ConnectionString,
			StdioConfig:               dbClient.StdioConfig,
			AuthType:                  schemas.MCPAuthType(dbClient.AuthType),
			OauthConfigID:             dbClient.OauthConfigID,
			ToolsToExecute:            dbClient.ToolsToExecute,
			ToolsToAutoExecute:        dbClient.ToolsToAutoExecute,
			Headers:                   dbClient.Headers,
			IsPingAvailable:           isPingAvailable,
			ToolSyncInterval:          time.Duration(dbClient.ToolSyncInterval) * time.Minute,
			ToolPricing:               dbClient.ToolPricing,
			DiscoveredTools:           dbClient.DiscoveredTools,
			DiscoveredToolNameMapping: dbClient.ToolNameMapping,
		}
		redactedConfig := h.store.RedactMCPClientConfig(clientConfig)
		if connectedClient, exists := connectedClientsMap[clientConfig.ID]; exists {
			sortedTools, snapshotSource := resolveMCPToolSnapshot(clientConfig, connectedClient.Tools)
			clients = append(clients, MCPClientResponse{
				Config:             redactedConfig,
				Tools:              sortedTools,
				State:              connectedClient.State,
				ToolSnapshotSource: snapshotSource,
				ToolNameMapping:    maps.Clone(clientConfig.DiscoveredToolNameMapping),
			})
		} else {
			tools, snapshotSource := resolveMCPToolSnapshot(clientConfig, nil)
			clients = append(clients, MCPClientResponse{
				Config:             redactedConfig,
				Tools:              tools,
				State:              schemas.MCPConnectionStateError,
				ToolSnapshotSource: snapshotSource,
				ToolNameMapping:    maps.Clone(clientConfig.DiscoveredToolNameMapping),
			})
		}
	}

	SendJSON(ctx, map[string]interface{}{
		"clients":     clients,
		"count":       len(clients),
		"total_count": totalCount,
		"limit":       params.Limit,
		"offset":      params.Offset,
	})
}

// reconnectMCPClient handles POST /api/mcp/client/{id}/reconnect - Reconnect an MCP client
func (h *MCPHandler) reconnectMCPClient(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP operations unavailable: config store is disabled")
		return
	}
	id, err := getIDFromCtx(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid id: %v", err))
		return
	}
	if err := h.mcpManager.ReconnectMCPClient(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to reconnect MCP client: %v", err))
		return
	}
	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "MCP client reconnected successfully",
	})
}

// OAuthConfigRequest represents OAuth configuration in the request
type OAuthConfigRequest struct {
	ClientID        string   `json:"client_id"`
	ClientSecret    string   `json:"client_secret"`
	AuthorizeURL    string   `json:"authorize_url"`
	TokenURL        string   `json:"token_url"`
	RegistrationURL string   `json:"registration_url"`
	Scopes          []string `json:"scopes"`
}

// MCPClientRequest represents the full MCP client creation request with OAuth support
type MCPClientRequest struct {
	configstoreTables.TableMCPClient
	OauthConfig *OAuthConfigRequest `json:"oauth_config,omitempty"`
}

type MCPClientValidationResponse struct {
	Status          string   `json:"status"`
	Message         string   `json:"message"`
	Reason          string   `json:"reason,omitempty"`
	DiscoveredTools []string `json:"discovered_tools,omitempty"`
}

type MCPHostedToolRequest struct {
	Name             string                                           `json:"name"`
	Description      *string                                          `json:"description,omitempty"`
	Method           string                                           `json:"method"`
	URL              string                                           `json:"url"`
	Headers          map[string]string                                `json:"headers,omitempty"`
	QueryParams      map[string]string                                `json:"query_params,omitempty"`
	AuthProfile      *configstoreTables.MCPHostedToolAuthProfile      `json:"auth_profile,omitempty"`
	ExecutionProfile *configstoreTables.MCPHostedToolExecutionProfile `json:"execution_profile,omitempty"`
	BodyTemplate     *string                                          `json:"body_template,omitempty"`
	ResponseJSONPath *string                                          `json:"response_json_path,omitempty"`
	ResponseTemplate *string                                          `json:"response_template,omitempty"`
	ResponseSchema   map[string]any                                   `json:"response_schema,omitempty"`
	ResponseExamples []any                                            `json:"response_examples,omitempty"`
	ToolSchema       *schemas.ChatTool                                `json:"tool_schema,omitempty"`
}

type MCPHostedToolPreviewRequest struct {
	Args map[string]any `json:"args"`
}

type MCPHostedToolPreviewResponse struct {
	Status  string                                          `json:"status"`
	Preview *configstoreTables.MCPHostedToolExecutionResult `json:"preview"`
}

func normalizeHostedToolAuthProfile(profile *configstoreTables.MCPHostedToolAuthProfile) *configstoreTables.MCPHostedToolAuthProfile {
	if profile == nil {
		return nil
	}
	normalized := *profile
	switch normalized.Mode {
	case configstoreTables.MCPHostedToolAuthModeNone,
		configstoreTables.MCPHostedToolAuthModeBearerPassthrough,
		configstoreTables.MCPHostedToolAuthModeHeaderPassthrough:
	default:
		normalized.Mode = configstoreTables.MCPHostedToolAuthModeNone
	}
	if len(normalized.HeaderMappings) > 0 {
		mappings := make(map[string]string, len(normalized.HeaderMappings))
		for target, source := range normalized.HeaderMappings {
			target = strings.TrimSpace(target)
			source = strings.TrimSpace(strings.ToLower(source))
			if target == "" || source == "" {
				continue
			}
			mappings[target] = source
		}
		normalized.HeaderMappings = mappings
	}
	return &normalized
}

func normalizeHostedToolExecutionProfile(profile *configstoreTables.MCPHostedToolExecutionProfile) *configstoreTables.MCPHostedToolExecutionProfile {
	if profile == nil {
		return nil
	}
	normalized := *profile
	if normalized.TimeoutSeconds != nil && *normalized.TimeoutSeconds <= 0 {
		normalized.TimeoutSeconds = nil
	}
	if normalized.MaxResponseBodyBytes != nil && *normalized.MaxResponseBodyBytes <= 0 {
		normalized.MaxResponseBodyBytes = nil
	}
	if normalized.TimeoutSeconds == nil && normalized.MaxResponseBodyBytes == nil {
		return nil
	}
	return &normalized
}

func normalizeHostedToolSchema(name string, description *string, provided *schemas.ChatTool, argNames []string) schemas.ChatTool {
	if provided == nil {
		return buildHostedToolSchema(name, description, argNames)
	}
	normalized := *provided
	normalized.Type = schemas.ChatToolTypeFunction
	if normalized.Function == nil {
		normalized.Function = &schemas.ChatToolFunction{}
	}
	normalized.Function.Name = name
	normalized.Function.Description = description
	if normalized.Function.Parameters == nil {
		normalized.Function.Parameters = &schemas.ToolFunctionParameters{
			Type:       "object",
			Properties: schemas.NewOrderedMap(),
			Required:   argNames,
		}
		return normalized
	}
	if strings.TrimSpace(normalized.Function.Parameters.Type) == "" {
		normalized.Function.Parameters.Type = "object"
	}
	if normalized.Function.Parameters.Properties == nil {
		normalized.Function.Parameters.Properties = schemas.NewOrderedMap()
	}
	if len(normalized.Function.Parameters.Required) == 0 {
		normalized.Function.Parameters.Required = argNames
	}
	return normalized
}

type MCPHostedToolsResponse struct {
	Tools []configstoreTables.TableMCPHostedTool `json:"tools"`
	Count int                                    `json:"count"`
}

func envVarRequiresRuntimeTemplate(value *schemas.EnvVar) bool {
	if value == nil {
		return false
	}
	raw := strings.TrimSpace(value.GetValue())
	if strings.Contains(raw, "{{") && strings.Contains(raw, "}}") {
		return true
	}
	return false
}

func headersRequireRuntimeTemplate(headers map[string]schemas.EnvVar) bool {
	for _, value := range headers {
		if envVarRequiresRuntimeTemplate(&value) {
			return true
		}
	}
	return false
}

func probeMCPCompatibility(ctx context.Context, config *schemas.MCPClientConfig) ([]string, error) {
	if config == nil || config.ConnectionString == nil || strings.TrimSpace(config.ConnectionString.GetValue()) == "" {
		return nil, fmt.Errorf("connection string is required")
	}

	headers, err := config.HttpHeaders(ctx, nil)
	if err != nil {
		return nil, err
	}

	var externalClient *mcpclient.Client
	switch config.ConnectionType {
	case schemas.MCPConnectionTypeHTTP:
		httpTransport, err := transport.NewStreamableHTTP(
			config.ConnectionString.GetValue(),
			transport.WithHTTPHeaders(headers),
			transport.WithHTTPTimeout(5*time.Second),
		)
		if err != nil {
			return nil, err
		}
		externalClient = mcpclient.NewClient(httpTransport)
	case schemas.MCPConnectionTypeSSE:
		sseTransport, err := transport.NewSSE(config.ConnectionString.GetValue(), transport.WithHeaders(headers))
		if err != nil {
			return nil, err
		}
		externalClient = mcpclient.NewClient(sseTransport)
	default:
		return nil, fmt.Errorf("validation only supports http or sse MCP connections")
	}

	defer externalClient.Close()

	probeCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := externalClient.Start(probeCtx); err != nil {
		return nil, err
	}

	_, err = externalClient.Initialize(probeCtx, mcpproto.InitializeRequest{
		Params: mcpproto.InitializeParams{
			ProtocolVersion: mcpproto.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcpproto.ClientCapabilities{},
			ClientInfo: mcpproto.Implementation{
				Name:    "Bifrost-import-validator",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		return nil, err
	}

	toolsResult, err := externalClient.ListTools(probeCtx, mcpproto.ListToolsRequest{
		PaginatedRequest: mcpproto.PaginatedRequest{
			Request: mcpproto.Request{
				Method: string(mcpproto.MethodToolsList),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if toolsResult == nil || len(toolsResult.Tools) == 0 {
		return []string{}, nil
	}

	discoveredTools := make([]string, 0, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		discoveredTools = append(discoveredTools, tool.Name)
	}
	sort.Strings(discoveredTools)
	return discoveredTools, nil
}

var hostedToolTemplatePattern = regexp.MustCompile(`\{\{\s*([^{}]+)\s*\}\}`)
var hostedToolNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func normalizeHostedToolName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = hostedToolNameSanitizer.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return "hosted_tool"
	}
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "tool_" + name
	}
	return name
}

func hostedToolArgumentNames(url string, headers map[string]string, queryParams map[string]string, bodyTemplate *string) []string {
	seen := map[string]struct{}{}
	add := func(value string) {
		for _, match := range hostedToolTemplatePattern.FindAllStringSubmatch(value, -1) {
			if len(match) < 2 {
				continue
			}
			token := strings.TrimSpace(match[1])
			switch {
			case strings.HasPrefix(token, "args."):
				token = strings.TrimSpace(strings.TrimPrefix(token, "args."))
			case strings.HasPrefix(token, "req.body."):
				token = strings.TrimSpace(strings.TrimPrefix(token, "req.body."))
			case strings.HasPrefix(token, "req.query."):
				token = strings.TrimSpace(strings.TrimPrefix(token, "req.query."))
			default:
				continue
			}
			if token == "" {
				continue
			}
			seen[token] = struct{}{}
		}
	}

	add(url)
	for _, value := range headers {
		add(value)
	}
	for _, value := range queryParams {
		add(value)
	}
	if bodyTemplate != nil {
		add(*bodyTemplate)
	}

	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func buildHostedToolSchema(name string, description *string, argNames []string) schemas.ChatTool {
	properties := schemas.NewOrderedMapWithCapacity(len(argNames))
	for _, argName := range argNames {
		properties.Set(argName, schemas.NewOrderedMapFromPairs(
			schemas.KV("type", "string"),
		))
	}
	return schemas.ChatTool{
		Type: schemas.ChatToolTypeFunction,
		Function: &schemas.ChatToolFunction{
			Name:        name,
			Description: description,
			Parameters: &schemas.ToolFunctionParameters{
				Type:       "object",
				Properties: properties,
				Required:   argNames,
			},
		},
	}
}

func (h *MCPHandler) validateMCPClient(ctx *fasthttp.RequestCtx) {
	var req MCPClientRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}

	if req.ConnectionType != string(schemas.MCPConnectionTypeHTTP) && req.ConnectionType != string(schemas.MCPConnectionTypeSSE) {
		SendJSON(ctx, MCPClientValidationResponse{
			Status:  "incompatible",
			Message: "Only MCP streamable HTTP or SSE endpoints can be validated in this flow.",
			Reason:  "unsupported_connection_type",
		})
		return
	}

	if req.ConnectionString == nil || strings.TrimSpace(req.ConnectionString.GetValue()) == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "connection_string is required")
		return
	}

	if req.AuthType == string(schemas.MCPAuthTypeOauth) {
		SendJSON(ctx, MCPClientValidationResponse{
			Status:  "unverified",
			Message: "OAuth-backed MCP endpoints require interactive authorization before they can be probed.",
			Reason:  "oauth_requires_authorization",
		})
		return
	}

	if envVarRequiresRuntimeTemplate(req.ConnectionString) || headersRequireRuntimeTemplate(req.Headers) {
		SendJSON(ctx, MCPClientValidationResponse{
			Status:  "unverified",
			Message: "This endpoint uses runtime request templating (for example {{req.header.authorization}}), so offline validation is skipped. It must still point to an MCP-compatible server at runtime.",
			Reason:  "runtime_templates_require_request_context",
		})
		return
	}

	schemasConfig := &schemas.MCPClientConfig{
		Name:             req.Name,
		ConnectionType:   schemas.MCPConnectionType(req.ConnectionType),
		ConnectionString: req.ConnectionString,
		AuthType:         schemas.MCPAuthType(req.AuthType),
		Headers:          req.Headers,
	}

	discoveredTools, err := probeMCPCompatibility(ctx, schemasConfig)
	if err != nil {
		SendJSON(ctx, MCPClientValidationResponse{
			Status:  "incompatible",
			Message: fmt.Sprintf("Endpoint did not respond as an MCP-compatible server: %v", err),
			Reason:  "probe_failed",
		})
		return
	}

	message := "Endpoint responded as an MCP-compatible server."
	if len(discoveredTools) > 0 {
		message = fmt.Sprintf("Endpoint responded as an MCP-compatible server and exposed %d tool(s).", len(discoveredTools))
	}

	SendJSON(ctx, MCPClientValidationResponse{
		Status:          "compatible",
		Message:         message,
		Reason:          "validated",
		DiscoveredTools: discoveredTools,
	})
}

func (h *MCPHandler) getMCPHostedTools(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendJSON(ctx, MCPHostedToolsResponse{
			Tools: []configstoreTables.TableMCPHostedTool{},
			Count: 0,
		})
		return
	}

	tools, err := h.store.ConfigStore.GetMCPHostedTools(ctx)
	if err != nil {
		logger.Error("failed to retrieve hosted MCP tools: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve hosted MCP tools")
		return
	}

	SendJSON(ctx, MCPHostedToolsResponse{
		Tools: tools,
		Count: len(tools),
	})
}

func (h *MCPHandler) addMCPHostedTool(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP hosted tools unavailable: config store is disabled")
		return
	}

	var req MCPHostedToolRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}

	req.Name = normalizeHostedToolName(req.Name)
	req.Method = strings.ToUpper(strings.TrimSpace(req.Method))
	req.URL = strings.TrimSpace(req.URL)
	if req.Name == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name is required")
		return
	}
	if err := mcp.ValidateMCPClientName(req.Name); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid tool name: %v", err))
		return
	}
	if req.Method == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "method is required")
		return
	}
	switch req.Method {
	case fasthttp.MethodGet, fasthttp.MethodPost, fasthttp.MethodPut, fasthttp.MethodDelete, fasthttp.MethodPatch:
	default:
		SendError(ctx, fasthttp.StatusBadRequest, "method must be one of GET, POST, PUT, DELETE, PATCH")
		return
	}
	if req.URL == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "url is required")
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		SendError(ctx, fasthttp.StatusBadRequest, "url must start with http:// or https://")
		return
	}

	argNames := hostedToolArgumentNames(req.URL, req.Headers, req.QueryParams, req.BodyTemplate)
	tool := &configstoreTables.TableMCPHostedTool{
		ToolID:           uuid.New().String(),
		Name:             req.Name,
		Description:      req.Description,
		Method:           req.Method,
		URL:              req.URL,
		Headers:          maps.Clone(req.Headers),
		QueryParams:      maps.Clone(req.QueryParams),
		AuthProfile:      normalizeHostedToolAuthProfile(req.AuthProfile),
		ExecutionProfile: normalizeHostedToolExecutionProfile(req.ExecutionProfile),
		BodyTemplate:     req.BodyTemplate,
		ResponseSchema:   cloneStringAnyMap(req.ResponseSchema),
		ResponseExamples: cloneAnySlice(req.ResponseExamples),
		ResponseJSONPath: req.ResponseJSONPath,
		ResponseTemplate: req.ResponseTemplate,
		ToolSchema:       normalizeHostedToolSchema(req.Name, req.Description, req.ToolSchema, argNames),
	}

	if err := h.mcpManager.AddMCPHostedTool(ctx, tool); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to create hosted MCP tool: %v", err))
		return
	}

	h.propagateClusterMCPHostedToolChange(ctx, &ClusterConfigChange{
		Scope:           ClusterConfigScopeMCPHostedTool,
		MCPHostedToolID: tool.ToolID,
		MCPHostedTool:   tool,
	})

	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "Hosted MCP tool created successfully",
		"tool":    tool,
	})
}

func (h *MCPHandler) deleteMCPHostedTool(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP hosted tools unavailable: config store is disabled")
		return
	}

	id, err := getIDFromCtx(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid id: %v", err))
		return
	}

	existing, err := h.store.ConfigStore.GetMCPHostedToolByID(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Hosted MCP tool not found")
			return
		}
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to load hosted MCP tool: %v", err))
		return
	}

	if err := h.mcpManager.RemoveMCPHostedTool(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to remove hosted MCP tool: %v", err))
		return
	}

	h.propagateClusterMCPHostedToolChange(ctx, &ClusterConfigChange{
		Scope:           ClusterConfigScopeMCPHostedTool,
		MCPHostedToolID: id,
		MCPHostedTool:   existing,
		Delete:          true,
	})

	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "Hosted MCP tool removed successfully",
	})
}

func (h *MCPHandler) updateMCPHostedTool(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP hosted tools unavailable: config store is disabled")
		return
	}

	id, err := getIDFromCtx(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid id: %v", err))
		return
	}

	existing, err := h.store.ConfigStore.GetMCPHostedToolByID(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Hosted MCP tool not found")
			return
		}
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to load hosted MCP tool: %v", err))
		return
	}

	var req MCPHostedToolRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}

	req.Name = normalizeHostedToolName(req.Name)
	req.Method = strings.ToUpper(strings.TrimSpace(req.Method))
	req.URL = strings.TrimSpace(req.URL)
	if req.Name == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name is required")
		return
	}
	if err := mcp.ValidateMCPClientName(req.Name); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid tool name: %v", err))
		return
	}
	if req.Method == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "method is required")
		return
	}
	switch req.Method {
	case fasthttp.MethodGet, fasthttp.MethodPost, fasthttp.MethodPut, fasthttp.MethodDelete, fasthttp.MethodPatch:
	default:
		SendError(ctx, fasthttp.StatusBadRequest, "method must be one of GET, POST, PUT, DELETE, PATCH")
		return
	}
	if req.URL == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "url is required")
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		SendError(ctx, fasthttp.StatusBadRequest, "url must start with http:// or https://")
		return
	}

	argNames := hostedToolArgumentNames(req.URL, req.Headers, req.QueryParams, req.BodyTemplate)
	tool := &configstoreTables.TableMCPHostedTool{
		ID:               existing.ID,
		ToolID:           existing.ToolID,
		Name:             req.Name,
		Description:      req.Description,
		Method:           req.Method,
		URL:              req.URL,
		Headers:          maps.Clone(req.Headers),
		QueryParams:      maps.Clone(req.QueryParams),
		AuthProfile:      normalizeHostedToolAuthProfile(req.AuthProfile),
		ExecutionProfile: normalizeHostedToolExecutionProfile(req.ExecutionProfile),
		BodyTemplate:     req.BodyTemplate,
		ResponseSchema:   cloneStringAnyMap(req.ResponseSchema),
		ResponseExamples: cloneAnySlice(req.ResponseExamples),
		ResponseJSONPath: req.ResponseJSONPath,
		ResponseTemplate: req.ResponseTemplate,
		CreatedAt:        existing.CreatedAt,
		UpdatedAt:        existing.UpdatedAt,
		ToolSchema:       normalizeHostedToolSchema(req.Name, req.Description, req.ToolSchema, argNames),
	}

	if err := h.mcpManager.UpdateMCPHostedTool(ctx, id, tool); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to update hosted MCP tool: %v", err))
		return
	}

	h.propagateClusterMCPHostedToolChange(ctx, &ClusterConfigChange{
		Scope:           ClusterConfigScopeMCPHostedTool,
		MCPHostedToolID: tool.ToolID,
		MCPHostedTool:   tool,
	})

	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "Hosted MCP tool updated successfully",
		"tool":    tool,
	})
}

func cloneStringAnyMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		cloned := make(map[string]any, len(value))
		for key, item := range value {
			cloned[key] = item
		}
		return cloned
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil {
		cloned = make(map[string]any, len(value))
		for key, item := range value {
			cloned[key] = item
		}
	}
	return cloned
}

func cloneAnySlice(value []any) []any {
	if len(value) == 0 {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		cloned := make([]any, len(value))
		copy(cloned, value)
		return cloned
	}
	var cloned []any
	if err := json.Unmarshal(data, &cloned); err != nil {
		cloned = make([]any, len(value))
		copy(cloned, value)
	}
	return cloned
}

func (h *MCPHandler) previewMCPHostedTool(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP hosted tools unavailable: config store is disabled")
		return
	}

	id, err := getIDFromCtx(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid id: %v", err))
		return
	}

	var req MCPHostedToolPreviewRequest
	if len(ctx.PostBody()) > 0 {
		if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
			return
		}
	}
	if req.Args == nil {
		req.Args = map[string]any{}
	}

	bifrostCtx, cancel := lib.ConvertToBifrostContext(ctx, false, nil)
	defer cancel()

	preview, err := h.mcpManager.PreviewMCPHostedTool(bifrostCtx, id, req.Args)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Hosted MCP tool not found")
			return
		}
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to preview hosted MCP tool: %v", err))
		return
	}

	SendJSON(ctx, MCPHostedToolPreviewResponse{
		Status:  "success",
		Preview: preview,
	})
}

// addMCPClient handles POST /api/mcp/client - Add a new MCP client
func (h *MCPHandler) addMCPClient(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP operations unavailable: config store is disabled")
		return
	}
	var req MCPClientRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}

	// Generate a unique client ID if not provided
	if req.ClientID == "" {
		req.ClientID = uuid.New().String()
	}

	if err := validateToolsToExecute(req.ToolsToExecute); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid tools_to_execute: %v", err))
		return
	}
	// Auto-clear tools_to_auto_execute if tools_to_execute is empty
	// If no tools are allowed to execute, no tools can be auto-executed
	if len(req.ToolsToExecute) == 0 {
		req.ToolsToAutoExecute = []string{}
	}
	if err := validateToolsToAutoExecute(req.ToolsToAutoExecute, req.ToolsToExecute); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid tools_to_auto_execute: %v", err))
		return
	}
	if err := mcp.ValidateMCPClientName(req.Name); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid client name: %v", err))
		return
	}

	// Check if OAuth flow is needed
	if req.AuthType == "oauth" {
		if req.OauthConfig == nil {
			SendError(ctx, fasthttp.StatusBadRequest, "OAuth configuration is required when auth_type is 'oauth'")
			return
		}

		// Validate: Either client_id must be provided, OR we need a server URL for discovery + dynamic registration
		// Client ID can be empty if the OAuth provider supports dynamic client registration (RFC 7591)
		if req.OauthConfig.ClientID == "" {
			// If no client_id, we need server URL for discovery
			if req.ConnectionString.GetValue() == "" {
				SendError(ctx, fasthttp.StatusBadRequest, "Either client_id must be provided, or server URL must be set for OAuth discovery and dynamic client registration")
				return
			}
			// Note: The InitiateOAuthFlow will check if registration_endpoint is available
			// and return a clear error if dynamic registration is not supported
		}

		// Build redirect URI - use Bifrost's own callback endpoint
		// Extract the base URL from the current request
		scheme := "http"
		if ctx.IsTLS() || string(ctx.Request.Header.Peek("X-Forwarded-Proto")) == "https" {
			scheme = "https"
		}
		host := string(ctx.Host())
		redirectURI := fmt.Sprintf("%s://%s/api/oauth/callback", scheme, host)

		// Initiate OAuth flow
		// ServerURL comes from ConnectionString (MCP server URL for OAuth discovery)
		// ClientID is optional - will be obtained via dynamic registration if not provided
		flowInitiation, err := h.oauthHandler.InitiateOAuthFlow(ctx, OAuthInitiationRequest{
			ClientID:        req.OauthConfig.ClientID,        // Optional: auto-generated if empty
			ClientSecret:    req.OauthConfig.ClientSecret,    // Optional: for PKCE or dynamic registration
			AuthorizeURL:    req.OauthConfig.AuthorizeURL,    // Optional: discovered if empty
			TokenURL:        req.OauthConfig.TokenURL,        // Optional: discovered if empty
			RegistrationURL: req.OauthConfig.RegistrationURL, // Optional: discovered if empty
			RedirectURI:     redirectURI,                     // Use server's own callback URL
			Scopes:          req.OauthConfig.Scopes,          // Optional: discovered if empty
			ServerURL:       req.ConnectionString.GetValue(), // MCP server URL for OAuth discovery
		})
		if err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to initiate OAuth flow: %v", err))
			return
		}

		toolSyncInterval := mcp.DefaultToolSyncInterval
		if req.ToolSyncInterval != 0 {
			toolSyncInterval = time.Duration(req.ToolSyncInterval) * time.Minute
		} else {
			config, err := h.store.ConfigStore.GetClientConfig(ctx)
			if err != nil {
				SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get client config: %v", err))
				return
			}
			if config != nil {
				toolSyncInterval = time.Duration(config.MCPToolSyncInterval) * time.Minute
			}
		}

		isPingAvailable := true
		if req.IsPingAvailable != nil {
			isPingAvailable = *req.IsPingAvailable
		}

		// Store MCP client config in OAuth provider memory (not in database)
		// It will be stored in database only after OAuth completion
		pendingConfig := schemas.MCPClientConfig{
			ID:                 req.ClientID,
			Name:               req.Name,
			IsCodeModeClient:   req.IsCodeModeClient,
			IsPingAvailable:    isPingAvailable,
			ToolSyncInterval:   toolSyncInterval,
			ConnectionType:     schemas.MCPConnectionType(req.ConnectionType),
			ConnectionString:   req.ConnectionString,
			StdioConfig:        req.StdioConfig,
			AuthType:           schemas.MCPAuthType(req.AuthType),
			OauthConfigID:      &flowInitiation.OauthConfigID,
			ToolsToExecute:     req.ToolsToExecute,
			ToolsToAutoExecute: req.ToolsToAutoExecute,
			Headers:            req.Headers,
		}

		// Store pending config in database (associated with oauth_config_id for multi-instance support)
		if err := h.oauthHandler.StorePendingMCPClient(flowInitiation.OauthConfigID, pendingConfig); err != nil {
			logger.Error(fmt.Sprintf("[Add MCP Client] Failed to store pending MCP client: %v", err))
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to store pending MCP client: %v", err))
			return
		}

		// Return OAuth flow initiation response
		SendJSON(ctx, map[string]any{
			"status":          "pending_oauth",
			"message":         "OAuth authorization required",
			"oauth_config_id": flowInitiation.OauthConfigID,
			"authorize_url":   flowInitiation.AuthorizeURL,
			"status_url":      buildAPIURL(ctx, fmt.Sprintf("/api/oauth/config/%s/status", flowInitiation.OauthConfigID)),
			"complete_url":    buildAPIURL(ctx, fmt.Sprintf("/api/mcp/client/%s/complete-oauth", flowInitiation.OauthConfigID)),
			"next_steps": []string{
				"打开 authorize_url 完成 OAuth 授权。",
				"轮询 status_url，直到状态变为 authorized。",
				"授权完成后调用 complete_url，让 MCP Server 正式落库并连接。",
			},
			"expires_at":    flowInitiation.ExpiresAt,
			"mcp_client_id": req.ClientID,
		})
		return
	}

	toolSyncInterval := mcp.DefaultToolSyncInterval
	if req.ToolSyncInterval != 0 {
		toolSyncInterval = time.Duration(req.ToolSyncInterval) * time.Minute
	} else {
		config, err := h.store.ConfigStore.GetClientConfig(ctx)
		if err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get client config: %v", err))
			return
		}
		if config != nil {
			toolSyncInterval = time.Duration(config.MCPToolSyncInterval) * time.Minute
		}
	}

	// Convert to schemas.MCPClientConfig for runtime bifrost client (without tool_pricing)
	// Dereference IsPingAvailable pointer, defaulting to true if nil (new clients default to ping available)
	isPingAvailable := true
	if req.IsPingAvailable != nil {
		isPingAvailable = *req.IsPingAvailable
	}
	schemasConfig := &schemas.MCPClientConfig{
		ID:                 req.ClientID,
		Name:               req.Name,
		IsCodeModeClient:   req.IsCodeModeClient,
		ConnectionType:     schemas.MCPConnectionType(req.ConnectionType),
		ConnectionString:   req.ConnectionString,
		StdioConfig:        req.StdioConfig,
		ToolsToExecute:     req.ToolsToExecute,
		ToolsToAutoExecute: req.ToolsToAutoExecute,
		Headers:            req.Headers,
		AuthType:           schemas.MCPAuthType(req.AuthType),
		OauthConfigID:      req.OauthConfigID,
		IsPingAvailable:    isPingAvailable,
		ToolSyncInterval:   toolSyncInterval,
		ToolPricing:        req.ToolPricing,
	}

	// Creating MCP client config in config store
	if h.store.ConfigStore != nil {
		if err := h.store.ConfigStore.CreateMCPClientConfig(ctx, schemasConfig); err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to create MCP config: %v", err))
			return
		}
	}
	if err := h.mcpManager.AddMCPClient(ctx, schemasConfig); err != nil {
		// Delete the created config from config store
		if h.store.ConfigStore != nil {
			if err := h.store.ConfigStore.DeleteMCPClientConfig(ctx, schemasConfig.ID); err != nil {
				logger.Error(fmt.Sprintf("Failed to delete MCP client config from database: %v. please restart bifrost to keep core and database in sync", err))
				SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to delete MCP client config from database: %v. please restart bifrost to keep core and database in sync", err))
				return
			}
		}
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to connect MCP client: %v", err))
		return
	}

	h.propagateClusterMCPClientChange(ctx, &ClusterConfigChange{
		Scope:           ClusterConfigScopeMCPClient,
		MCPClientID:     schemasConfig.ID,
		MCPClientConfig: schemasConfig,
	})

	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "MCP client connected successfully",
	})
}

// updateMCPClient handles PUT /api/mcp/client/{id} - Edit MCP client
func (h *MCPHandler) updateMCPClient(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP operations unavailable: config store is disabled")
		return
	}
	id, err := getIDFromCtx(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid id: %v", err))
		return
	}
	// Accept the full table client config to support tool_pricing
	var req *configstoreTables.TableMCPClient
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid request format: %v", err))
		return
	}
	req.ClientID = id
	// Validate tools_to_execute
	if err := validateToolsToExecute(req.ToolsToExecute); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid tools_to_execute: %v", err))
		return
	}
	// Auto-clear tools_to_auto_execute if tools_to_execute is empty
	// If no tools are allowed to execute, no tools can be auto-executed
	if len(req.ToolsToExecute) == 0 {
		req.ToolsToAutoExecute = []string{}
	}
	// Validate tools_to_auto_execute
	if err := validateToolsToAutoExecute(req.ToolsToAutoExecute, req.ToolsToExecute); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid tools_to_auto_execute: %v", err))
		return
	}
	// Validate client name
	if err := mcp.ValidateMCPClientName(req.Name); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid client name: %v", err))
		return
	}
	// Get existing config to handle redacted values
	var existingConfig *schemas.MCPClientConfig
	if h.store.MCPConfig != nil {
		for i, client := range h.store.MCPConfig.ClientConfigs {
			if client.ID == id {
				existingConfig = h.store.MCPConfig.ClientConfigs[i]
				break
			}
		}
	}
	if existingConfig == nil {
		SendError(ctx, fasthttp.StatusNotFound, "MCP client not found")
		return
	}

	// Merge redacted values - preserve old values if incoming values are redacted and unchanged
	req = mergeMCPRedactedValues(req, existingConfig, h.store.RedactMCPClientConfig(existingConfig))
	// Save existing DB config before update so we can rollback if memory update fails
	var oldDBConfig *configstoreTables.TableMCPClient
	if h.store.ConfigStore != nil {
		var err error
		oldDBConfig, err = h.store.ConfigStore.GetMCPClientByID(ctx, id)
		if err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get existing mcp client config: %v", err))
			return
		}
	}
	// Persist changes to config store
	if h.store.ConfigStore != nil {
		if err := h.store.ConfigStore.UpdateMCPClientConfig(ctx, id, req); err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to update mcp client config in store: %v", err))
			return
		}
	}
	toolSyncInterval := mcp.DefaultToolSyncInterval
	if req.ToolSyncInterval != 0 {
		toolSyncInterval = time.Duration(req.ToolSyncInterval) * time.Minute
	} else {
		config, err := h.store.ConfigStore.GetClientConfig(ctx)
		if err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get client config: %v", err))
			return
		}
		if config != nil {
			toolSyncInterval = time.Duration(config.MCPToolSyncInterval) * time.Minute
		}
	}
	// Convert to schemas.MCPClientConfig for runtime bifrost client (without tool_pricing)
	isPingAvailable := true
	if req.IsPingAvailable != nil {
		isPingAvailable = *req.IsPingAvailable
	}
	schemasConfig := &schemas.MCPClientConfig{
		ID:                 req.ClientID,
		Name:               req.Name,
		IsCodeModeClient:   req.IsCodeModeClient,
		ConnectionType:     existingConfig.ConnectionType,
		ConnectionString:   existingConfig.ConnectionString,
		StdioConfig:        existingConfig.StdioConfig,
		ToolsToExecute:     req.ToolsToExecute,
		ToolsToAutoExecute: req.ToolsToAutoExecute,
		Headers:            req.Headers,
		AuthType:           existingConfig.AuthType,
		OauthConfigID:      existingConfig.OauthConfigID,
		IsPingAvailable:    isPingAvailable,
		ToolSyncInterval:   toolSyncInterval,
		ToolPricing:        req.ToolPricing,
	}
	// Update MCP client in memory
	if err := h.mcpManager.UpdateMCPClient(ctx, id, schemasConfig); err != nil {
		// Rollback DB update to keep DB and memory in sync
		if h.store.ConfigStore != nil && oldDBConfig != nil {
			if rollbackErr := h.store.ConfigStore.UpdateMCPClientConfig(ctx, id, oldDBConfig); rollbackErr != nil {
				logger.Error(fmt.Sprintf("Failed to rollback MCP client DB update: %v. please restart bifrost to keep core and database in sync", rollbackErr))
			}
		}
		logger.Error(fmt.Sprintf("Failed to update MCP client: %v", err))
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to update mcp client: %v", err))
		return
	}

	h.propagateClusterMCPClientChange(ctx, &ClusterConfigChange{
		Scope:           ClusterConfigScopeMCPClient,
		MCPClientID:     id,
		MCPClientConfig: schemasConfig,
	})

	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "MCP client edited successfully",
	})
}

// deleteMCPClient handles DELETE /api/mcp/client/{id} - Remove an MCP client
func (h *MCPHandler) deleteMCPClient(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP operations unavailable: config store is disabled")
		return
	}
	id, err := getIDFromCtx(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid id: %v", err))
		return
	}
	// Delete from DB first to avoid memory/DB inconsistency if DB delete fails
	if h.store.ConfigStore != nil {
		if err := h.store.ConfigStore.DeleteMCPClientConfig(ctx, id); err != nil {
			logger.Error(fmt.Sprintf("Failed to delete MCP client config from database: %v", err))
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to delete MCP config: %v", err))
			return
		}
	}
	if err := h.mcpManager.RemoveMCPClient(ctx, id); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to remove MCP client: %v", err))
		return
	}

	h.propagateClusterMCPClientChange(ctx, &ClusterConfigChange{
		Scope:       ClusterConfigScopeMCPClient,
		MCPClientID: id,
		Delete:      true,
	})

	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "MCP client removed successfully",
	})
}

func getIDFromCtx(ctx *fasthttp.RequestCtx) (string, error) {
	idValue := ctx.UserValue("id")
	if idValue == nil {
		return "", fmt.Errorf("missing id parameter")
	}
	idStr, ok := idValue.(string)
	if !ok {
		return "", fmt.Errorf("invalid id parameter type")
	}

	return idStr, nil
}

func validateToolsToExecute(toolsToExecute []string) error {
	if len(toolsToExecute) > 0 {
		// Check if wildcard "*" is combined with other tool names
		hasWildcard := slices.Contains(toolsToExecute, "*")
		if hasWildcard && len(toolsToExecute) > 1 {
			return fmt.Errorf("invalid tools_to_execute: wildcard '*' cannot be combined with other tool names")
		}

		// Check for duplicate entries
		seen := make(map[string]bool)
		for _, tool := range toolsToExecute {
			if seen[tool] {
				return fmt.Errorf("invalid tools_to_execute: duplicate tool name '%s'", tool)
			}
			seen[tool] = true
		}
	}

	return nil
}

func validateToolsToAutoExecute(toolsToAutoExecute []string, toolsToExecute []string) error {
	if len(toolsToAutoExecute) > 0 {
		// Check if wildcard "*" is combined with other tool names
		hasWildcard := slices.Contains(toolsToAutoExecute, "*")
		if hasWildcard && len(toolsToAutoExecute) > 1 {
			return fmt.Errorf("wildcard '*' cannot be combined with other tool names")
		}

		// Check for duplicate entries
		seen := make(map[string]bool)
		for _, tool := range toolsToAutoExecute {
			if seen[tool] {
				return fmt.Errorf("duplicate tool name '%s'", tool)
			}
			seen[tool] = true
		}

		// Check that all tools in ToolsToAutoExecute are also in ToolsToExecute
		// Create a set of allowed tools from ToolsToExecute
		allowedTools := make(map[string]bool)
		hasWildcardInExecute := slices.Contains(toolsToExecute, "*")
		if hasWildcardInExecute {
			// If "*" is in ToolsToExecute, all tools are allowed
			return nil
		}
		for _, tool := range toolsToExecute {
			allowedTools[tool] = true
		}

		// Validate each tool in ToolsToAutoExecute
		for _, tool := range toolsToAutoExecute {
			if tool == "*" {
				// Wildcard is allowed if "*" is in ToolsToExecute
				if !hasWildcardInExecute {
					return fmt.Errorf("tool '%s' in tools_to_auto_execute is not in tools_to_execute", tool)
				}
			} else if !allowedTools[tool] {
				return fmt.Errorf("tool '%s' in tools_to_auto_execute is not in tools_to_execute", tool)
			}
		}
	}

	return nil
}

// mergeMCPRedactedValues merges incoming MCP client config with existing config,
// preserving old values when incoming values are redacted and unchanged.
// This follows the same pattern as provider config updates.
func mergeMCPRedactedValues(incoming *configstoreTables.TableMCPClient, oldRaw, oldRedacted *schemas.MCPClientConfig) *configstoreTables.TableMCPClient {
	merged := incoming

	// Handle ConnectionString - if incoming is redacted and equals old redacted, keep old raw value
	if incoming.ConnectionString != nil && oldRaw.ConnectionString != nil && oldRedacted.ConnectionString != nil {
		if incoming.ConnectionString.IsRedacted() && incoming.ConnectionString.Equals(oldRedacted.ConnectionString) {
			merged.ConnectionString = oldRaw.ConnectionString
		}
	}

	// Handle Headers - merge incoming with old, preserving redacted values
	if incoming.Headers != nil {
		incomingHeaders := incoming.Headers
		merged.Headers = make(map[string]schemas.EnvVar, len(incomingHeaders))
		for key, incomingValue := range incomingHeaders {
			if oldRaw.Headers != nil && oldRedacted.Headers != nil {
				if oldRedactedValue, existsInRedacted := oldRedacted.Headers[key]; existsInRedacted {
					if oldRawValue, existsInRaw := oldRaw.Headers[key]; existsInRaw {
						if incomingValue.IsRedacted() && incomingValue.Equals(&oldRedactedValue) {
							merged.Headers[key] = oldRawValue
							continue
						}
					}
				}
			}
			merged.Headers[key] = incomingValue
		}
	} else if oldRaw.Headers != nil {
		merged.Headers = oldRaw.Headers
	}

	// Preserve IsPingAvailable if not explicitly set in incoming request
	// This prevents the zero-value (false) from overwriting the existing DB value
	if incoming.IsPingAvailable == nil {
		merged.IsPingAvailable = bifrost.Ptr(oldRaw.IsPingAvailable)
	}

	return merged
}

// completeMCPClientOAuth handles POST /api/mcp/client/{id}/complete-oauth - Complete MCP client creation after OAuth authorization
// The {id} parameter is the oauth_config_id returned from the initial addMCPClient call
func (h *MCPHandler) completeMCPClientOAuth(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "MCP operations unavailable: config store is disabled")
		return
	}
	oauthConfigID, err := getIDFromCtx(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("[OAuth Complete] Invalid oauth_config_id: %v", err))
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Invalid oauth_config_id: %v", err))
		return
	}

	logger.Debug(fmt.Sprintf("[OAuth Complete] Completing OAuth for oauth_config_id: %s", oauthConfigID))

	// Check if OAuth flow is authorized
	oauthConfig, err := h.store.ConfigStore.GetOauthConfigByID(ctx, oauthConfigID)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to get OAuth config: %v", err))
		return
	}

	if oauthConfig == nil {
		SendError(ctx, fasthttp.StatusNotFound, "OAuth config not found")
		return
	}

	if oauthConfig.Status != "authorized" {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("OAuth not authorized yet. Current status: %s", oauthConfig.Status))
		return
	}

	// Get MCP client config from database (stored with oauth_config for multi-instance support)
	mcpClientConfig, err := h.oauthHandler.GetPendingMCPClient(oauthConfigID)
	if err != nil {
		logger.Error(fmt.Sprintf("[OAuth Complete] Failed to get pending MCP client: %v", err))
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to get pending MCP client: %v", err))
		return
	}
	if mcpClientConfig == nil {
		SendError(ctx, fasthttp.StatusNotFound, "MCP client not found in pending OAuth clients. The OAuth flow may have expired or already been completed.")
		return
	}

	// Creating MCP client config in config store
	if h.store.ConfigStore != nil {
		if err := h.store.ConfigStore.CreateMCPClientConfig(ctx, mcpClientConfig); err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to create MCP config: %v", err))
			return
		}
	}

	// Add MCP client to Bifrost (this will save to database and connect)
	if err := h.mcpManager.AddMCPClient(ctx, mcpClientConfig); err != nil {
		logger.Error(fmt.Sprintf("[OAuth Complete] Failed to connect MCP client: %v", err))
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to connect MCP client: %v", err))
		return
	}

	h.propagateClusterMCPClientChange(ctx, &ClusterConfigChange{
		Scope:           ClusterConfigScopeMCPClient,
		MCPClientID:     mcpClientConfig.ID,
		MCPClientConfig: mcpClientConfig,
	})

	// Clear pending MCP client config from oauth_config (cleanup)
	if err := h.oauthHandler.RemovePendingMCPClient(oauthConfigID); err != nil {
		logger.Warn(fmt.Sprintf("[OAuth Complete] Failed to clear pending MCP client config: %v", err))
		// Don't fail the request - the MCP client was successfully created
	}

	logger.Debug(fmt.Sprintf("[OAuth Complete] MCP client connected successfully: %s", mcpClientConfig.ID))
	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "MCP client connected successfully with OAuth",
	})
}

func (h *MCPHandler) propagateClusterMCPClientChange(ctx context.Context, change *ClusterConfigChange) {
	if h == nil || h.propagator == nil || change == nil {
		return
	}
	if err := h.propagator.PropagateClusterConfigChange(ctx, change); err != nil {
		logger.Warn("failed to propagate mcp client cluster config change for client %s: %v", change.MCPClientID, err)
	}
}

func (h *MCPHandler) propagateClusterMCPHostedToolChange(ctx context.Context, change *ClusterConfigChange) {
	if h == nil || h.propagator == nil || change == nil {
		return
	}
	if err := h.propagator.PropagateClusterConfigChange(ctx, change); err != nil {
		logger.Warn("failed to propagate hosted mcp tool cluster config change for tool %s: %v", change.MCPHostedToolID, err)
	}
}
