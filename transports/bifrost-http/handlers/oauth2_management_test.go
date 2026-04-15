package handlers

import (
	"context"
	"encoding/json"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fasthttp/router"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/oauth2"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type oauthManagementTestStore struct {
	configstore.ConfigStore
	configs map[string]*configstoreTables.TableOauthConfig
	tokens  map[string]*configstoreTables.TableOauthToken
}

func newOAuthManagementTestStore() *oauthManagementTestStore {
	return &oauthManagementTestStore{
		configs: map[string]*configstoreTables.TableOauthConfig{},
		tokens:  map[string]*configstoreTables.TableOauthToken{},
	}
}

func (s *oauthManagementTestStore) GetOauthConfigsPaginated(_ context.Context, params configstore.OAuthConfigsQueryParams) ([]configstoreTables.TableOauthConfig, int64, error) {
	configs := make([]configstoreTables.TableOauthConfig, 0, len(s.configs))
	for _, cfg := range s.configs {
		if cfg == nil {
			continue
		}
		if params.Status != "" && cfg.Status != params.Status {
			continue
		}
		if params.Search != "" {
			search := strings.ToLower(params.Search)
			if !strings.Contains(strings.ToLower(cfg.ClientID), search) &&
				!strings.Contains(strings.ToLower(cfg.ServerURL), search) &&
				!strings.Contains(strings.ToLower(cfg.AuthorizeURL), search) {
				continue
			}
		}
		configs = append(configs, *cloneOAuthConfigRecord(cfg))
	}
	slices.SortFunc(configs, func(a, b configstoreTables.TableOauthConfig) int {
		if a.CreatedAt.Equal(b.CreatedAt) {
			return strings.Compare(a.ID, b.ID)
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return -1
		}
		return 1
	})
	totalCount := int64(len(configs))
	limit := params.Limit
	if limit <= 0 {
		limit = 25
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(configs) {
		return []configstoreTables.TableOauthConfig{}, totalCount, nil
	}
	end := offset + limit
	if end > len(configs) {
		end = len(configs)
	}
	return configs[offset:end], totalCount, nil
}

func (s *oauthManagementTestStore) GetOauthConfigByID(_ context.Context, id string) (*configstoreTables.TableOauthConfig, error) {
	cfg := s.configs[id]
	if cfg == nil {
		return nil, nil
	}
	return cloneOAuthConfigRecord(cfg), nil
}

func (s *oauthManagementTestStore) GetOauthConfigByState(_ context.Context, state string) (*configstoreTables.TableOauthConfig, error) {
	for _, cfg := range s.configs {
		if cfg != nil && cfg.State == state {
			return cloneOAuthConfigRecord(cfg), nil
		}
	}
	return nil, nil
}

func (s *oauthManagementTestStore) CreateOauthConfig(_ context.Context, config *configstoreTables.TableOauthConfig) error {
	s.configs[config.ID] = cloneOAuthConfigRecord(config)
	return nil
}

func (s *oauthManagementTestStore) UpdateOauthConfig(_ context.Context, config *configstoreTables.TableOauthConfig) error {
	s.configs[config.ID] = cloneOAuthConfigRecord(config)
	return nil
}

func (s *oauthManagementTestStore) GetOauthTokenByID(_ context.Context, id string) (*configstoreTables.TableOauthToken, error) {
	token := s.tokens[id]
	if token == nil {
		return nil, nil
	}
	return cloneOAuthTokenRecord(token), nil
}

func (s *oauthManagementTestStore) CreateOauthToken(_ context.Context, token *configstoreTables.TableOauthToken) error {
	s.tokens[token.ID] = cloneOAuthTokenRecord(token)
	return nil
}

func (s *oauthManagementTestStore) UpdateOauthToken(_ context.Context, token *configstoreTables.TableOauthToken) error {
	s.tokens[token.ID] = cloneOAuthTokenRecord(token)
	return nil
}

func (s *oauthManagementTestStore) DeleteOauthToken(_ context.Context, id string) error {
	delete(s.tokens, id)
	return nil
}

type noopMCPManager struct{}

func (noopMCPManager) AddMCPClient(_ context.Context, _ *schemas.MCPClientConfig) error { return nil }
func (noopMCPManager) RemoveMCPClient(_ context.Context, _ string) error                { return nil }
func (noopMCPManager) UpdateMCPClient(_ context.Context, _ string, _ *schemas.MCPClientConfig) error {
	return nil
}
func (noopMCPManager) ReconnectMCPClient(_ context.Context, _ string) error { return nil }
func (noopMCPManager) AddMCPHostedTool(_ context.Context, _ *configstoreTables.TableMCPHostedTool) error {
	return nil
}
func (noopMCPManager) UpdateMCPHostedTool(_ context.Context, _ string, _ *configstoreTables.TableMCPHostedTool) error {
	return nil
}
func (noopMCPManager) RemoveMCPHostedTool(_ context.Context, _ string) error { return nil }
func (noopMCPManager) PreviewMCPHostedTool(_ context.Context, _ string, _ map[string]any) (*configstoreTables.MCPHostedToolExecutionResult, error) {
	return nil, nil
}

func cloneOAuthConfigRecord(cfg *configstoreTables.TableOauthConfig) *configstoreTables.TableOauthConfig {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	if cfg.RegistrationURL != nil {
		registrationURL := *cfg.RegistrationURL
		cloned.RegistrationURL = &registrationURL
	}
	if cfg.TokenID != nil {
		tokenID := *cfg.TokenID
		cloned.TokenID = &tokenID
	}
	if cfg.MCPClientConfigJSON != nil {
		jsonValue := *cfg.MCPClientConfigJSON
		cloned.MCPClientConfigJSON = &jsonValue
	}
	return &cloned
}

func cloneOAuthTokenRecord(token *configstoreTables.TableOauthToken) *configstoreTables.TableOauthToken {
	if token == nil {
		return nil
	}
	cloned := *token
	if token.LastRefreshedAt != nil {
		lastRefreshedAt := *token.LastRefreshedAt
		cloned.LastRefreshedAt = &lastRefreshedAt
	}
	return &cloned
}

func newInmemoryHTTPClient(t *testing.T, handler func(r *router.Router)) (*fasthttp.Client, func()) {
	t.Helper()
	testRouter := router.New()
	handler(testRouter)

	listener := fasthttputil.NewInmemoryListener()
	server := &fasthttp.Server{Handler: testRouter.Handler}
	go server.Serve(listener) //nolint:errcheck

	client := &fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) {
			return listener.Dial()
		},
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	cleanup := func() {
		_ = server.Shutdown()
		_ = listener.Close()
	}
	return client, cleanup
}

func TestAddMCPClientOAuthResponseIncludesGuidance(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newOAuthManagementTestStore()
	oauthProvider := oauth2.NewOAuth2Provider(store, bifrost.NewNoOpLogger())
	oauthHandler := NewOAuthHandler(oauthProvider, nil, &lib.Config{ConfigStore: store}, &fakeClusterConfigPropagator{})
	mcpHandler := NewMCPHandler(noopMCPManager{}, nil, &lib.Config{ConfigStore: store}, oauthHandler, &fakeClusterConfigPropagator{})

	client, cleanup := newInmemoryHTTPClient(t, func(r *router.Router) {
		mcpHandler.RegisterRoutes(r)
	})
	defer cleanup()

	body := map[string]any{
		"name":               "docsmcp",
		"connection_type":    "http",
		"connection_string":  map[string]any{"value": "https://mcp.example.com", "from_env": false, "env_var": ""},
		"auth_type":          "oauth",
		"tool_sync_interval": 5,
		"oauth_config": map[string]any{
			"client_id":     "client-id",
			"client_secret": "client-secret",
			"authorize_url": "https://auth.example.com/authorize",
			"token_url":     "https://auth.example.com/token",
			"scopes":        []string{"tools.read"},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://gateway.local/api/mcp/client")
	req.SetBody(payload)

	if err := client.Do(req, resp); err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var response struct {
		Status        string   `json:"status"`
		OAuthConfigID string   `json:"oauth_config_id"`
		AuthorizeURL  string   `json:"authorize_url"`
		StatusURL     string   `json:"status_url"`
		CompleteURL   string   `json:"complete_url"`
		NextSteps     []string `json:"next_steps"`
	}
	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.Status != "pending_oauth" {
		t.Fatalf("expected pending_oauth response, got %+v", response)
	}
	if response.OAuthConfigID == "" || response.AuthorizeURL == "" {
		t.Fatalf("expected oauth initiation details, got %+v", response)
	}
	if !strings.Contains(response.StatusURL, "/api/oauth/config/") || !strings.Contains(response.CompleteURL, "/api/mcp/client/") {
		t.Fatalf("expected status/complete urls, got %+v", response)
	}
	if len(response.NextSteps) != 3 {
		t.Fatalf("expected next steps guidance, got %+v", response.NextSteps)
	}
}

func TestGetOAuthConfigsListsPendingAndLinkedClients(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newOAuthManagementTestStore()
	tokenID := "oauth-token-1"
	store.tokens[tokenID] = &configstoreTables.TableOauthToken{
		ID:        tokenID,
		TokenType: "Bearer",
		Scopes:    `["tools.read","tools.write"]`,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	pendingConfigJSON := `{"id":"pending-mcp-client","name":"Pending Docs MCP","auth_type":"oauth"}`
	store.configs["oauth-authorized"] = &configstoreTables.TableOauthConfig{
		ID:           "oauth-authorized",
		ClientID:     "client-id",
		AuthorizeURL: "https://auth.example.com/authorize",
		TokenURL:     "https://auth.example.com/token",
		RedirectURI:  "https://gateway.example.com/api/oauth/callback",
		ServerURL:    "https://mcp.example.com",
		Status:       "authorized",
		TokenID:      &tokenID,
		Scopes:       `["tools.read"]`,
		CreatedAt:    time.Now().Add(-2 * time.Hour),
		UpdatedAt:    time.Now().Add(-time.Minute),
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	store.configs["oauth-pending"] = &configstoreTables.TableOauthConfig{
		ID:                  "oauth-pending",
		ClientID:            "pending-client-id",
		AuthorizeURL:        "https://auth.example.com/authorize",
		TokenURL:            "https://auth.example.com/token",
		RedirectURI:         "https://gateway.example.com/api/oauth/callback",
		ServerURL:           "https://pending-mcp.example.com",
		Status:              "pending",
		MCPClientConfigJSON: &pendingConfigJSON,
		CreatedAt:           time.Now().Add(-time.Hour),
		UpdatedAt:           time.Now().Add(-30 * time.Second),
		ExpiresAt:           time.Now().Add(5 * time.Minute),
	}

	oauthConfigID := "oauth-authorized"
	cfg := &lib.Config{
		ConfigStore: store,
		MCPConfig: &schemas.MCPConfig{
			ClientConfigs: []*schemas.MCPClientConfig{
				{
					ID:            "linked-mcp-client",
					Name:          "CRM MCP",
					AuthType:      schemas.MCPAuthType("oauth"),
					OauthConfigID: &oauthConfigID,
				},
			},
		},
	}
	oauthHandler := NewOAuthHandler(nil, nil, cfg, nil)

	client, cleanup := newInmemoryHTTPClient(t, func(r *router.Router) {
		oauthHandler.RegisterRoutes(r)
	})
	defer cleanup()

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodGet)
	req.SetRequestURI("http://gateway.local/api/oauth/configs")

	if err := client.Do(req, resp); err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var response struct {
		Configs []struct {
			ID              string `json:"id"`
			Status          string `json:"status"`
			LinkedMCPClient *struct {
				ClientID string `json:"client_id"`
				Name     string `json:"name"`
			} `json:"linked_mcp_client"`
			PendingMCPClient *struct {
				ClientID string `json:"client_id"`
				Name     string `json:"name"`
			} `json:"pending_mcp_client"`
			TokenExpiresAt *string  `json:"token_expires_at"`
			NextSteps      []string `json:"next_steps"`
		} `json:"configs"`
		TotalCount int64 `json:"total_count"`
	}
	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if response.TotalCount != 2 || len(response.Configs) != 2 {
		t.Fatalf("expected 2 oauth configs, got %+v", response)
	}

	var sawAuthorized, sawPending bool
	for _, record := range response.Configs {
		switch record.ID {
		case "oauth-authorized":
			sawAuthorized = true
			if record.LinkedMCPClient == nil || record.LinkedMCPClient.Name != "CRM MCP" {
				t.Fatalf("expected linked MCP client summary, got %+v", record.LinkedMCPClient)
			}
			if record.TokenExpiresAt == nil || *record.TokenExpiresAt == "" {
				t.Fatalf("expected token metadata, got %+v", record)
			}
		case "oauth-pending":
			sawPending = true
			if record.PendingMCPClient == nil || record.PendingMCPClient.Name != "Pending Docs MCP" {
				t.Fatalf("expected pending MCP client summary, got %+v", record.PendingMCPClient)
			}
			if len(record.NextSteps) == 0 {
				t.Fatalf("expected next steps for pending config, got %+v", record.NextSteps)
			}
		}
	}
	if !sawAuthorized || !sawPending {
		t.Fatalf("expected both authorized and pending configs, got %+v", response.Configs)
	}
}

func TestRevokeOAuthConfigHandlesPendingConfigWithoutToken(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newOAuthManagementTestStore()
	pendingConfigJSON := `{"id":"pending-mcp-client","name":"Pending Docs MCP","auth_type":"oauth"}`
	store.configs["oauth-pending"] = &configstoreTables.TableOauthConfig{
		ID:                  "oauth-pending",
		ClientID:            "pending-client-id",
		AuthorizeURL:        "https://auth.example.com/authorize",
		TokenURL:            "https://auth.example.com/token",
		RedirectURI:         "https://gateway.example.com/api/oauth/callback",
		ServerURL:           "https://pending-mcp.example.com",
		Status:              "pending",
		MCPClientConfigJSON: &pendingConfigJSON,
		ExpiresAt:           time.Now().Add(5 * time.Minute),
	}

	oauthProvider := oauth2.NewOAuth2Provider(store, bifrost.NewNoOpLogger())
	oauthHandler := NewOAuthHandler(oauthProvider, nil, &lib.Config{ConfigStore: store}, nil)

	client, cleanup := newInmemoryHTTPClient(t, func(r *router.Router) {
		oauthHandler.RegisterRoutes(r)
	})
	defer cleanup()

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(fasthttp.MethodDelete)
	req.SetRequestURI("http://gateway.local/api/oauth/config/oauth-pending")

	if err := client.Do(req, resp); err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	updated := store.configs["oauth-pending"]
	if updated == nil || updated.Status != "revoked" {
		t.Fatalf("expected pending oauth config to be revoked, got %+v", updated)
	}
	if updated.MCPClientConfigJSON != nil {
		t.Fatalf("expected pending mcp client config to be cleared, got %+v", updated.MCPClientConfigJSON)
	}
}
