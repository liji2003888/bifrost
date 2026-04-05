package handlers

import (
	"context"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/encrypt"
	"github.com/valyala/fasthttp"
)

type fakeSessionClusterStore struct {
	configstore.ConfigStore
	authConfig *configstore.AuthConfig
	sessions   map[string]*tables.SessionsTable
}

func newFakeSessionClusterStore(t *testing.T) *fakeSessionClusterStore {
	t.Helper()

	passwordHash, err := encrypt.Hash("super-secret")
	if err != nil {
		t.Fatalf("encrypt.Hash() error = %v", err)
	}
	return &fakeSessionClusterStore{
		authConfig: &configstore.AuthConfig{
			AdminUserName: schemas.NewEnvVar("admin"),
			AdminPassword: schemas.NewEnvVar(passwordHash),
			IsEnabled:     true,
		},
		sessions: make(map[string]*tables.SessionsTable),
	}
}

func (s *fakeSessionClusterStore) GetAuthConfig(_ context.Context) (*configstore.AuthConfig, error) {
	return s.authConfig, nil
}

func (s *fakeSessionClusterStore) CreateSession(_ context.Context, session *tables.SessionsTable) error {
	cloned := *session
	s.sessions[session.Token] = &cloned
	return nil
}

func (s *fakeSessionClusterStore) GetSession(_ context.Context, token string) (*tables.SessionsTable, error) {
	session, ok := s.sessions[token]
	if !ok {
		return nil, configstore.ErrNotFound
	}
	cloned := *session
	return &cloned, nil
}

func (s *fakeSessionClusterStore) DeleteSession(_ context.Context, token string) error {
	if _, ok := s.sessions[token]; !ok {
		return configstore.ErrNotFound
	}
	delete(s.sessions, token)
	return nil
}

type fakeSessionClusterPropagator struct {
	changes []*ClusterConfigChange
}

func (p *fakeSessionClusterPropagator) PropagateClusterConfigChange(_ context.Context, change *ClusterConfigChange) error {
	if p == nil || change == nil {
		return nil
	}
	cloned := *change
	p.changes = append(p.changes, &cloned)
	return nil
}

func TestLoginPropagatesClusterSessionCreate(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newFakeSessionClusterStore(t)
	propagator := &fakeSessionClusterPropagator{}
	handler := NewSessionHandler(store, nil, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/session/login")
	ctx.Request.SetBodyString(`{"username":"admin","password":"super-secret"}`)

	handler.login(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(store.sessions) != 1 {
		t.Fatalf("expected one local session, got %d", len(store.sessions))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated change, got %d", len(propagator.changes))
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopeSession || change.SessionConfig == nil || change.SessionToken == "" || change.Delete {
		t.Fatalf("unexpected propagated session create: %+v", change)
	}
	if _, ok := store.sessions[change.SessionToken]; !ok {
		t.Fatalf("expected propagated token to exist locally, got %+v", change)
	}
}

func TestLogoutPropagatesClusterSessionDelete(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newFakeSessionClusterStore(t)
	store.sessions["token-1"] = &tables.SessionsTable{
		Token:     "token-1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	propagator := &fakeSessionClusterPropagator{}
	handler := NewSessionHandler(store, nil, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/session/logout")
	ctx.Request.Header.Set("Authorization", "Bearer token-1")

	handler.logout(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated change, got %d", len(propagator.changes))
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopeSession || change.SessionToken != "token-1" || !change.Delete {
		t.Fatalf("unexpected propagated session delete: %+v", change)
	}
	if _, ok := store.sessions["token-1"]; ok {
		t.Fatalf("expected local session to be deleted, got %+v", store.sessions["token-1"])
	}
}
