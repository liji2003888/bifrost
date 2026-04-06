package enterprise

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/kvstore"
)

type fakeClusterDiscoveryResolver struct {
	addresses []string
	err       error
}

func (f *fakeClusterDiscoveryResolver) Discover(_ context.Context, _ *ClusterConfig, _ string) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	result := make([]string, len(f.addresses))
	copy(result, f.addresses)
	return result, f.err
}

func TestClusterServiceAppliesRemoteMutations(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	service, err := NewClusterService(&ClusterConfig{Enabled: true}, store, "127.0.0.1:8080", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer service.Close()

	if err := service.ApplySet(ClusterMutation{
		Key:       "session:test",
		ValueJSON: []byte(`"hello"`),
		WrittenAt: time.Now().UnixNano(),
	}); err != nil {
		t.Fatalf("ApplySet() error = %v", err)
	}

	value, err := store.Get("session:test")
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	decoded, ok := value.([]byte)
	if !ok || string(decoded) != `"hello"` {
		t.Fatalf("unexpected stored value: %#v", value)
	}

	if err := service.ApplyDelete(ClusterMutation{
		Key:       "session:test",
		DeletedAt: time.Now().UnixNano(),
	}); err != nil {
		t.Fatalf("ApplyDelete() error = %v", err)
	}

	if _, err := store.Get("session:test"); err == nil {
		t.Fatal("expected key to be deleted")
	}
}

func TestClusterServiceBroadcastsAuthToken(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	tokenSeen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenSeen <- r.Header.Get(ClusterAuthHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service, err := NewClusterService(&ClusterConfig{
		Enabled:   true,
		Peers:     []string{server.URL},
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, store, "127.0.0.1:8080", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer service.Close()

	if err := store.SetWithTTL("session:test", "value", 0); err != nil {
		t.Fatalf("SetWithTTL() error = %v", err)
	}

	select {
	case token := <-tokenSeen:
		if token != "cluster-secret" {
			t.Fatalf("expected cluster auth token to be forwarded, got %q", token)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cluster broadcast")
	}
}

func TestClusterStatusReflectsFailedPeerThreshold(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	service, err := NewClusterService(&ClusterConfig{
		Enabled: true,
		Peers:   []string{"http://peer-a"},
		Gossip: &ClusterGossipConfig{
			Config: &ClusterHealthConfig{FailureThreshold: 2},
		},
	}, store, "127.0.0.1:8080", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer service.Close()

	if status := service.Status(); !status.Healthy {
		t.Fatal("expected cluster to be healthy before failures cross threshold")
	}

	service.markPeerFailure("http://peer-a", nil)
	if status := service.Status(); !status.Healthy {
		t.Fatal("expected cluster to remain healthy before failure threshold is reached")
	}

	service.markPeerFailure("http://peer-a", nil)
	if status := service.Status(); status.Healthy {
		t.Fatal("expected cluster to become unhealthy after peer failure threshold is reached")
	}
}

func TestClusterServiceRefreshesDiscoveredPeers(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	resolver := &fakeClusterDiscoveryResolver{
		addresses: []string{"10.1.2.3:8080"},
	}
	service, err := newClusterService(&ClusterConfig{
		Enabled: true,
		Discovery: &ClusterDiscoveryConfig{
			Enabled:  true,
			Type:     ClusterDiscoveryDNS,
			BindPort: 8080,
			DNSNames: []string{"cluster.internal"},
		},
	}, store, "127.0.0.1:8080", bifrost.NewNoOpLogger(), resolver)
	if err != nil {
		t.Fatalf("newClusterService() error = %v", err)
	}
	defer service.Close()

	status := service.Status()
	if len(status.Peers) != 1 {
		t.Fatalf("expected one discovered peer, got %+v", status.Peers)
	}
	if status.Peers[0].Address != "http://10.1.2.3:8080" {
		t.Fatalf("unexpected discovered peer address: %+v", status.Peers[0])
	}
	if status.Discovery == nil || status.Discovery.PeerCount != 1 {
		t.Fatalf("expected discovery status to include one peer, got %+v", status.Discovery)
	}
}

func TestClusterServiceFanoutAddressesExpandDNSBackedPeers(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	resolver := &dnsClusterDiscoveryResolver{
		lookupHost: func(_ context.Context, host string) ([]string, error) {
			if host != "bifrost-peer.ai-gateway.svc" {
				t.Fatalf("unexpected lookup host: %s", host)
			}
			return []string{"10.2.0.11", "10.2.0.12", "127.0.0.1", "10.2.0.11"}, nil
		},
	}

	service, err := newClusterService(&ClusterConfig{
		Enabled: true,
		Peers:   []string{"http://bifrost-peer.ai-gateway.svc:8080"},
	}, store, "127.0.0.1:8080", bifrost.NewNoOpLogger(), resolver)
	if err != nil {
		t.Fatalf("newClusterService() error = %v", err)
	}
	defer service.Close()

	targets := service.FanoutAddresses(context.Background())
	if len(targets) != 2 {
		t.Fatalf("expected two expanded fanout targets, got %+v", targets)
	}
	targetSet := map[string]bool{}
	for _, target := range targets {
		targetSet[target] = true
	}
	if !targetSet["http://10.2.0.11:8080"] || !targetSet["http://10.2.0.12:8080"] {
		t.Fatalf("unexpected expanded fanout targets: %+v", targets)
	}

	service.markPeerSuccess("http://10.2.0.11:8080", &ClusterStatus{NodeID: "bifrost-0:8080", Healthy: true})
	service.markPeerSuccess("http://10.2.0.12:8080", &ClusterStatus{NodeID: "bifrost-1:8080", Healthy: true})

	status := service.Status()
	if len(status.Peers) != 2 {
		t.Fatalf("expected status to expose expanded peer targets, got %+v", status.Peers)
	}
	if status.Peers[0].SeedAddress != "http://bifrost-peer.ai-gateway.svc:8080" || status.Peers[1].SeedAddress != "http://bifrost-peer.ai-gateway.svc:8080" {
		t.Fatalf("expected seed address to be preserved on expanded peers, got %+v", status.Peers)
	}
}

func TestClusterStatusIncludesLocalConfigSyncStatus(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	service, err := NewClusterService(&ClusterConfig{Enabled: true}, store, "127.0.0.1:8080", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer service.Close()

	inSync := true
	service.SetConfigSyncReporter(func() ClusterConfigSyncStatus {
		return ClusterConfigSyncStatus{
			StoreConnected: true,
			StoreKind:      "sqlite",
			RuntimeHash:    "runtime-hash",
			StoreHash:      "runtime-hash",
			InSync:         &inSync,
			DriftDomains:   []string{"providers"},
			ProviderCount:  2,
		}
	})

	status := service.Status()
	if status.ConfigSync == nil {
		t.Fatal("expected local config sync status to be present")
	}
	if !status.ConfigSync.StoreConnected || status.ConfigSync.StoreKind != "sqlite" {
		t.Fatalf("unexpected config sync status: %+v", status.ConfigSync)
	}
	if status.ConfigSync.InSync == nil || !*status.ConfigSync.InSync {
		t.Fatalf("expected in-sync flag to be true, got %+v", status.ConfigSync)
	}
	if len(status.ConfigSync.DriftDomains) != 1 || status.ConfigSync.DriftDomains[0] != "providers" {
		t.Fatalf("expected drift domains to be copied, got %+v", status.ConfigSync)
	}
}

func TestClusterServiceRemovesStaleDiscoveredPeersButKeepsStaticPeers(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	resolver := &fakeClusterDiscoveryResolver{
		addresses: []string{"10.1.2.3:8080"},
	}
	service, err := newClusterService(&ClusterConfig{
		Enabled: true,
		Peers:   []string{"10.9.9.9:8080"},
		Discovery: &ClusterDiscoveryConfig{
			Enabled:  true,
			Type:     ClusterDiscoveryDNS,
			BindPort: 8080,
			DNSNames: []string{"cluster.internal"},
		},
	}, store, "127.0.0.1:8080", bifrost.NewNoOpLogger(), resolver)
	if err != nil {
		t.Fatalf("newClusterService() error = %v", err)
	}
	defer service.Close()

	resolver.addresses = []string{"10.4.5.6:8080"}
	service.refreshDiscoveredPeers(context.Background())

	status := service.Status()
	if len(status.Peers) != 2 {
		t.Fatalf("expected one static and one discovered peer, got %+v", status.Peers)
	}

	addresses := map[string]bool{}
	for _, peer := range status.Peers {
		addresses[peer.Address] = true
	}
	if !addresses["http://10.9.9.9:8080"] {
		t.Fatalf("expected static peer to remain, got %+v", status.Peers)
	}
	if !addresses["http://10.4.5.6:8080"] {
		t.Fatalf("expected refreshed discovered peer to be present, got %+v", status.Peers)
	}
	if addresses["http://10.1.2.3:8080"] {
		t.Fatalf("expected stale discovered peer to be removed, got %+v", status.Peers)
	}
}

func TestClusterCheckPeersCapturesRemoteStatusMetadata(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	startedAt := time.Now().UTC().Add(-5 * time.Minute)
	remoteInSync := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(ClusterAuthHeader); got != "cluster-secret" {
			t.Fatalf("expected cluster auth header to be forwarded, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ClusterStatus{
			NodeID:    "node-b",
			StartedAt: startedAt,
			Healthy:   false,
			KVKeys:    42,
			ConfigSync: &ClusterConfigSyncStatus{
				StoreConnected: true,
				StoreKind:      "postgres",
				RuntimeHash:    "runtime-b",
				StoreHash:      "store-b",
				InSync:         &remoteInSync,
				DriftDomains:   []string{"providers", "mcp"},
			},
			Discovery: &ClusterDiscoveryStatus{
				Enabled:   true,
				Type:      ClusterDiscoveryDNS,
				PeerCount: 3,
			},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	service, err := NewClusterService(&ClusterConfig{
		Enabled:   true,
		Peers:     []string{server.URL},
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, store, "127.0.0.1:8080", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer service.Close()

	service.checkPeers()

	status := service.Status()
	if status.Healthy {
		t.Fatal("expected cluster to reflect unhealthy remote peer state")
	}
	if len(status.Peers) != 1 {
		t.Fatalf("expected one peer, got %+v", status.Peers)
	}

	peer := status.Peers[0]
	if peer.NodeID != "node-b" {
		t.Fatalf("expected peer node id to be populated, got %+v", peer)
	}
	if peer.ReportedHealthy == nil || *peer.ReportedHealthy {
		t.Fatalf("expected peer reported healthy to be false, got %+v", peer)
	}
	if peer.KVKeys != 42 {
		t.Fatalf("expected peer kv keys to be populated, got %+v", peer)
	}
	if peer.DiscoveryPeerCount != 3 {
		t.Fatalf("expected peer discovery count to be populated, got %+v", peer)
	}
	if peer.StartedAt == nil || !peer.StartedAt.Equal(startedAt) {
		t.Fatalf("expected peer started time to be populated, got %+v", peer)
	}
	if peer.ConfigSync == nil {
		t.Fatalf("expected peer config sync status to be populated, got %+v", peer)
	}
	if peer.ConfigSync.StoreKind != "postgres" || peer.ConfigSync.InSync == nil || *peer.ConfigSync.InSync {
		t.Fatalf("unexpected peer config sync state: %+v", peer.ConfigSync)
	}
	if len(peer.ConfigSync.DriftDomains) != 2 {
		t.Fatalf("expected peer drift domains to be populated, got %+v", peer.ConfigSync)
	}
}
