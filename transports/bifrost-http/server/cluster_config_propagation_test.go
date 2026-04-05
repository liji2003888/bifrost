package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/kvstore"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
)

func TestPropagateClusterConfigChangeBroadcastsPayloadToPeers(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	received := make(chan handlers.ClusterConfigChange, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != handlers.ClusterConfigReloadEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get(enterprisecfg.ClusterAuthHeader); got != "cluster-secret" {
			t.Fatalf("expected cluster auth header, got %q", got)
		}

		var change handlers.ClusterConfigChange
		if err := json.NewDecoder(r.Body).Decode(&change); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		received <- change
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		Peers:     []string{server.URL},
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, store, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer cluster.Close()

	s := &BifrostHTTPServer{
		ClusterService: cluster,
	}
	change := &handlers.ClusterConfigChange{
		Scope: handlers.ClusterConfigScopeProxy,
		ProxyConfig: &configstoreTables.GlobalProxyConfig{
			Enabled: true,
			Type:    "http",
			URL:     "http://proxy.internal:8080",
		},
	}

	if err := s.PropagateClusterConfigChange(context.Background(), change); err != nil {
		t.Fatalf("PropagateClusterConfigChange() error = %v", err)
	}

	select {
	case got := <-received:
		if got.Scope != handlers.ClusterConfigScopeProxy {
			t.Fatalf("expected proxy scope, got %+v", got)
		}
		if got.ProxyConfig == nil || got.ProxyConfig.URL != "http://proxy.internal:8080" || !got.ProxyConfig.Enabled {
			t.Fatalf("expected proxy config payload to be preserved, got %+v", got.ProxyConfig)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for propagated cluster config change")
	}
}
