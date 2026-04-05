package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/configstore/tables"
)

type oauthTestSyncHook struct {
	configs      []*tables.TableOauthConfig
	tokens       []*tables.TableOauthToken
	tokenDeletes []string
}

func (h *oauthTestSyncHook) OAuthConfigChanged(_ context.Context, config *tables.TableOauthConfig) error {
	h.configs = append(h.configs, cloneOAuthConfig(config))
	return nil
}

func (h *oauthTestSyncHook) OAuthTokenChanged(_ context.Context, token *tables.TableOauthToken, deleted bool) error {
	if deleted {
		h.tokenDeletes = append(h.tokenDeletes, token.ID)
		return nil
	}
	h.tokens = append(h.tokens, cloneOAuthToken(token))
	return nil
}

type oauthTestStore struct {
	configstore.ConfigStore
	configs map[string]*tables.TableOauthConfig
	tokens  map[string]*tables.TableOauthToken
}

func newOAuthTestStore() *oauthTestStore {
	return &oauthTestStore{
		configs: map[string]*tables.TableOauthConfig{},
		tokens:  map[string]*tables.TableOauthToken{},
	}
}

func (s *oauthTestStore) GetOauthConfigByID(_ context.Context, id string) (*tables.TableOauthConfig, error) {
	config := s.configs[id]
	if config == nil {
		return nil, nil
	}
	return cloneOAuthConfig(config), nil
}

func (s *oauthTestStore) GetOauthConfigByState(_ context.Context, state string) (*tables.TableOauthConfig, error) {
	for _, config := range s.configs {
		if config.State == state {
			return cloneOAuthConfig(config), nil
		}
	}
	return nil, nil
}

func (s *oauthTestStore) CreateOauthConfig(_ context.Context, config *tables.TableOauthConfig) error {
	s.configs[config.ID] = cloneOAuthConfig(config)
	return nil
}

func (s *oauthTestStore) UpdateOauthConfig(_ context.Context, config *tables.TableOauthConfig) error {
	s.configs[config.ID] = cloneOAuthConfig(config)
	return nil
}

func (s *oauthTestStore) GetOauthTokenByID(_ context.Context, id string) (*tables.TableOauthToken, error) {
	token := s.tokens[id]
	if token == nil {
		return nil, nil
	}
	return cloneOAuthToken(token), nil
}

func (s *oauthTestStore) CreateOauthToken(_ context.Context, token *tables.TableOauthToken) error {
	s.tokens[token.ID] = cloneOAuthToken(token)
	return nil
}

func (s *oauthTestStore) UpdateOauthToken(_ context.Context, token *tables.TableOauthToken) error {
	s.tokens[token.ID] = cloneOAuthToken(token)
	return nil
}

func (s *oauthTestStore) DeleteOauthToken(_ context.Context, id string) error {
	delete(s.tokens, id)
	return nil
}

