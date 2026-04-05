package enterprise

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/kvstore"
)

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
