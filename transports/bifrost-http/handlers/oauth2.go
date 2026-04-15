// Package handlers provides HTTP request handlers for the Bifrost HTTP transport.
// This file contains OAuth 2.0 authentication flow handlers.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/fasthttp/router"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/oauth2"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// OAuth2Handler manages HTTP requests for OAuth2 operations
type OAuthHandler struct {
	client        *bifrost.Bifrost
	store         *lib.Config
	oauthProvider *oauth2.OAuth2Provider
	propagator    ClusterConfigPropagator
}

// NewOAuthHandler creates a new OAuth handler instance
func NewOAuthHandler(oauthProvider *oauth2.OAuth2Provider, client *bifrost.Bifrost, store *lib.Config, propagator ClusterConfigPropagator) *OAuthHandler {
	return &OAuthHandler{
		client:        client,
		store:         store,
		oauthProvider: oauthProvider,
		propagator:    propagator,
	}
}

// RegisterRoutes registers all OAuth-related routes
func (h *OAuthHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/oauth/configs", lib.ChainMiddlewares(h.getOAuthConfigs, middlewares...))
	r.GET("/api/oauth/callback", lib.ChainMiddlewares(h.handleOAuthCallback, middlewares...))
	r.GET("/api/oauth/config/{id}/status", lib.ChainMiddlewares(h.getOAuthConfigStatus, middlewares...))
	r.DELETE("/api/oauth/config/{id}", lib.ChainMiddlewares(h.revokeOAuthConfig, middlewares...))
}

type oauthMCPClientSummary struct {
	ClientID string  `json:"client_id"`
	Name     string  `json:"name"`
	State    *string `json:"state,omitempty"`
	AuthType string  `json:"auth_type,omitempty"`
}

