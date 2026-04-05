package server

import (
	"context"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type authClusterConfigStore struct {
	configstore.ConfigStore

	authConfig          *configstore.AuthConfig
	clientConfig        *configstore.ClientConfig
	flushSessionsCalled bool
}

func (m *authClusterConfigStore) GetAuthConfig(_ context.Context) (*configstore.AuthConfig, error) {
	return m.authConfig, nil
}

func (m *authClusterConfigStore) UpdateAuthConfig(_ context.Context, config *configstore.AuthConfig) error {
	if config == nil {
		m.authConfig = nil
		return nil
	}
	cloned := *config
	m.authConfig = &cloned
	return nil
}

func (m *authClusterConfigStore) GetClientConfig(_ context.Context) (*configstore.ClientConfig, error) {
	return m.clientConfig, nil
}

func (m *authClusterConfigStore) FlushSessions(_ context.Context) error {
	m.flushSessionsCalled = true
	return nil
}

func TestApplyClusterConfigChangeAuthUpdatesRuntimeAndFlushesSessions(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	handlers.SetLogger(bifrost.NewNoOpLogger())

	store := &authClusterConfigStore{
		clientConfig: &configstore.ClientConfig{},
	}
	authMiddleware, err := handlers.InitAuthMiddleware(store, nil)
	if err != nil {
		t.Fatalf("InitAuthMiddleware() error = %v", err)
	}

	server := &BifrostHTTPServer{
		Config: &lib.Config{
			ConfigStore: store,
		},
		AuthMiddleware: authMiddleware,
	}

	assertAPIMiddlewarePassesWithoutAuth(t, authMiddleware)

	change := &handlers.ClusterConfigChange{
		Scope: handlers.ClusterConfigScopeAuth,
		AuthConfig: &configstore.AuthConfig{
			AdminUserName:          schemas.NewEnvVar("admin"),
			AdminPassword:          schemas.NewEnvVar("stored-hash"),
			IsEnabled:              true,
			DisableAuthOnInference: false,
		},
		FlushSessions: true,
	}

	if err := server.ApplyClusterConfigChange(context.Background(), change); err != nil {
		t.Fatalf("ApplyClusterConfigChange() error = %v", err)
	}
	if store.authConfig == nil || !store.authConfig.IsEnabled {
		t.Fatalf("expected auth config to be persisted, got %+v", store.authConfig)
	}
	if !store.flushSessionsCalled {
		t.Fatal("expected auth change to flush existing sessions on peer")
	}

	assertAPIMiddlewareRejectsWithoutAuth(t, authMiddleware)
}

func assertAPIMiddlewarePassesWithoutAuth(t *testing.T, middleware *handlers.AuthMiddleware) {
	t.Helper()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/api/config")

	nextCalled := false
	handler := middleware.APIMiddleware()(func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	})
	handler(ctx)

	if !nextCalled {
		t.Fatalf("expected request to pass through auth middleware, got status %d", ctx.Response.StatusCode())
	}
}

func assertAPIMiddlewareRejectsWithoutAuth(t *testing.T, middleware *handlers.AuthMiddleware) {
	t.Helper()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI("/api/config")

	nextCalled := false
	handler := middleware.APIMiddleware()(func(ctx *fasthttp.RequestCtx) {
		nextCalled = true
	})
	handler(ctx)

	if nextCalled {
		t.Fatal("expected auth middleware to stop unauthenticated request after auth sync")
	}
	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("expected 401 after auth sync, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
}
