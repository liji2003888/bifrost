package server

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/plugins/governance"
	"github.com/maximhq/bifrost/plugins/logging"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

type clientReloadConfigStore struct {
	configstore.ConfigStore
	clientConfig *configstore.ClientConfig
}

func (m *clientReloadConfigStore) GetClientConfig(_ context.Context) (*configstore.ClientConfig, error) {
	if m.clientConfig == nil {
		return nil, nil
	}
	cloned := *m.clientConfig
	cloned.LoggingHeaders = append([]string(nil), m.clientConfig.LoggingHeaders...)
	cloned.RequiredHeaders = append([]string(nil), m.clientConfig.RequiredHeaders...)
	cloned.WhitelistedRoutes = append([]string(nil), m.clientConfig.WhitelistedRoutes...)
	return &cloned, nil
}

func newServerTestLogStore(t *testing.T) logstore.LogStore {
	t.Helper()

	store, err := logstore.NewLogStore(context.Background(), &logstore.Config{
		Enabled: true,
		Type:    logstore.LogStoreTypeSQLite,
		Config: &logstore.SQLiteConfig{
			Path: filepath.Join(t.TempDir(), "server-logging.db"),
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}

	t.Cleanup(func() {
		_ = store.Close(context.Background())
	})
	return store
}

func TestReloadClientConfigFromConfigStoreRebindsClientConfigDependentPlugins(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())

	staleClientConfig := &configstore.ClientConfig{
		DisableContentLogging:  true,
		LoggingHeaders:         []string{"x-stale-log"},
		EnforceAuthOnInference: true,
		RequiredHeaders:        []string{"x-stale-required"},
	}
	runtimeClientConfig := &configstore.ClientConfig{
		DisableContentLogging:  false,
		LoggingHeaders:         []string{"x-runtime-log"},
		EnforceAuthOnInference: false,
		RequiredHeaders:        []string{"x-runtime-required"},
	}
	storeClientConfig := &configstore.ClientConfig{
		DisableContentLogging:  false,
		LoggingHeaders:         []string{"x-store-log"},
		EnforceAuthOnInference: false,
		RequiredHeaders:        []string{"x-store-required"},
		WhitelistedRoutes:      []string{"/health", "/ready"},
	}

	logPlugin, err := logging.Init(context.Background(), &logging.Config{
		DisableContentLogging: &staleClientConfig.DisableContentLogging,
		LoggingHeaders:        &staleClientConfig.LoggingHeaders,
	}, bifrost.NewNoOpLogger(), newServerTestLogStore(t), nil, nil)
	if err != nil {
		t.Fatalf("logging.Init() error = %v", err)
	}

	governanceStore, err := governance.NewLocalGovernanceStore(context.Background(), bifrost.NewNoOpLogger(), nil, &configstore.GovernanceConfig{}, nil)
	if err != nil {
		t.Fatalf("NewLocalGovernanceStore() error = %v", err)
	}
	governancePlugin, err := governance.InitFromStore(context.Background(), &governance.Config{
		IsVkMandatory:   &staleClientConfig.EnforceAuthOnInference,
		RequiredHeaders: &staleClientConfig.RequiredHeaders,
	}, bifrost.NewNoOpLogger(), governanceStore, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("governance.InitFromStore() error = %v", err)
	}

	cfg := &lib.Config{
		ClientConfig: runtimeClientConfig,
		ConfigStore: &clientReloadConfigStore{
			clientConfig: storeClientConfig,
		},
	}
	if err := cfg.ReloadPlugin(logPlugin); err != nil {
		t.Fatalf("ReloadPlugin(logging) error = %v", err)
	}
	if err := cfg.ReloadPlugin(governancePlugin); err != nil {
		t.Fatalf("ReloadPlugin(governance) error = %v", err)
	}

	server := &BifrostHTTPServer{
		Config: cfg,
	}

	enableLogging, disableContentLogging, loggingHeaders := logPlugin.CurrentClientConfigBindings()
	if !enableLogging || !disableContentLogging || !reflect.DeepEqual(loggingHeaders, staleClientConfig.LoggingHeaders) {
		t.Fatalf("expected logging plugin to start bound to stale config, got enable=%v disable=%v headers=%v", enableLogging, disableContentLogging, loggingHeaders)
	}

	isVkMandatory, requiredHeaders := governancePlugin.CurrentClientConfigBindings()
	if !isVkMandatory || !reflect.DeepEqual(requiredHeaders, staleClientConfig.RequiredHeaders) {
		t.Fatalf("expected governance plugin to start bound to stale config, got mandatory=%v headers=%v", isVkMandatory, requiredHeaders)
	}

	for i := 0; i < 2; i++ {
		if err := server.ReloadClientConfigFromConfigStore(context.Background()); err != nil {
			t.Fatalf("ReloadClientConfigFromConfigStore() iteration %d error = %v", i+1, err)
		}
	}

	enableLogging, disableContentLogging, loggingHeaders = logPlugin.CurrentClientConfigBindings()
	if storeClientConfig.EnableLogging != nil && enableLogging != *storeClientConfig.EnableLogging {
		t.Fatalf("expected logging plugin enable_logging=%v, got %v", *storeClientConfig.EnableLogging, enableLogging)
	}
	if disableContentLogging != storeClientConfig.DisableContentLogging {
		t.Fatalf("expected logging plugin disable_content_logging=%v, got %v", storeClientConfig.DisableContentLogging, disableContentLogging)
	}
	if !reflect.DeepEqual(loggingHeaders, storeClientConfig.LoggingHeaders) {
		t.Fatalf("expected logging plugin headers %v, got %v", storeClientConfig.LoggingHeaders, loggingHeaders)
	}

	isVkMandatory, requiredHeaders = governancePlugin.CurrentClientConfigBindings()
	if isVkMandatory != storeClientConfig.EnforceAuthOnInference {
		t.Fatalf("expected governance plugin is_vk_mandatory=%v, got %v", storeClientConfig.EnforceAuthOnInference, isVkMandatory)
	}
	if !reflect.DeepEqual(requiredHeaders, storeClientConfig.RequiredHeaders) {
		t.Fatalf("expected governance plugin required headers %v, got %v", storeClientConfig.RequiredHeaders, requiredHeaders)
	}

	if server.Config.ClientConfig == nil {
		t.Fatal("expected runtime client config to remain initialized")
	}
	if server.Config.ClientConfig.DisableContentLogging != storeClientConfig.DisableContentLogging {
		t.Fatalf("expected runtime disable_content_logging=%v, got %v", storeClientConfig.DisableContentLogging, server.Config.ClientConfig.DisableContentLogging)
	}
	if !reflect.DeepEqual(server.Config.ClientConfig.LoggingHeaders, storeClientConfig.LoggingHeaders) {
		t.Fatalf("expected runtime logging headers %v, got %v", storeClientConfig.LoggingHeaders, server.Config.ClientConfig.LoggingHeaders)
	}
	if !reflect.DeepEqual(server.Config.ClientConfig.RequiredHeaders, storeClientConfig.RequiredHeaders) {
		t.Fatalf("expected runtime required headers %v, got %v", storeClientConfig.RequiredHeaders, server.Config.ClientConfig.RequiredHeaders)
	}
}
