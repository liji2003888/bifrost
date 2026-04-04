package enterprise

import (
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
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