func TestOAuthSyncHookReceivesLifecycleUpdates(t *testing.T) {
	store := newOAuthTestStore()
	hook := &oauthTestSyncHook{}
	provider := NewOAuth2Provider(store, bifrost.NewNoOpLogger())
	provider.SetSyncHook(hook)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Method; got != http.MethodPost {
			t.Fatalf("expected POST token exchange, got %s", got)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"scope":         "read write",
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer tokenServer.Close()

	flow, err := provider.InitiateOAuthFlow(context.Background(), &schemas.OAuth2Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		AuthorizeURL: "https://auth.example.com/authorize",
		TokenURL:     tokenServer.URL,
		RedirectURI:  "https://gateway.example.com/api/oauth/callback",
		Scopes:       []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("InitiateOAuthFlow() error = %v", err)
	}
	if len(hook.configs) == 0 || hook.configs[len(hook.configs)-1].Status != "pending" {
		t.Fatalf("expected pending oauth config sync, got %+v", hook.configs)
	}

	pendingClient := schemas.MCPClientConfig{
		ID:   "mcp-client-1",
		Name: "docs-client",
	}
	if err := provider.StorePendingMCPClient(flow.OauthConfigID, pendingClient); err != nil {
		t.Fatalf("StorePendingMCPClient() error = %v", err)
	}
	lastConfig := hook.configs[len(hook.configs)-1]
	if lastConfig.MCPClientConfigJSON == nil {
		t.Fatalf("expected pending MCP client config to be synced, got %+v", lastConfig)
	}
	var syncedMCPClient schemas.MCPClientConfig
	if err := json.Unmarshal([]byte(*lastConfig.MCPClientConfigJSON), &syncedMCPClient); err != nil {
		t.Fatalf("json.Unmarshal(MCPClientConfigJSON) error = %v", err)
	}
	if syncedMCPClient.ID != "mcp-client-1" || syncedMCPClient.Name != "docs-client" {
		t.Fatalf("unexpected synced pending MCP client config: %+v", syncedMCPClient)
	}

	if err := provider.CompleteOAuthFlow(context.Background(), flow.State, "auth-code"); err != nil {
		t.Fatalf("CompleteOAuthFlow() error = %v", err)
	}
	if len(hook.tokens) == 0 || hook.tokens[len(hook.tokens)-1].AccessToken != "access-token" {
		t.Fatalf("expected oauth token sync after completion, got %+v", hook.tokens)
	}
	if last := hook.configs[len(hook.configs)-1]; last.Status != "authorized" || last.TokenID == nil {
		t.Fatalf("expected authorized oauth config sync, got %+v", last)
	}

	if err := provider.RemovePendingMCPClient(flow.OauthConfigID); err != nil {
		t.Fatalf("RemovePendingMCPClient() error = %v", err)
	}
	if last := hook.configs[len(hook.configs)-1]; last.MCPClientConfigJSON != nil {
		t.Fatalf("expected pending MCP client config to be cleared, got %+v", last)
	}

	if err := provider.RevokeToken(context.Background(), flow.OauthConfigID); err != nil {
		t.Fatalf("RevokeToken() error = %v", err)
	}
	if len(hook.tokenDeletes) == 0 || hook.tokenDeletes[len(hook.tokenDeletes)-1] == "" {
		t.Fatalf("expected oauth token delete sync, got %+v", hook.tokenDeletes)
	}
	if last := hook.configs[len(hook.configs)-1]; last.Status != "revoked" || last.TokenID != nil {
		t.Fatalf("expected revoked oauth config sync, got %+v", last)
	}
}

func TestOAuthSyncHookReceivesRefreshUpdates(t *testing.T) {
	store := newOAuthTestStore()
	hook := &oauthTestSyncHook{}
	provider := NewOAuth2Provider(store, bifrost.NewNoOpLogger())
	provider.SetSyncHook(hook)

	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-access-token",
			"refresh_token": "refreshed-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    7200,
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer refreshServer.Close()

	tokenID := "oauth-token-refresh"
	store.tokens[tokenID] = &tables.TableOauthToken{
		ID:           tokenID,
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(-time.Minute),
	}
	store.configs["oauth-config-refresh"] = &tables.TableOauthConfig{
		ID:           "oauth-config-refresh",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		TokenURL:     refreshServer.URL,
		Status:       "authorized",
		TokenID:      bifrost.Ptr(tokenID),
	}

	if err := provider.RefreshAccessToken(context.Background(), "oauth-config-refresh"); err != nil {
		t.Fatalf("RefreshAccessToken() error = %v", err)
	}
	if len(hook.tokens) == 0 {
		t.Fatal("expected refresh to trigger oauth token sync")
	}
	lastToken := hook.tokens[len(hook.tokens)-1]
	if lastToken.AccessToken != "refreshed-access-token" || lastToken.RefreshToken != "refreshed-refresh-token" {
		t.Fatalf("unexpected refreshed token sync payload: %+v", lastToken)
	}
}