type oauthConfigListItem struct {
	ID               string                 `json:"id"`
	Status           string                 `json:"status"`
	ClientID         string                 `json:"client_id"`
	AuthorizeURL     string                 `json:"authorize_url"`
	TokenURL         string                 `json:"token_url"`
	RegistrationURL  *string                `json:"registration_url,omitempty"`
	RedirectURI      string                 `json:"redirect_uri"`
	ServerURL        string                 `json:"server_url"`
	UseDiscovery     bool                   `json:"use_discovery"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
	ExpiresAt        string                 `json:"expires_at"`
	Scopes           []string               `json:"scopes,omitempty"`
	TokenID          *string                `json:"token_id,omitempty"`
	TokenExpiresAt   *string                `json:"token_expires_at,omitempty"`
	TokenScopes      []string               `json:"token_scopes,omitempty"`
	LinkedMCPClient  *oauthMCPClientSummary `json:"linked_mcp_client,omitempty"`
	PendingMCPClient *oauthMCPClientSummary `json:"pending_mcp_client,omitempty"`
	StatusURL        string                 `json:"status_url"`
	CompleteURL      string                 `json:"complete_url"`
	NextSteps        []string               `json:"next_steps,omitempty"`
}

func (h *OAuthHandler) getOAuthConfigs(ctx *fasthttp.RequestCtx) {
	if h == nil || h.store == nil || h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "OAuth config management is unavailable: config store is disabled")
		return
	}

	params := configstore.OAuthConfigsQueryParams{
		Search: strings.TrimSpace(string(ctx.QueryArgs().Peek("search"))),
		Status: strings.TrimSpace(string(ctx.QueryArgs().Peek("status"))),
	}
	if limitStr := strings.TrimSpace(string(ctx.QueryArgs().Peek("limit"))); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			SendError(ctx, fasthttp.StatusBadRequest, "Invalid limit parameter: must be a number")
			return
		}
		params.Limit = limit
	}
	if offsetStr := strings.TrimSpace(string(ctx.QueryArgs().Peek("offset"))); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			SendError(ctx, fasthttp.StatusBadRequest, "Invalid offset parameter: must be a number")
			return
		}
		params.Offset = offset
	}

	configs, totalCount, err := h.store.ConfigStore.GetOauthConfigsPaginated(ctx, params)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to get OAuth configs: %v", err))
		return
	}

	linkedClients := make(map[string]*oauthMCPClientSummary)
	if h.store.MCPConfig != nil {
		runtimeStates := map[string]string{}
		if h.client != nil {
			if runtimeClients, runtimeErr := h.client.GetMCPClients(); runtimeErr == nil {
				for _, runtimeClient := range runtimeClients {
					runtimeStates[runtimeClient.Config.ID] = string(runtimeClient.State)
				}
			}
		}
		for _, client := range h.store.MCPConfig.ClientConfigs {
			if client == nil || client.OauthConfigID == nil || strings.TrimSpace(*client.OauthConfigID) == "" {
				continue
			}
			stateValue, hasState := runtimeStates[client.ID]
			var state *string
			if hasState {
				state = &stateValue
			}
			linkedClients[*client.OauthConfigID] = &oauthMCPClientSummary{
				ClientID: client.ID,
				Name:     client.Name,
				State:    state,
				AuthType: string(client.AuthType),
			}
		}
	}

	items := make([]oauthConfigListItem, 0, len(configs))
	for _, oauthConfig := range configs {
		scopes := parseOAuthScopes(oauthConfig.Scopes)
		item := oauthConfigListItem{
			ID:              oauthConfig.ID,
			Status:          oauthConfig.Status,
			ClientID:        oauthConfig.ClientID,
			AuthorizeURL:    oauthConfig.AuthorizeURL,
			TokenURL:        oauthConfig.TokenURL,
			RegistrationURL: cloneOptionalString(oauthConfig.RegistrationURL),
			RedirectURI:     oauthConfig.RedirectURI,
			ServerURL:       oauthConfig.ServerURL,
			UseDiscovery:    oauthConfig.UseDiscovery,
			CreatedAt:       oauthConfig.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:       oauthConfig.UpdatedAt.UTC().Format(time.RFC3339),
			ExpiresAt:       oauthConfig.ExpiresAt.UTC().Format(time.RFC3339),
			Scopes:          scopes,
			StatusURL:       buildAPIURL(ctx, fmt.Sprintf("/api/oauth/config/%s/status", oauthConfig.ID)),
			CompleteURL:     buildAPIURL(ctx, fmt.Sprintf("/api/mcp/client/%s/complete-oauth", oauthConfig.ID)),
		}
		if oauthConfig.TokenID != nil {
			tokenID := *oauthConfig.TokenID
			item.TokenID = &tokenID
			if token, tokenErr := h.store.ConfigStore.GetOauthTokenByID(ctx, tokenID); tokenErr == nil && token != nil {
				tokenExpiry := token.ExpiresAt.UTC().Format(time.RFC3339)
				item.TokenExpiresAt = &tokenExpiry
				item.TokenScopes = parseOAuthScopes(token.Scopes)
			}
		}
		if linkedClient := linkedClients[oauthConfig.ID]; linkedClient != nil {
			item.LinkedMCPClient = linkedClient
		}
		if oauthConfig.MCPClientConfigJSON != nil && strings.TrimSpace(*oauthConfig.MCPClientConfigJSON) != "" {
			var pendingConfig schemas.MCPClientConfig
			if err := json.Unmarshal([]byte(*oauthConfig.MCPClientConfigJSON), &pendingConfig); err == nil {
				item.PendingMCPClient = &oauthMCPClientSummary{
					ClientID: pendingConfig.ID,
					Name:     pendingConfig.Name,
					AuthType: string(pendingConfig.AuthType),
				}
			}
		}
		item.NextSteps = buildOAuthConfigNextSteps(item)
		items = append(items, item)
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 25
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	SendJSON(ctx, map[string]any{
		"configs":     items,
		"count":       len(items),
		"total_count": totalCount,
		"limit":       limit,
		"offset":      offset,
	})
}

// handleOAuthCallback handles the OAuth provider callback
// GET /api/oauth/callback?state=xxx&code=yyy&error=zzz
func (h *OAuthHandler) handleOAuthCallback(ctx *fasthttp.RequestCtx) {
	state := string(ctx.QueryArgs().Peek("state"))
	code := string(ctx.QueryArgs().Peek("code"))
	errorParam := string(ctx.QueryArgs().Peek("error"))
	errorDescription := string(ctx.QueryArgs().Peek("error_description"))

	// Handle authorization denial
	if errorParam != "" {
		h.handleCallbackError(ctx, state, errorParam, errorDescription)
		return
	}

	// Validate required parameters
	if state == "" || code == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "Missing required parameters: state and code")
		return
	}

	// Complete OAuth flow
	if err := h.oauthProvider.CompleteOAuthFlow(context.Background(), state, code); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("OAuth flow completion failed: %v", err))
		return
	}

	// Redirect to success page (or close popup)
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("text/html")
	ctx.SetBodyString(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>OAuth Success</title>
			<script>
				// Close the popup window
				if (window.opener) {
					window.opener.postMessage({ type: 'oauth_success' }, window.location.origin);
					window.close();
				} else {
					document.getElementById('message').textContent = 'OAuth authorization successful! You can close this window.';
				}
			</script>
		</head>
		<body>
			<div style="display: flex; align-items: center; justify-content: center; height: 100vh; font-family: system-ui;">
				<div style="text-align: center;">
					<h1>✓ Authorization Successful</h1>
					<p id="message">This window will close automatically...</p>
				</div>
			</div>
		</body>
		</html>
	`)
}

