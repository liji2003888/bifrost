package handlers

import (
	"context"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
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

type fakeMCPClusterStore struct {
	configstore.ConfigStore
	clientConfig *configstore.ClientConfig
}

func (s *fakeMCPClusterStore) CreateMCPClientConfig(_ context.Context, _ *schemas.MCPClientConfig) error {
	return nil
}

func (s *fakeMCPClusterStore) GetClientConfig(_ context.Context) (*configstore.ClientConfig, error) {
	return s.clientConfig, nil
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
