package server

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

func newClusterPluginApplyStore(t *testing.T) configstore.ConfigStore {
	t.Helper()

	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config: &configstore.SQLiteConfig{
			Path: filepath.Join(t.TempDir(), "cluster-plugin.db"),
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewConfigStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close(context.Background())
	})
	return store
}

func TestApplyClusterPluginConfigBuiltinsOnly(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	store := newClusterPluginApplyStore(t)
	server := &BifrostHTTPServer{
		Config: &lib.Config{ConfigStore: store},
	}

	if err := server.ApplyClusterPluginConfig(context.Background(), "logging", &configstoreTables.TablePlugin{
		Name:     "logging",
		Enabled:  false,
		IsCustom: false,
		Config:   map[string]any{"capture_response_body": true},
	}, false); err != nil {
		t.Fatalf("ApplyClusterPluginConfig(logging disabled) error = %v", err)
	}

	storedPlugin, err := store.GetPlugin(context.Background(), "logging")
	if err != nil {
		t.Fatalf("GetPlugin(logging) error = %v", err)
	}
	if storedPlugin.Enabled || storedPlugin.Path != nil || storedPlugin.IsCustom {
		t.Fatalf("unexpected stored builtin plugin: %+v", storedPlugin)
	}

	if err := server.ApplyClusterPluginConfig(context.Background(), "logging", nil, true); err != nil {
		t.Fatalf("ApplyClusterPluginConfig(logging delete) error = %v", err)
	}
	if _, err := store.GetPlugin(context.Background(), "logging"); !errors.Is(err, configstore.ErrNotFound) {
		t.Fatalf("expected logging plugin to be deleted, got err=%v", err)
	}

	customPath := "/tmp/custom-plugin.so"
	err = server.ApplyClusterPluginConfig(context.Background(), "custom-plugin", &configstoreTables.TablePlugin{
		Name:     "custom-plugin",
		Enabled:  false,
		IsCustom: true,
		Path:     &customPath,
	}, false)
	if err == nil {
		t.Fatal("expected custom plugin cluster sync to be rejected")
	}
}
