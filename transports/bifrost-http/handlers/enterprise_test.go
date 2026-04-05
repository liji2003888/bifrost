package handlers

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
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/loadbalancer"
)

type fakeLoadBalancerStatusProvider struct {
	routes     []loadbalancer.RouteStatus
	directions []loadbalancer.DirectionStatus
}

func (f *fakeLoadBalancerStatusProvider) ListSnapshots(provider schemas.ModelProvider, model string) []loadbalancer.RouteStatus {
	result := make([]loadbalancer.RouteStatus, 0, len(f.routes))
	for _, route := range f.routes {
		if provider != "" && route.Provider != provider {
			continue
		}
		if model != "" && route.Model != model {
			continue
		}
		result = append(result, route)
	}
	return result
}

func (f *fakeLoadBalancerStatusProvider) ListDirectionSnapshots(provider schemas.ModelProvider, model string) []loadbalancer.DirectionStatus {
	result := make([]loadbalancer.DirectionStatus, 0, len(f.directions))
	for _, direction := range f.directions {
		if provider != "" && direction.Provider != provider {
			continue
		}
		if model != "" && direction.Model != model {
			continue
		}
		result = append(result, direction)
	}
	return result
}

func TestCollectAdaptiveRoutingStatusAggregatesPeerResponses(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != clusterAdaptiveRoutingEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get(enterprisecfg.ClusterAuthHeader); got != "cluster-secret" {
			t.Fatalf("expected cluster auth header, got %q", got)
		}
		if got := r.URL.Query().Get("provider"); got != "openai" {
			t.Fatalf("expected provider filter to be forwarded, got %q", got)
		}
		if got := r.URL.Query().Get("model"); got != "gpt-4" {
			t.Fatalf("expected model filter to be forwarded, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(adaptiveRoutingStatusResponse{
			NodeID: "remote-node",
			Routes: []clusterRouteStatus{
				{
					RouteStatus: loadbalancer.RouteStatus{
						Provider: schemas.ModelProvider("openai"),
						Model:    "gpt-4",
						KeyID:    "remote-key",
					},
				},
			},
			Directions: []clusterDirectionStatus{
				{
					DirectionStatus: loadbalancer.DirectionStatus{
						Provider: schemas.ModelProvider("openai"),
						Model:    "gpt-4",
						Score:    0.91,
					},
				},
			},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
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

	handler := NewEnterpriseHandler(cluster, nil, nil, nil, nil, &fakeLoadBalancerStatusProvider{
		routes: []loadbalancer.RouteStatus{
			{
				Provider: schemas.ModelProvider("openai"),
				Model:    "gpt-4",
				KeyID:    "local-key",
			},
		},
		directions: []loadbalancer.DirectionStatus{
			{
				Provider: schemas.ModelProvider("openai"),
				Model:    "gpt-4",
				Score:    0.82,
			},
		},
	})

	response := handler.collectAdaptiveRoutingStatus(context.Background(), schemas.ModelProvider("openai"), "gpt-4", true)
	if !response.Cluster {
		t.Fatal("expected adaptive routing response to be cluster-aware")
	}
	if len(response.Warnings) != 0 {
		t.Fatalf("expected no aggregation warnings, got %+v", response.Warnings)
	}
	if len(response.Routes) != 2 {
		t.Fatalf("expected local and remote routes, got %+v", response.Routes)
	}
	if len(response.Directions) != 2 {
		t.Fatalf("expected local and remote directions, got %+v", response.Directions)
	}

	var foundRemote bool
	for _, route := range response.Routes {
		if route.KeyID != "remote-key" {
			continue
		}
		foundRemote = true
		if route.NodeID != "remote-node" {
			t.Fatalf("expected remote node id to be propagated, got %+v", route)
		}
		if route.Address != server.URL {
			t.Fatalf("expected remote address to be propagated, got %+v", route)
		}
		if route.Source != peerClusterSource {
			t.Fatalf("expected remote route source to be %q, got %+v", peerClusterSource, route)
		}
	}
	if !foundRemote {
		t.Fatalf("expected remote route to be present, got %+v", response.Routes)
	}
}

func TestCollectAlertsAggregatesPeerResponses(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	triggeredAt := time.Now().UTC()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != clusterAlertsEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get(enterprisecfg.ClusterAuthHeader); got != "cluster-secret" {
			t.Fatalf("expected cluster auth header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(alertsResponse{
			NodeID: "remote-node",
			Alerts: []clusterAlertRecord{
				{
					AlertRecord: enterprisecfg.AlertRecord{
						ID:          "alert-1",
						Key:         "health.error_rate",
						Type:        "health",
						Severity:    enterprisecfg.AlertSeverityCritical,
						Title:       "High error rate detected",
						Message:     "Error rate reached 12%.",
						TriggeredAt: triggeredAt,
					},
				},
			},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
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

	handler := NewEnterpriseHandler(cluster, nil, nil, nil, nil, nil)
	response := handler.collectAlerts(context.Background(), true)

	if !response.Cluster {
		t.Fatal("expected alerts response to be cluster-aware")
	}
	if len(response.Warnings) != 0 {
		t.Fatalf("expected no aggregation warnings, got %+v", response.Warnings)
	}
	if len(response.Alerts) != 1 {
		t.Fatalf("expected one remote alert, got %+v", response.Alerts)
	}
	if response.Alerts[0].NodeID != "remote-node" {
		t.Fatalf("expected remote alert node id to be propagated, got %+v", response.Alerts[0])
	}
	if response.Alerts[0].Address != server.URL {
		t.Fatalf("expected remote alert address to be propagated, got %+v", response.Alerts[0])
	}
	if response.Alerts[0].Source != peerClusterSource {
		t.Fatalf("expected remote alert source to be %q, got %+v", peerClusterSource, response.Alerts[0])
	}
}