// handleCallbackError handles OAuth callback errors
func (h *OAuthHandler) handleCallbackError(ctx *fasthttp.RequestCtx, state, errorParam, errorDescription string) {
	// Update OAuth config status to failed if state is provided
	if state != "" {
		oauthConfig, err := h.store.ConfigStore.GetOauthConfigByState(context.Background(), state)
		if err == nil && oauthConfig != nil {
			oauthConfig.Status = "failed"
			if updateErr := h.store.ConfigStore.UpdateOauthConfig(context.Background(), oauthConfig); updateErr == nil {
				h.propagateClusterOAuthConfig(context.Background(), oauthConfig)
			}
		}
	}

	// Show error page
	ctx.SetStatusCode(fasthttp.StatusBadRequest)
	ctx.SetContentType("text/html")
	errorMsg := errorParam
	if errorDescription != "" {
		errorMsg = fmt.Sprintf("%s: %s", errorParam, errorDescription)
	}
	// JSON-encode for safe embedding in JavaScript context (prevents JS injection)
	jsEscaped, _ := json.Marshal(errorMsg)
	// HTML-escape for safe embedding in HTML body (prevents HTML injection)
	htmlEscaped := html.EscapeString(errorMsg)
	ctx.SetBodyString(fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>OAuth Failed</title>
			<script>
				// Notify parent window
				if (window.opener) {
					window.opener.postMessage({ type: 'oauth_failed', error: %s }, window.location.origin);
					window.close();
				}
			</script>
		</head>
		<body>
			<div style="display: flex; align-items: center; justify-content: center; height: 100vh; font-family: system-ui;">
				<div style="text-align: center;">
					<h1>✗ Authorization Failed</h1>
					<p>%s</p>
					<p style="color: #666;">You can close this window.</p>
				</div>
			</div>
		</body>
		</html>
	`, jsEscaped, htmlEscaped))
}

// getOAuthConfigStatus returns the current status of an OAuth config
// GET /api/oauth/config/{id}/status
func (h *OAuthHandler) getOAuthConfigStatus(ctx *fasthttp.RequestCtx) {
	configID := ctx.UserValue("id").(string)

	oauthConfig, err := h.store.ConfigStore.GetOauthConfigByID(context.Background(), configID)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to get OAuth config: %v", err))
		return
	}

	if oauthConfig == nil {
		SendError(ctx, fasthttp.StatusNotFound, "OAuth config not found")
		return
	}

	response := map[string]interface{}{
		"id":         oauthConfig.ID,
		"status":     oauthConfig.Status,
		"created_at": oauthConfig.CreatedAt,
		"expires_at": oauthConfig.ExpiresAt,
	}

	if oauthConfig.Status == "authorized" && oauthConfig.TokenID != nil {
		response["token_id"] = *oauthConfig.TokenID

		// Get token metadata
		token, err := h.store.ConfigStore.GetOauthTokenByID(context.Background(), *oauthConfig.TokenID)
		if err == nil && token != nil {
			response["token_expires_at"] = token.ExpiresAt
			response["token_scopes"] = token.Scopes
		}
	}

	SendJSON(ctx, response)
}

// revokeOAuthConfig revokes an OAuth configuration and its associated token
// DELETE /api/oauth/config/{id}
func (h *OAuthHandler) revokeOAuthConfig(ctx *fasthttp.RequestCtx) {
	configID := ctx.UserValue("id").(string)

	if err := h.oauthProvider.RevokeToken(context.Background(), configID); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to revoke OAuth token: %v", err))
		return
	}

	SendJSON(ctx, map[string]interface{}{
		"message": "OAuth token revoked successfully",
	})
}

func parseOAuthScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var scopes []string
	if err := json.Unmarshal([]byte(raw), &scopes); err == nil {
		return scopes
	}
	parts := strings.Split(raw, " ")
	scopes = scopes[:0]
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			scopes = append(scopes, part)
		}
	}
	if len(scopes) == 0 {
		return nil
	}
	return scopes
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func buildAPIURL(ctx *fasthttp.RequestCtx, path string) string {
	if ctx == nil {
		return path
	}
	scheme := "http"
	if ctx.IsTLS() || strings.EqualFold(string(ctx.Request.Header.Peek("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}
	host := strings.TrimSpace(string(ctx.Host()))
	if host == "" {
		return path
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

func buildOAuthConfigNextSteps(item oauthConfigListItem) []string {
	switch item.Status {
	case "pending":
		if item.PendingMCPClient != nil {
			return []string{
				"在 MCP Registry 中完成此 MCP Server 的 OAuth 授权。",
				"授权成功后，Bifrost 会自动在任一节点完成状态同步，并可通过 Complete OAuth 完成挂载。",
			}
		}
		return []string{"等待 OAuth 授权完成后，再由 MCP Registry 完成 MCP Server 挂载。"}
	case "authorized":
		if item.LinkedMCPClient != nil {
			return []string{"此 OAuth 配置已经绑定到 MCP Server，可继续在 MCP Registry 中查看工具发现与连接状态。"}
		}
		return []string{"OAuth 已授权，但还没有绑定到已落库的 MCP Server。可以在 MCP Registry 中完成挂载。"}
	case "failed":
		return []string{"此 OAuth 流程已失败。建议在 MCP Registry 中重新发起授权。"}
	case "expired":
		return []string{"此 OAuth 流程已过期。建议在 MCP Registry 中重新发起授权。"}
	case "revoked":
		return []string{"此 OAuth 配置已撤销。如需恢复，请在 MCP Registry 中重新发起授权。"}
	default:
		return nil
	}
}

// OAuthInitiationRequest represents the request to initiate an OAuth flow
type OAuthInitiationRequest struct {
	ClientID        string   `json:"client_id"`
	ClientSecret    string   `json:"client_secret"`
	AuthorizeURL    string   `json:"authorize_url"`
	TokenURL        string   `json:"token_url"`
	RegistrationURL string   `json:"registration_url"`
	RedirectURI     string   `json:"redirect_uri"`
	Scopes          []string `json:"scopes"`
	ServerURL       string   `json:"server_url"` // For OAuth discovery
}

// InitiateOAuthFlow initiates an OAuth flow and returns the authorization URL
// This is called internally by the MCP client creation endpoint
func (h *OAuthHandler) InitiateOAuthFlow(ctx context.Context, req OAuthInitiationRequest) (*schemas.OAuth2FlowInitiation, error) {
	var registrationURL *string
	if req.RegistrationURL != "" {
		registrationURL = &req.RegistrationURL
	}

	config := &schemas.OAuth2Config{
		ClientID:        req.ClientID,
		ClientSecret:    req.ClientSecret,
		AuthorizeURL:    req.AuthorizeURL,
		TokenURL:        req.TokenURL,
		RegistrationURL: registrationURL,
		RedirectURI:     req.RedirectURI,
		Scopes:          req.Scopes,
		ServerURL:       req.ServerURL, // MCP server URL for OAuth discovery
	}

	return h.oauthProvider.InitiateOAuthFlow(ctx, config)
}

// StorePendingMCPClient stores an MCP client config in the database while waiting for OAuth completion
// This supports multi-instance deployments where OAuth callback may hit a different server instance
func (h *OAuthHandler) StorePendingMCPClient(oauthConfigID string, mcpClientConfig schemas.MCPClientConfig) error {
	return h.oauthProvider.StorePendingMCPClient(oauthConfigID, mcpClientConfig)
}

// GetPendingMCPClient retrieves a pending MCP client config by oauth_config_id
func (h *OAuthHandler) GetPendingMCPClient(oauthConfigID string) (*schemas.MCPClientConfig, error) {
	return h.oauthProvider.GetPendingMCPClient(oauthConfigID)
}

// GetPendingMCPClientByState retrieves a pending MCP client config by OAuth state token
func (h *OAuthHandler) GetPendingMCPClientByState(state string) (*schemas.MCPClientConfig, string, error) {
	return h.oauthProvider.GetPendingMCPClientByState(state)
}

// RemovePendingMCPClient removes a pending MCP client after OAuth completion
func (h *OAuthHandler) RemovePendingMCPClient(oauthConfigID string) error {
	return h.oauthProvider.RemovePendingMCPClient(oauthConfigID)
}

func (h *OAuthHandler) propagateClusterOAuthConfig(ctx context.Context, oauthConfig *tables.TableOauthConfig) {
	if h == nil || h.propagator == nil || oauthConfig == nil {
		return
	}
	change := &ClusterConfigChange{
		Scope:         ClusterConfigScopeOAuthConfig,
		OAuthConfigID: oauthConfig.ID,
		OAuthConfig:   CloneClusterOAuthConfig(oauthConfig),
	}
	if err := h.propagator.PropagateClusterConfigChange(ctx, change); err != nil {
		logger.Warn("failed to propagate oauth config cluster change for %s: %v", oauthConfig.ID, err)
	}
}
