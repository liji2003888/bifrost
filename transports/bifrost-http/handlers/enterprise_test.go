package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fasthttp/router"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/kvstore"
	"github.com/maximhq/bifrost/framework/logstore"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/loadbalancer"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type fakeLoadBalancerStatusProvider struct {
	routes          []loadbalancer.RouteStatus
	localRoutes     []loadbalancer.RouteStatus
	directions      []loadbalancer.DirectionStatus
	localDirections []loadbalancer.DirectionStatus
	enabled         bool
}

type fakeClusterConfigApplier struct {
	lastChange *ClusterConfigChange
	err        error
}

func (f *fakeClusterConfigApplier) ApplyClusterConfigChange(_ context.Context, change *ClusterConfigChange) error {
	if f == nil {
		return nil
	}
	if change != nil {
		copyChange := *change
		f.lastChange = &copyChange
	}
	return f.err
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

func (f *fakeLoadBalancerStatusProvider) ListLocalSnapshots(provider schemas.ModelProvider, model string) []loadbalancer.RouteStatus {
	if len(f.localRoutes) == 0 {
		return f.ListSnapshots(provider, model)
	}
	result := make([]loadbalancer.RouteStatus, 0, len(f.localRoutes))
	for _, route := range f.localRoutes {
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

func (f *fakeLoadBalancerStatusProvider) ListLocalDirectionSnapshots(provider schemas.ModelProvider, model string) []loadbalancer.DirectionStatus {
	if len(f.localDirections) == 0 {
		return f.ListDirectionSnapshots(provider, model)
	}
	result := make([]loadbalancer.DirectionStatus, 0, len(f.localDirections))
	for _, direction := range f.localDirections {
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

func (f *fakeLoadBalancerStatusProvider) Enabled() bool {
	if f == nil {
		return false
	}
	if !f.enabled && len(f.routes) == 0 && len(f.directions) == 0 {
		return false
	}
	return true
}

func TestRegisterRoutesExposesDisabledEnterpriseStatusEndpoints(t *testing.T) {
	handler := NewEnterpriseHandler(nil, nil, nil, nil, nil, nil, &fakeClusterConfigApplier{}, nil, nil)
	testRouter := router.New()
	handler.RegisterRoutes(testRouter)

	listener := fasthttputil.NewInmemoryListener()
	server := &fasthttp.Server{Handler: testRouter.Handler}
	go server.Serve(listener) //nolint:errcheck
	defer listener.Close()

	client := &fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) {
			return listener.Dial()
		},
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	testCases := []struct {
		name        string
		method      string
		uri         string
		wantMessage string
	}{
		{
			name:        "cluster status",
			method:      fasthttp.MethodGet,
			uri:         "http://enterprise.local/api/cluster/status",
			wantMessage: "cluster service is not enabled",
		},
		{
			name:        "vault status",
			method:      fasthttp.MethodGet,
			uri:         "http://enterprise.local/api/vault/status",
			wantMessage: "vault service is not enabled",
		},
		{
			name:        "adaptive routing",
			method:      fasthttp.MethodGet,
			uri:         "http://enterprise.local/api/adaptive-routing/status",
			wantMessage: "adaptive routing is not enabled",
		},
		{
			name:        "audit logs",
			method:      fasthttp.MethodGet,
			uri:         "http://enterprise.local/api/audit-logs",
			wantMessage: "audit service is not enabled",
		},
		{
			name:        "log exports",
			method:      fasthttp.MethodGet,
			uri:         "http://enterprise.local/api/log-exports",
			wantMessage: "log export service is not enabled",
		},
		{
			name:        "alerts",
			method:      fasthttp.MethodGet,
			uri:         "http://enterprise.local/api/alerts",
			wantMessage: "alert service is not enabled",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := fasthttp.AcquireRequest()
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(resp)

			req.Header.SetMethod(tc.method)
			req.SetRequestURI(tc.uri)

			if err := client.Do(req, resp); err != nil {
				t.Fatalf("client.Do() error = %v", err)
			}
			if resp.StatusCode() != fasthttp.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d: %s", resp.StatusCode(), string(resp.Body()))
			}
			if !strings.Contains(strings.ToLower(string(resp.Body())), tc.wantMessage) {
				t.Fatalf("expected response body to contain %q, got %s", tc.wantMessage, string(resp.Body()))
			}
		})
	}
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

	handler := NewEnterpriseHandler(cluster, nil, nil, nil, nil, func() LoadBalancerStatusProvider {
		return &fakeLoadBalancerStatusProvider{
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
			enabled: true,
		}
	}, nil, nil, nil)

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

func TestGetInternalAdaptiveRoutingStatusReturnsLocalOnlySnapshots(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	cluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, store, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer cluster.Close()

	handler := NewEnterpriseHandler(cluster, nil, nil, nil, nil, func() LoadBalancerStatusProvider {
		return &fakeLoadBalancerStatusProvider{
			routes: []loadbalancer.RouteStatus{
				{Provider: schemas.OpenAI, Model: "gpt-4", KeyID: "merged-key", RouteSnapshot: loadbalancer.RouteSnapshot{Samples: 5}},
			},
			localRoutes: []loadbalancer.RouteStatus{
				{Provider: schemas.OpenAI, Model: "gpt-4", KeyID: "local-key", RouteSnapshot: loadbalancer.RouteSnapshot{Samples: 2}},
			},
			directions: []loadbalancer.DirectionStatus{
				{Provider: schemas.OpenAI, Model: "gpt-4", DirectionSnapshot: loadbalancer.DirectionSnapshot{Samples: 5}},
			},
			localDirections: []loadbalancer.DirectionStatus{
				{Provider: schemas.OpenAI, Model: "gpt-4", DirectionSnapshot: loadbalancer.DirectionSnapshot{Samples: 2}},
			},
			enabled: true,
		}
	}, nil, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.Header.Set(enterprisecfg.ClusterAuthHeader, "cluster-secret")
	ctx.Request.SetRequestURI(clusterAdaptiveRoutingEndpoint)

	handler.getInternalAdaptiveRoutingStatus(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}

	var response adaptiveRoutingStatusResponse
	if err := json.Unmarshal(ctx.Response.Body(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(response.Routes) != 1 || response.Routes[0].KeyID != "local-key" {
		t.Fatalf("expected only local route snapshot, got %+v", response.Routes)
	}
	if len(response.Directions) != 1 || response.Directions[0].Samples != 2 {
		t.Fatalf("expected only local direction snapshot, got %+v", response.Directions)
	}
}

func TestApplyClusterConfigReloadDelegatesToApplier(t *testing.T) {
	SetLogger(&mockLogger{})

	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	cluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, store, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer cluster.Close()

	applier := &fakeClusterConfigApplier{}
	handler := NewEnterpriseHandler(cluster, nil, nil, nil, nil, nil, applier, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.Header.Set(enterprisecfg.ClusterAuthHeader, "cluster-secret")
	ctx.Request.SetRequestURI(ClusterConfigReloadEndpoint)
	ctx.Request.SetBodyString(`{"scope":"provider","provider":"openai","provider_config":{"send_back_raw_response":true}}`)

	handler.applyClusterConfigReload(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if applier.lastChange == nil {
		t.Fatal("expected cluster config change to be delegated")
	}
	if applier.lastChange.Scope != ClusterConfigScopeProvider || applier.lastChange.Provider != schemas.OpenAI {
		t.Fatalf("unexpected delegated config change: %+v", applier.lastChange)
	}
	if applier.lastChange.ProviderConfig == nil || !applier.lastChange.ProviderConfig.SendBackRawResponse {
		t.Fatalf("expected provider config payload to be preserved, got %+v", applier.lastChange.ProviderConfig)
	}
}

func TestApplyClusterConfigReloadRejectsInvalidClusterToken(t *testing.T) {
	SetLogger(&mockLogger{})

	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	cluster, err := enterprisecfg.NewClusterService(&enterprisecfg.ClusterConfig{
		Enabled:   true,
		AuthToken: schemas.NewEnvVar("cluster-secret"),
	}, store, "local-node", bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewClusterService() error = %v", err)
	}
	defer cluster.Close()

	applier := &fakeClusterConfigApplier{}
	handler := NewEnterpriseHandler(cluster, nil, nil, nil, nil, nil, applier, nil, nil)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodPost)
	ctx.Request.Header.Set(enterprisecfg.ClusterAuthHeader, "wrong-token")
	ctx.Request.SetRequestURI(ClusterConfigReloadEndpoint)
	ctx.Request.SetBodyString(`{"scope":"client","client_config":{"enable_swagger":true}}`)

	handler.applyClusterConfigReload(ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if applier.lastChange != nil {
		t.Fatalf("expected config change not to be delegated on auth failure, got %+v", applier.lastChange)
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

	handler := NewEnterpriseHandler(cluster, nil, nil, nil, nil, nil, nil, nil, nil)
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

func TestCollectAuditLogsAggregatesPeerResponses(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	auditDir := t.TempDir()
	audit, err := enterprisecfg.NewAuditService(auditDir, &enterprisecfg.AuditLogsConfig{}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewAuditService() error = %v", err)
	}
	localTime := time.Now().UTC()
	if err := audit.Append(&enterprisecfg.AuditEvent{
		ID:        "local-audit",
		Timestamp: localTime,
		Category:  enterprisecfg.AuditCategorySystem,
		Action:    "local",
		Message:   "local audit event",
	}); err != nil {
		t.Fatalf("audit.Append() error = %v", err)
	}

	remoteTime := localTime.Add(30 * time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != clusterAuditLogsEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get(enterprisecfg.ClusterAuthHeader); got != "cluster-secret" {
			t.Fatalf("expected cluster auth header, got %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "10" {
			t.Fatalf("expected merged limit to be forwarded, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(auditLogsResponse{
			NodeID: "remote-node",
			Total:  1,
			Events: []clusterAuditEvent{
				{
					AuditEvent: enterprisecfg.AuditEvent{
						ID:        "remote-audit",
						Timestamp: remoteTime,
						Category:  enterprisecfg.AuditCategorySecurityEvent,
						Action:    "remote",
						Message:   "remote audit event",
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

	handler := NewEnterpriseHandler(cluster, audit, nil, nil, nil, nil, nil, nil, nil)
	response := handler.collectAuditLogs(context.Background(), enterprisecfg.AuditSearchFilters{Limit: 10}, true)

	if !response.Cluster {
		t.Fatal("expected audit response to be cluster-aware")
	}
	if response.Total != 2 {
		t.Fatalf("expected aggregate total of 2, got %+v", response)
	}
	if len(response.Events) != 2 {
		t.Fatalf("expected local and remote audit events, got %+v", response.Events)
	}
	if response.Events[0].ID != "remote-audit" {
		t.Fatalf("expected newest remote event first, got %+v", response.Events)
	}
	if response.Events[0].NodeID != "remote-node" || response.Events[0].Address != server.URL || response.Events[0].Source != peerClusterSource {
		t.Fatalf("expected remote metadata to be propagated, got %+v", response.Events[0])
	}
	if response.Events[1].Source != localClusterSource {
		t.Fatalf("expected local audit source to be propagated, got %+v", response.Events[1])
	}
}

func TestCollectExportJobsAggregatesPeerResponses(t *testing.T) {
	store, err := kvstore.New(kvstore.Config{CleanupInterval: time.Minute})
	if err != nil {
		t.Fatalf("kvstore.New() error = %v", err)
	}
	defer store.Close()

	remoteCreated := time.Now().UTC().Add(1 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != clusterLogExportsEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get(enterprisecfg.ClusterAuthHeader); got != "cluster-secret" {
			t.Fatalf("expected cluster auth header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(logExportsResponse{
			NodeID: "remote-node",
			Jobs: []clusterExportJob{
				{
					ExportJob: enterprisecfg.ExportJob{
						ID:        "remote-export",
						Status:    enterprisecfg.ExportJobCompleted,
						Scope:     enterprisecfg.ExportScopeLogs,
						Format:    "jsonl",
						CreatedAt: remoteCreated,
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

	baseDir := t.TempDir()
	exportDir := filepath.Join(baseDir, "exports")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	metadata := enterprisecfg.ExportJob{
		ID:        "local-export",
		Status:    enterprisecfg.ExportJobCompleted,
		Scope:     enterprisecfg.ExportScopeLogs,
		Format:    "csv",
		CreatedAt: remoteCreated.Add(-1 * time.Minute),
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(exportDir, metadata.ID+".job.json"), payload, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	exportService := mustNewMinimalExportServiceForTest(t, exportDir)
	handler := NewEnterpriseHandler(cluster, nil, exportService, nil, nil, nil, nil, nil, nil)

	response := handler.collectExportJobs(context.Background(), true)
	if !response.Cluster {
		t.Fatal("expected export jobs response to be cluster-aware")
	}
	if len(response.Jobs) != 2 {
		t.Fatalf("expected local and remote export jobs, got %+v", response.Jobs)
	}
	if response.Jobs[0].ID != "remote-export" {
		t.Fatalf("expected newest remote export first, got %+v", response.Jobs)
	}
	if response.Jobs[0].NodeID != "remote-node" || response.Jobs[0].Address != server.URL || response.Jobs[0].Source != peerClusterSource {
		t.Fatalf("expected remote export metadata to be propagated, got %+v", response.Jobs[0])
	}
}

func mustNewMinimalExportServiceForTest(t *testing.T, exportDir string) *enterprisecfg.LogExportService {
	t.Helper()

	baseDir := filepath.Dir(exportDir)
	service, err := enterprisecfg.NewLogExportService(baseDir, &enterprisecfg.LogExportsConfig{
		Enabled:     true,
		StoragePath: exportDir,
	}, fakeLogSearchProvider{}, nil, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewLogExportService() error = %v", err)
	}
	return service
}

type fakeLogSearchProvider struct{}

func (fakeLogSearchProvider) Search(_ context.Context, _ *logstore.SearchFilters, _ *logstore.PaginationOptions) (*logstore.SearchResult, error) {
	return nil, nil
}

func (fakeLogSearchProvider) SearchMCPToolLogs(_ context.Context, _ *logstore.MCPToolLogSearchFilters, _ *logstore.PaginationOptions) (*logstore.MCPToolLogSearchResult, error) {
	return nil, nil
}
