package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/valyala/fasthttp"
)

type authMiddlewareWebSocketStore struct {
	configstore.ConfigStore
	authConfig   *configstore.AuthConfig
	clientConfig *configstore.ClientConfig
	session      *tables.SessionsTable
}

func (s *authMiddlewareWebSocketStore) GetAuthConfig(_ context.Context) (*configstore.AuthConfig, error) {
	return s.authConfig, nil
}

func (s *authMiddlewareWebSocketStore) GetClientConfig(_ context.Context) (*configstore.ClientConfig, error) {
	return s.clientConfig, nil
}

func (s *authMiddlewareWebSocketStore) GetSession(_ context.Context, token string) (*tables.SessionsTable, error) {
	if s.session != nil && s.session.Token == token {
		cloned := *s.session
		return &cloned, nil
	}
	return nil, nil
}

func TestAuthMiddleware_WebSocketInvalidTicketFallsBackToCookieSession(t *testing.T) {
	SetLogger(&mockLogger{})

	store := &authMiddlewareWebSocketStore{
		authConfig: &configstore.AuthConfig{
			AdminUserName: schemas.NewEnvVar("admin"),
			AdminPassword: schemas.NewEnvVar("hashedpassword"),
			IsEnabled:     true,
		},
		clientConfig: &configstore.ClientConfig{},
		session: &tables.SessionsTable{
			Token:     "dashboard-session-token",
			ExpiresAt: time.Now().Add(5 * time.Minute),
			CreatedAt: time.Now().Add(-time.Minute),
			UpdatedAt: time.Now(),
		},
	}
	wsTickets := NewWSTicketStore()
	defer wsTickets.Stop()

	am, err := InitAuthMiddleware(store, wsTickets)
	if err != nil {
		t.Fatalf("InitAuthMiddleware() error = %v", err)
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/ws?ticket=invalid-ticket")
	ctx.Request.Header.Set("Upgrade", "websocket")
	ctx.Request.Header.SetCookie("token", "dashboard-session-token")

	nextCalled := false
	next := func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	}

	handler := am.APIMiddleware()(next)
	handler(ctx)

	if !nextCalled {
		t.Fatal("expected websocket auth to fall back to cookie-backed session")
	}
	if ctx.Response.StatusCode() == fasthttp.StatusUnauthorized {
		t.Fatalf("expected websocket auth not to reject valid cookie fallback, got %d", ctx.Response.StatusCode())
	}
}
