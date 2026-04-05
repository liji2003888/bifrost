package server

import (
	"context"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

func TestApplyClusterOAuthConfigAndToken(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newClusterPluginApplyStore(t)
	server := &BifrostHTTPServer{
		Config: &lib.Config{ConfigStore: store},
	}

	token := &handlers.ClusterOAuthToken{
		ID:           "oauth-token-1",
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    time.Unix(1700003600, 0).UTC(),
		Scopes:       `["read","write"]`,
	}
	if err := server.ApplyClusterOAuthToken(context.Background(), token.ID, token, false); err != nil {
		t.Fatalf("ApplyClusterOAuthToken(create) error = %v", err)
	}

	storedToken, err := store.GetOauthTokenByID(context.Background(), token.ID)
	if err != nil {
		t.Fatalf("GetOauthTokenByID() error = %v", err)
	}
	if storedToken == nil || storedToken.AccessToken != "access-token" || storedToken.RefreshToken != "refresh-token" {
		t.Fatalf("unexpected stored oauth token: %+v", storedToken)
	}

	oauthConfig := &handlers.ClusterOAuthConfig{
		ID:                  "oauth-config-1",
		ClientID:            "client-id",
		ClientSecret:        "client-secret",
		AuthorizeURL:        "https://auth.example.com/authorize",
		TokenURL:            "https://auth.example.com/token",
		RedirectURI:         "https://gateway.example.com/api/oauth/callback",
		Scopes:              `["read","write"]`,
		State:               "oauth-state",
		CodeVerifier:        "code-verifier",
		CodeChallenge:       "code-challenge",
		Status:              "authorized",
		TokenID:             bifrost.Ptr(token.ID),
		ServerURL:           "https://mcp.example.com",
		UseDiscovery:        true,
		MCPClientConfigJSON: bifrost.Ptr(`{"id":"mcp-client-1","name":"docs-client"}`),
		ExpiresAt:           time.Unix(1700001800, 0).UTC(),
	}
	if err := server.ApplyClusterOAuthConfig(context.Background(), oauthConfig.ID, oauthConfig); err != nil {
		t.Fatalf("ApplyClusterOAuthConfig(create) error = %v", err)
	}

	storedConfig, err := store.GetOauthConfigByID(context.Background(), oauthConfig.ID)
	if err != nil {
		t.Fatalf("GetOauthConfigByID() error = %v", err)
	}
	if storedConfig == nil || storedConfig.Status != "authorized" || storedConfig.TokenID == nil || *storedConfig.TokenID != token.ID {
		t.Fatalf("unexpected stored oauth config: %+v", storedConfig)
	}
	if storedConfig.MCPClientConfigJSON == nil || *storedConfig.MCPClientConfigJSON == "" {
		t.Fatalf("expected pending mcp client config to be preserved, got %+v", storedConfig)
	}

	if err := server.ApplyClusterOAuthToken(context.Background(), token.ID, nil, true); err != nil {
		t.Fatalf("ApplyClusterOAuthToken(delete) error = %v", err)
	}
	deletedToken, err := store.GetOauthTokenByID(context.Background(), token.ID)
	if err != nil {
		t.Fatalf("GetOauthTokenByID(after delete) error = %v", err)
	}
	if deletedToken != nil {
		t.Fatalf("expected oauth token to be deleted, got %+v", deletedToken)
	}
}
