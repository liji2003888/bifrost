package handlers

import (
	"context"
	"encoding/json"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

type clusterTestPluginsLoader struct {
	reloaded []string
	removed  []string
}

func (l *clusterTestPluginsLoader) ReloadPlugin(_ context.Context, name string, _ *string, _ any, _ *schemas.PluginPlacement, _ *int) error {
	l.reloaded = append(l.reloaded, name)
	return nil
}

func (l *clusterTestPluginsLoader) RemovePlugin(_ context.Context, name string) error {
	l.removed = append(l.removed, name)
	return nil
}

func (l *clusterTestPluginsLoader) GetPluginStatus(_ context.Context) map[string]schemas.PluginStatus {
	return map[string]schemas.PluginStatus{}
}

type clusterTestPluginPropagator struct {
	changes []*ClusterConfigChange
}

func (p *clusterTestPluginPropagator) PropagateClusterConfigChange(_ context.Context, change *ClusterConfigChange) error {
	if p == nil || change == nil {
		return nil
	}
	cloned := *change
	p.changes = append(p.changes, &cloned)
	return nil
}

type inMemoryPluginsClusterStore struct {
	configstore.ConfigStore
	plugins map[string]*configstoreTables.TablePlugin
}

func newPluginsClusterTestStore() *inMemoryPluginsClusterStore {
	return &inMemoryPluginsClusterStore{
		plugins: map[string]*configstoreTables.TablePlugin{},
	}
}

func (s *inMemoryPluginsClusterStore) GetPlugin(_ context.Context, name string) (*configstoreTables.TablePlugin, error) {
	plugin, ok := s.plugins[name]
	if !ok {
		return nil, configstore.ErrNotFound
	}
	return clonePluginsClusterRecord(plugin), nil
}

func (s *inMemoryPluginsClusterStore) CreatePlugin(_ context.Context, plugin *configstoreTables.TablePlugin, _ ...*gorm.DB) error {
	if plugin == nil {
		return nil
	}
	if _, exists := s.plugins[plugin.Name]; exists {
		return configstore.ErrAlreadyExists
	}
	s.plugins[plugin.Name] = clonePluginsClusterRecord(plugin)
	return nil
}

func (s *inMemoryPluginsClusterStore) UpdatePlugin(_ context.Context, plugin *configstoreTables.TablePlugin, _ ...*gorm.DB) error {
	if plugin == nil {
		return nil
	}
	if _, exists := s.plugins[plugin.Name]; !exists {
		return configstore.ErrNotFound
	}
	s.plugins[plugin.Name] = clonePluginsClusterRecord(plugin)
	return nil
}

func (s *inMemoryPluginsClusterStore) DeletePlugin(_ context.Context, name string, _ ...*gorm.DB) error {
	if _, exists := s.plugins[name]; !exists {
		return configstore.ErrNotFound
	}
	delete(s.plugins, name)
	return nil
}

func clonePluginsClusterRecord(plugin *configstoreTables.TablePlugin) *configstoreTables.TablePlugin {
	if plugin == nil {
		return nil
	}
	cloned := *plugin
	if config, ok := plugin.Config.(map[string]any); ok {
		clonedConfig := make(map[string]any, len(config))
		for key, value := range config {
			clonedConfig[key] = value
		}
		cloned.Config = clonedConfig
	}
	return &cloned
}

func TestCreatePluginPropagatesBuiltinClusterChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newPluginsClusterTestStore()
	loader := &clusterTestPluginsLoader{}
	propagator := &clusterTestPluginPropagator{}
	handler := NewPluginsHandler(loader, store, propagator)

	body, err := json.Marshal(map[string]any{
		"name":    "logging",
		"enabled": true,
		"config": map[string]any{
			"capture_response_body": true,
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/plugins")
	ctx.Request.SetBody(body)

	handler.createPlugin(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(loader.reloaded) != 1 || loader.reloaded[0] != "logging" {
		t.Fatalf("expected logging plugin reload, got %+v", loader.reloaded)
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated change, got %+v", propagator.changes)
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopePlugin || change.PluginName != "logging" || change.Delete {
		t.Fatalf("unexpected propagated change: %+v", change)
	}
	if change.PluginConfig == nil || change.PluginConfig.Path != nil || change.PluginConfig.IsCustom || !change.PluginConfig.Enabled {
		t.Fatalf("unexpected propagated plugin payload: %+v", change.PluginConfig)
	}
}

func TestCreatePluginDoesNotPropagateCustomClusterChange(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newPluginsClusterTestStore()
	loader := &clusterTestPluginsLoader{}
	propagator := &clusterTestPluginPropagator{}
	handler := NewPluginsHandler(loader, store, propagator)

	customPath := "/tmp/custom-plugin.so"
	body, err := json.Marshal(map[string]any{
		"name":    "custom-plugin",
		"enabled": false,
		"path":    customPath,
		"config":  map[string]any{"enabled": true},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.SetRequestURI("/api/plugins")
	ctx.Request.SetBody(body)

	handler.createPlugin(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 0 {
		t.Fatalf("expected no propagated changes for custom plugin, got %+v", propagator.changes)
	}
}

func TestDeletePluginPropagatesBuiltinClusterDelete(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newPluginsClusterTestStore()
	if err := store.CreatePlugin(context.Background(), &configstoreTables.TablePlugin{
		Name:     "logging",
		Enabled:  false,
		IsCustom: false,
		Config:   map[string]any{},
	}); err != nil {
		t.Fatalf("CreatePlugin() error = %v", err)
	}

	loader := &clusterTestPluginsLoader{}
	propagator := &clusterTestPluginPropagator{}
	handler := NewPluginsHandler(loader, store, propagator)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodDelete)
	ctx.Request.SetRequestURI("/api/plugins/logging")
	ctx.SetUserValue("name", "logging")

	handler.deletePlugin(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if len(propagator.changes) != 1 {
		t.Fatalf("expected one propagated delete change, got %+v", propagator.changes)
	}
	change := propagator.changes[0]
	if change.Scope != ClusterConfigScopePlugin || change.PluginName != "logging" || !change.Delete {
		t.Fatalf("unexpected propagated delete change: %+v", change)
	}
}
