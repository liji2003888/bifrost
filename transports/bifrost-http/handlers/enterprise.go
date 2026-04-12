package handlers

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/maximhq/bifrost/transports/bifrost-http/loadbalancer"
	"github.com/valyala/fasthttp"
)

type LoadBalancerStatusProvider interface {
	Enabled() bool
	ListSnapshots(provider schemas.ModelProvider, model string) []loadbalancer.RouteStatus
	ListDirectionSnapshots(provider schemas.ModelProvider, model string) []loadbalancer.DirectionStatus
}

const (
	clusterAlertsEndpoint          = "/_cluster/alerts"
	clusterAdaptiveRoutingEndpoint = "/_cluster/adaptive-routing/status"
	clusterAuditLogsEndpoint       = "/_cluster/audit-logs"
	clusterLogExportsEndpoint      = "/_cluster/log-exports"
)

type clusterAggregationWarning struct {
	Address string `json:"address"`
	Error   string `json:"error"`
}

type clusterAlertRecord struct {
	enterprisecfg.AlertRecord
	NodeID  string `json:"node_id,omitempty"`
	Address string `json:"address,omitempty"`
	Source  string `json:"source,omitempty"`
}

type clusterAuditEvent struct {
	enterprisecfg.AuditEvent
	NodeID  string `json:"node_id,omitempty"`
	Address string `json:"address,omitempty"`
	Source  string `json:"source,omitempty"`
}

type clusterExportJob struct {
	enterprisecfg.ExportJob
	NodeID  string `json:"node_id,omitempty"`
	Address string `json:"address,omitempty"`
	Source  string `json:"source,omitempty"`
}

type clusterRouteStatus struct {
	loadbalancer.RouteStatus
	NodeID  string `json:"node_id,omitempty"`
	Address string `json:"address,omitempty"`
	Source  string `json:"source,omitempty"`
}

type clusterDirectionStatus struct {
	loadbalancer.DirectionStatus
	NodeID  string `json:"node_id,omitempty"`
	Address string `json:"address,omitempty"`
	Source  string `json:"source,omitempty"`
}

type adaptiveRoutingStatusResponse struct {
	Cluster    bool                        `json:"cluster"`
	NodeID     string                      `json:"node_id,omitempty"`
	Routes     []clusterRouteStatus        `json:"routes"`
	Directions []clusterDirectionStatus    `json:"directions"`
	Warnings   []clusterAggregationWarning `json:"warnings,omitempty"`
}

type alertsResponse struct {
	Cluster  bool                        `json:"cluster"`
	NodeID   string                      `json:"node_id,omitempty"`
	Alerts   []clusterAlertRecord        `json:"alerts"`
	Warnings []clusterAggregationWarning `json:"warnings,omitempty"`
}

type auditLogsResponse struct {
	Cluster  bool                        `json:"cluster"`
	NodeID   string                      `json:"node_id,omitempty"`
	Events   []clusterAuditEvent         `json:"events"`
	Total    int                         `json:"total"`
	Warnings []clusterAggregationWarning `json:"warnings,omitempty"`
}

type logExportsResponse struct {
	Cluster  bool                        `json:"cluster"`
	NodeID   string                      `json:"node_id,omitempty"`
	Jobs     []clusterExportJob          `json:"jobs"`
	Warnings []clusterAggregationWarning `json:"warnings,omitempty"`
}

type EnterpriseHandler struct {
	cluster *enterprisecfg.ClusterService
	audit   *enterprisecfg.AuditService
	exports *enterprisecfg.LogExportService
	alerts  *enterprisecfg.AlertManager
	vault   *enterprisecfg.VaultService
	lb      func() LoadBalancerStatusProvider
	config  ClusterConfigApplier
}

func NewEnterpriseHandler(cluster *enterprisecfg.ClusterService, audit *enterprisecfg.AuditService, exports *enterprisecfg.LogExportService, alerts *enterprisecfg.AlertManager, vault *enterprisecfg.VaultService, lb func() LoadBalancerStatusProvider, config ClusterConfigApplier) *EnterpriseHandler {
	if cluster == nil && audit == nil && exports == nil && alerts == nil && vault == nil && lb == nil && config == nil {
		return nil
	}
	return &EnterpriseHandler{
		cluster: cluster,
		audit:   audit,
		exports: exports,
		alerts:  alerts,
		vault:   vault,
		lb:      lb,
		config:  config,
	}
}

func (h *EnterpriseHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	if h == nil {
		return
	}

	r.GET("/api/cluster/status", lib.ChainMiddlewares(h.getClusterStatus, middlewares...))
	r.GET("/api/audit-logs", lib.ChainMiddlewares(h.getAuditLogs, middlewares...))
	r.POST("/api/logs/exports", lib.ChainMiddlewares(h.createLogsExport, middlewares...))
	r.POST("/api/mcp-logs/exports", lib.ChainMiddlewares(h.createMCPLogsExport, middlewares...))
	r.GET("/api/log-exports", lib.ChainMiddlewares(h.listExportJobs, middlewares...))
	r.GET("/api/log-exports/{id}", lib.ChainMiddlewares(h.getExportJob, middlewares...))
	r.GET("/api/log-exports/{id}/download", lib.ChainMiddlewares(h.downloadExportJob, middlewares...))
	r.GET("/api/alerts", lib.ChainMiddlewares(h.getAlerts, middlewares...))
	r.GET("/api/vault/status", lib.ChainMiddlewares(h.getVaultStatus, middlewares...))
	r.GET("/api/adaptive-routing/status", lib.ChainMiddlewares(h.getAdaptiveRoutingStatus, middlewares...))

	if h.cluster != nil {
		r.GET("/_cluster/status", h.getInternalClusterStatus)
		r.POST("/_cluster/kv/set", h.applyClusterSet)
		r.POST("/_cluster/kv/delete", h.applyClusterDelete)
		if h.config != nil {
			r.POST(ClusterConfigReloadEndpoint, h.applyClusterConfigReload)
		}
		if h.audit != nil {
			r.GET(clusterAuditLogsEndpoint, h.getInternalAuditLogs)
		}
		if h.exports != nil {
			r.GET(clusterLogExportsEndpoint, h.getInternalExportJobs)
		}
		if h.alerts != nil {
			r.GET(clusterAlertsEndpoint, h.getInternalAlerts)
		}
		r.GET(clusterAdaptiveRoutingEndpoint, h.getInternalAdaptiveRoutingStatus)
	}
}

func (h *EnterpriseHandler) getClusterStatus(ctx *fasthttp.RequestCtx) {
	if h.cluster == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "cluster service is not enabled")
		return
	}
	SendJSON(ctx, h.cluster.Status())
}

func (h *EnterpriseHandler) getInternalClusterStatus(ctx *fasthttp.RequestCtx) {
	if h.cluster == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "cluster service is not enabled")
		return
	}
	if !h.requireClusterAuth(ctx) {
		return
	}
	SendJSON(ctx, h.cluster.Status())
}

func (h *EnterpriseHandler) applyClusterSet(ctx *fasthttp.RequestCtx) {
	if h.cluster == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "cluster service is not enabled")
		return
	}
	if !h.requireClusterAuth(ctx) {
		return
	}

	var mutation enterprisecfg.ClusterMutation
	if err := sonic.Unmarshal(ctx.PostBody(), &mutation); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid cluster set payload: %v", err))
		return
	}
	if err := h.cluster.ApplySet(mutation); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to apply cluster set: %v", err))
		return
	}
	SendJSON(ctx, map[string]any{"ok": true})
}

func (h *EnterpriseHandler) applyClusterDelete(ctx *fasthttp.RequestCtx) {
	if h.cluster == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "cluster service is not enabled")
		return
	}
	if !h.requireClusterAuth(ctx) {
		return
	}

	var mutation enterprisecfg.ClusterMutation
	if err := sonic.Unmarshal(ctx.PostBody(), &mutation); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid cluster delete payload: %v", err))
		return
	}
	if err := h.cluster.ApplyDelete(mutation); err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to apply cluster delete: %v", err))
		return
	}
	SendJSON(ctx, map[string]any{"ok": true})
}

func (h *EnterpriseHandler) applyClusterConfigReload(ctx *fasthttp.RequestCtx) {
	if h.cluster == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "cluster service is not enabled")
		return
	}
	if h.config == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "cluster config reload is not enabled")
		return
	}
	if !h.requireClusterAuth(ctx) {
		return
	}

	var change ClusterConfigChange
	if err := sonic.Unmarshal(ctx.PostBody(), &change); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid cluster config reload payload: %v", err))
		return
	}
	summary := clusterConfigChangeSummary(&change)
	if err := h.config.ApplyClusterConfigChange(clusterRequestContext(), &change); err != nil {
		if logger != nil {
			logger.Warn("failed to apply cluster config reload from %s: %s: %v", ctx.RemoteAddr().String(), summary, err)
		}
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to apply cluster config reload: %v", err))
		return
	}
	if logger != nil {
		logger.Info("applied cluster config reload from %s: %s", ctx.RemoteAddr().String(), summary)
	}
	SendJSON(ctx, map[string]any{"ok": true})
}

func (h *EnterpriseHandler) getAuditLogs(ctx *fasthttp.RequestCtx) {
	if h.audit == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "audit service is not enabled")
		return
	}
	filters, err := parseAuditFilters(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	SendJSON(ctx, h.collectAuditLogs(clusterRequestContext(), filters, wantsClusterAggregation(ctx)))
}

func (h *EnterpriseHandler) getInternalAuditLogs(ctx *fasthttp.RequestCtx) {
	if h.audit == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "audit service is not enabled")
		return
	}
	if !h.requireClusterAuth(ctx) {
		return
	}
	filters, err := parseAuditFilters(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	SendJSON(ctx, h.collectAuditLogs(clusterRequestContext(), filters, false))
}

func (h *EnterpriseHandler) createLogsExport(ctx *fasthttp.RequestCtx) {
	h.createExport(ctx, enterprisecfg.ExportScopeLogs)
}

func (h *EnterpriseHandler) createMCPLogsExport(ctx *fasthttp.RequestCtx) {
	h.createExport(ctx, enterprisecfg.ExportScopeMCPLogs)
}

func (h *EnterpriseHandler) createExport(ctx *fasthttp.RequestCtx, scope enterprisecfg.ExportScope) {
	if h.exports == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "log export service is not enabled")
		return
	}

	var request enterprisecfg.LogExportRequest
	if len(ctx.PostBody()) > 0 {
		if err := sonic.Unmarshal(ctx.PostBody(), &request); err != nil {
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid export request: %v", err))
			return
		}
	}
	request.Scope = scope

	actorID, _ := ctx.UserValue(schemas.BifrostContextKeyUserID).(string)
	job, err := h.exports.Submit(context.Background(), request, actorID)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	SendJSON(ctx, job)
}

func (h *EnterpriseHandler) listExportJobs(ctx *fasthttp.RequestCtx) {
	if h.exports == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "log export service is not enabled")
		return
	}
	SendJSON(ctx, h.collectExportJobs(clusterRequestContext(), wantsClusterAggregation(ctx)))
}

func (h *EnterpriseHandler) getInternalExportJobs(ctx *fasthttp.RequestCtx) {
	if h.exports == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "log export service is not enabled")
		return
	}
	if !h.requireClusterAuth(ctx) {
		return
	}
	SendJSON(ctx, h.collectExportJobs(clusterRequestContext(), false))
}

func (h *EnterpriseHandler) getExportJob(ctx *fasthttp.RequestCtx) {
	if h.exports == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "log export service is not enabled")
		return
	}
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "missing export id")
		return
	}
	job, found := h.exports.GetJob(id)
	if !found {
		SendError(ctx, fasthttp.StatusNotFound, "export job not found")
		return
	}
	SendJSON(ctx, job)
}

func (h *EnterpriseHandler) downloadExportJob(ctx *fasthttp.RequestCtx) {
	if h.exports == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "log export service is not enabled")
		return
	}
	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "missing export id")
		return
	}

	job, file, err := h.exports.OpenJobFile(id)
	if err != nil {
		errMessage := err.Error()
		switch {
		case strings.Contains(errMessage, "not found"):
			SendError(ctx, fasthttp.StatusNotFound, errMessage)
		case strings.Contains(errMessage, "not completed"):
			SendError(ctx, fasthttp.StatusConflict, errMessage)
		default:
			SendError(ctx, fasthttp.StatusBadRequest, errMessage)
		}
		return
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to stat export file: %v", err))
		return
	}

	// Read the entire file into memory so we can close the file handle before returning.
	// SetBodyStream with an *os.File is unreliable because fasthttp reads from the stream
	// asynchronously after the handler returns, at which point defer would have closed the file.
	fileSize := info.Size()
	body := make([]byte, fileSize)
	_, err = io.ReadFull(file, body)
	file.Close()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to read export file: %v", err))
		return
	}

	ctx.Response.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sanitizeAttachmentName(h.exports.DownloadFileName(job))))
	ctx.SetContentType(h.exports.DownloadContentType(job))
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.SetBody(body)
}

func (h *EnterpriseHandler) getAlerts(ctx *fasthttp.RequestCtx) {
	if h.alerts == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "alert service is not enabled")
		return
	}
	SendJSON(ctx, h.collectAlerts(clusterRequestContext(), wantsClusterAggregation(ctx)))
}

func (h *EnterpriseHandler) getInternalAlerts(ctx *fasthttp.RequestCtx) {
	if h.alerts == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "alert service is not enabled")
		return
	}
	if !h.requireClusterAuth(ctx) {
		return
	}
	SendJSON(ctx, h.collectAlerts(clusterRequestContext(), false))
}

func (h *EnterpriseHandler) getVaultStatus(ctx *fasthttp.RequestCtx) {
	if h.vault == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "vault service is not enabled")
		return
	}
	SendJSON(ctx, h.vault.Status())
}

func (h *EnterpriseHandler) getAdaptiveRoutingStatus(ctx *fasthttp.RequestCtx) {
	lb := h.currentLoadBalancer()
	if lb == nil || !lb.Enabled() {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "adaptive routing is not enabled")
		return
	}

	provider := schemas.ModelProvider(strings.TrimSpace(string(ctx.QueryArgs().Peek("provider"))))
	model := strings.TrimSpace(string(ctx.QueryArgs().Peek("model")))
	SendJSON(ctx, h.collectAdaptiveRoutingStatus(clusterRequestContext(), provider, model, wantsClusterAggregation(ctx)))
}

func (h *EnterpriseHandler) getInternalAdaptiveRoutingStatus(ctx *fasthttp.RequestCtx) {
	lb := h.currentLoadBalancer()
	if lb == nil || !lb.Enabled() {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "adaptive routing is not enabled")
		return
	}
	if !h.requireClusterAuth(ctx) {
		return
	}

	provider := schemas.ModelProvider(strings.TrimSpace(string(ctx.QueryArgs().Peek("provider"))))
	model := strings.TrimSpace(string(ctx.QueryArgs().Peek("model")))
	SendJSON(ctx, h.collectAdaptiveRoutingStatus(clusterRequestContext(), provider, model, false))
}

func (h *EnterpriseHandler) requireClusterAuth(ctx *fasthttp.RequestCtx) bool {
	if h.cluster == nil {
		return false
	}
	token := string(ctx.Request.Header.Peek(enterprisecfg.ClusterAuthHeader))
	if h.cluster.IsInternalTokenValid(token) {
		return true
	}
	SendError(ctx, fasthttp.StatusUnauthorized, "unauthorized cluster request")
	return false
}

func (h *EnterpriseHandler) collectAlerts(ctx context.Context, includeCluster bool) alertsResponse {
	response := alertsResponse{
		Cluster: includeCluster && h.cluster != nil,
		NodeID:  h.clusterNodeID(),
		Alerts:  make([]clusterAlertRecord, 0),
	}

	if h.alerts != nil {
		for _, alert := range h.alerts.ListActive() {
			response.Alerts = append(response.Alerts, clusterAlertRecord{
				AlertRecord: alert,
				NodeID:      response.NodeID,
				Source:      localClusterSource,
			})
		}
	}

	if !includeCluster || h.cluster == nil {
		sortClusterAlerts(response.Alerts)
		return response
	}

	for _, peer := range h.cluster.PeerStatuses() {
		if peer.Address == "" {
			continue
		}

		var remote alertsResponse
		if err := h.cluster.GetJSON(ctx, peer.Address, clusterAlertsEndpoint, &remote); err != nil {
			response.Warnings = append(response.Warnings, clusterAggregationWarning{
				Address: peer.Address,
				Error:   err.Error(),
			})
			continue
		}

		remoteNodeID := firstNonEmptyString(remote.NodeID, peer.NodeID)
		for _, alert := range remote.Alerts {
			alert.NodeID = firstNonEmptyString(alert.NodeID, remoteNodeID)
			alert.Address = firstNonEmptyString(alert.Address, peer.Address)
			if alert.Source == "" {
				alert.Source = peerClusterSource
			}
			response.Alerts = append(response.Alerts, alert)
		}
	}

	sortClusterAlerts(response.Alerts)
	sortClusterWarnings(response.Warnings)
	return response
}

func (h *EnterpriseHandler) collectAuditLogs(ctx context.Context, filters enterprisecfg.AuditSearchFilters, includeCluster bool) auditLogsResponse {
	response := auditLogsResponse{
		Cluster: includeCluster && h.cluster != nil,
		NodeID:  h.clusterNodeID(),
		Events:  make([]clusterAuditEvent, 0),
	}

	if h.audit != nil {
		localResult, err := h.audit.Search(filters)
		if err == nil && localResult != nil {
			response.Total += localResult.Total
			for _, event := range localResult.Events {
				response.Events = append(response.Events, clusterAuditEvent{
					AuditEvent: event,
					NodeID:     response.NodeID,
					Source:     localClusterSource,
				})
			}
		} else if err != nil {
			response.Warnings = append(response.Warnings, clusterAggregationWarning{
				Address: localClusterSource,
				Error:   err.Error(),
			})
		}
	}

	if !includeCluster || h.cluster == nil {
		sortClusterAuditEvents(response.Events)
		return applyAuditPagination(response, filters)
	}

	remoteFilters := filters
	remoteFilters.Offset = 0
	if remoteFilters.Limit <= 0 {
		remoteFilters.Limit = 100
	}
	remoteFilters.Limit += max(filters.Offset, 0)

	path, err := clusterAuditLogsPath(remoteFilters)
	if err != nil {
		response.Warnings = append(response.Warnings, clusterAggregationWarning{
			Address: localClusterSource,
			Error:   err.Error(),
		})
		sortClusterAuditEvents(response.Events)
		sortClusterWarnings(response.Warnings)
		return applyAuditPagination(response, filters)
	}

	for _, peer := range h.cluster.PeerStatuses() {
		if peer.Address == "" {
			continue
		}

		var remote auditLogsResponse
		if err := h.cluster.GetJSON(ctx, peer.Address, path, &remote); err != nil {
			response.Warnings = append(response.Warnings, clusterAggregationWarning{
				Address: peer.Address,
				Error:   err.Error(),
			})
			continue
		}

		response.Total += remote.Total
		remoteNodeID := firstNonEmptyString(remote.NodeID, peer.NodeID)
		for _, event := range remote.Events {
			event.NodeID = firstNonEmptyString(event.NodeID, remoteNodeID)
			event.Address = firstNonEmptyString(event.Address, peer.Address)
			if event.Source == "" {
				event.Source = peerClusterSource
			}
			response.Events = append(response.Events, event)
		}
	}

	sortClusterAuditEvents(response.Events)
	sortClusterWarnings(response.Warnings)
	return applyAuditPagination(response, filters)
}

func (h *EnterpriseHandler) collectExportJobs(ctx context.Context, includeCluster bool) logExportsResponse {
	response := logExportsResponse{
		Cluster: includeCluster && h.cluster != nil,
		NodeID:  h.clusterNodeID(),
		Jobs:    make([]clusterExportJob, 0),
	}

	if h.exports != nil {
		for _, job := range h.exports.ListJobs() {
			response.Jobs = append(response.Jobs, clusterExportJob{
				ExportJob: job,
				NodeID:    response.NodeID,
				Source:    localClusterSource,
			})
		}
	}

	if !includeCluster || h.cluster == nil {
		sortClusterExportJobs(response.Jobs)
		return response
	}

	for _, peer := range h.cluster.PeerStatuses() {
		if peer.Address == "" {
			continue
		}

		var remote logExportsResponse
		if err := h.cluster.GetJSON(ctx, peer.Address, clusterLogExportsEndpoint, &remote); err != nil {
			response.Warnings = append(response.Warnings, clusterAggregationWarning{
				Address: peer.Address,
				Error:   err.Error(),
			})
			continue
		}

		remoteNodeID := firstNonEmptyString(remote.NodeID, peer.NodeID)
		for _, job := range remote.Jobs {
			job.NodeID = firstNonEmptyString(job.NodeID, remoteNodeID)
			job.Address = firstNonEmptyString(job.Address, peer.Address)
			if job.Source == "" {
				job.Source = peerClusterSource
			}
			response.Jobs = append(response.Jobs, job)
		}
	}

	sortClusterExportJobs(response.Jobs)
	sortClusterWarnings(response.Warnings)
	return response
}

func (h *EnterpriseHandler) collectAdaptiveRoutingStatus(ctx context.Context, provider schemas.ModelProvider, model string, includeCluster bool) adaptiveRoutingStatusResponse {
	lb := h.currentLoadBalancer()
	response := adaptiveRoutingStatusResponse{
		Cluster:    includeCluster && h.cluster != nil,
		NodeID:     h.clusterNodeID(),
		Routes:     make([]clusterRouteStatus, 0),
		Directions: make([]clusterDirectionStatus, 0),
	}

	localNodeID := h.clusterNodeID()
	if lb != nil {
		for _, route := range lb.ListSnapshots(provider, model) {
			response.Routes = append(response.Routes, clusterRouteStatus{
				RouteStatus: route,
				NodeID:      localNodeID,
				Source:      localClusterSource,
			})
		}
		for _, direction := range lb.ListDirectionSnapshots(provider, model) {
			response.Directions = append(response.Directions, clusterDirectionStatus{
				DirectionStatus: direction,
				NodeID:          localNodeID,
				Source:          localClusterSource,
			})
		}
	}

	if !includeCluster || h.cluster == nil {
		sortClusterRoutes(response.Routes)
		sortClusterDirections(response.Directions)
		return response
	}

	path := clusterAdaptiveRoutingPath(provider, model)
	for _, peer := range h.cluster.PeerStatuses() {
		if peer.Address == "" {
			continue
		}

		var remote struct {
			NodeID     string                   `json:"node_id,omitempty"`
			Cluster    bool                     `json:"cluster"`
			Routes     []clusterRouteStatus     `json:"routes"`
			Directions []clusterDirectionStatus `json:"directions"`
		}
		if err := h.cluster.GetJSON(ctx, peer.Address, path, &remote); err != nil {
			response.Warnings = append(response.Warnings, clusterAggregationWarning{
				Address: peer.Address,
				Error:   err.Error(),
			})
			continue
		}

		remoteNodeID := firstNonEmptyString(remote.NodeID, peer.NodeID)
		for _, route := range remote.Routes {
			route.NodeID = firstNonEmptyString(route.NodeID, remoteNodeID)
			route.Address = firstNonEmptyString(route.Address, peer.Address)
			if route.Source == "" {
				route.Source = peerClusterSource
			}
			response.Routes = append(response.Routes, route)
		}
		for _, direction := range remote.Directions {
			direction.NodeID = firstNonEmptyString(direction.NodeID, remoteNodeID)
			direction.Address = firstNonEmptyString(direction.Address, peer.Address)
			if direction.Source == "" {
				direction.Source = peerClusterSource
			}
			response.Directions = append(response.Directions, direction)
		}
	}

	sortClusterRoutes(response.Routes)
	sortClusterDirections(response.Directions)
	sortClusterWarnings(response.Warnings)
	return response
}

func (h *EnterpriseHandler) currentLoadBalancer() LoadBalancerStatusProvider {
	if h == nil || h.lb == nil {
		return nil
	}
	return h.lb()
}

const (
	localClusterSource = "local"
	peerClusterSource  = "peer"
)

func clusterRequestContext() context.Context {
	return context.Background()
}

func wantsClusterAggregation(ctx *fasthttp.RequestCtx) bool {
	if ctx == nil {
		return false
	}
	value := strings.TrimSpace(strings.ToLower(string(ctx.QueryArgs().Peek("cluster"))))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseAuditFilters(ctx *fasthttp.RequestCtx) (enterprisecfg.AuditSearchFilters, error) {
	filters := enterprisecfg.AuditSearchFilters{
		Category:     enterprisecfg.AuditCategory(string(ctx.QueryArgs().Peek("category"))),
		Action:       string(ctx.QueryArgs().Peek("action")),
		ResourceType: string(ctx.QueryArgs().Peek("resource_type")),
		ActorID:      string(ctx.QueryArgs().Peek("actor_id")),
		Limit:        100,
	}

	if limitStr := string(ctx.QueryArgs().Peek("limit")); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return enterprisecfg.AuditSearchFilters{}, fmt.Errorf("invalid limit: %w", err)
		}
		filters.Limit = limit
	}
	if offsetStr := string(ctx.QueryArgs().Peek("offset")); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			return enterprisecfg.AuditSearchFilters{}, fmt.Errorf("invalid offset: %w", err)
		}
		filters.Offset = offset
	}
	if startStr := string(ctx.QueryArgs().Peek("start_time")); startStr != "" {
		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			return enterprisecfg.AuditSearchFilters{}, fmt.Errorf("invalid start_time: %w", err)
		}
		filters.StartTime = &start
	}
	if endStr := string(ctx.QueryArgs().Peek("end_time")); endStr != "" {
		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return enterprisecfg.AuditSearchFilters{}, fmt.Errorf("invalid end_time: %w", err)
		}
		filters.EndTime = &end
	}
	return filters, nil
}

func clusterAdaptiveRoutingPath(provider schemas.ModelProvider, model string) string {
	values := url.Values{}
	if provider != "" {
		values.Set("provider", string(provider))
	}
	if strings.TrimSpace(model) != "" {
		values.Set("model", model)
	}
	encoded := values.Encode()
	if encoded == "" {
		return clusterAdaptiveRoutingEndpoint
	}
	return clusterAdaptiveRoutingEndpoint + "?" + encoded
}

func clusterAuditLogsPath(filters enterprisecfg.AuditSearchFilters) (string, error) {
	values := url.Values{}
	if filters.Category != "" {
		values.Set("category", string(filters.Category))
	}
	if strings.TrimSpace(filters.Action) != "" {
		values.Set("action", filters.Action)
	}
	if strings.TrimSpace(filters.ResourceType) != "" {
		values.Set("resource_type", filters.ResourceType)
	}
	if strings.TrimSpace(filters.ActorID) != "" {
		values.Set("actor_id", filters.ActorID)
	}
	if filters.StartTime != nil {
		values.Set("start_time", filters.StartTime.UTC().Format(time.RFC3339))
	}
	if filters.EndTime != nil {
		values.Set("end_time", filters.EndTime.UTC().Format(time.RFC3339))
	}
	values.Set("limit", strconv.Itoa(max(filters.Limit, 1)))
	values.Set("offset", "0")
	return clusterAuditLogsEndpoint + "?" + values.Encode(), nil
}

func (h *EnterpriseHandler) clusterNodeID() string {
	if h == nil || h.cluster == nil {
		return ""
	}
	return h.cluster.NodeID()
}

func sortClusterRoutes(routes []clusterRouteStatus) {
	slices.SortFunc(routes, func(a, b clusterRouteStatus) int {
		if cmp := strings.Compare(a.NodeID, b.NodeID); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Address, b.Address); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(string(a.Provider), string(b.Provider)); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Model, b.Model); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.KeyID, b.KeyID)
	})
}

func sortClusterDirections(directions []clusterDirectionStatus) {
	slices.SortFunc(directions, func(a, b clusterDirectionStatus) int {
		if cmp := strings.Compare(a.NodeID, b.NodeID); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Address, b.Address); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(string(a.Provider), string(b.Provider)); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Model, b.Model)
	})
}

func sortClusterAlerts(alerts []clusterAlertRecord) {
	slices.SortFunc(alerts, func(a, b clusterAlertRecord) int {
		if cmp := b.TriggeredAt.Compare(a.TriggeredAt); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.NodeID, b.NodeID); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Address, b.Address); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.ID, b.ID)
	})
}

func sortClusterAuditEvents(events []clusterAuditEvent) {
	slices.SortFunc(events, func(a, b clusterAuditEvent) int {
		if cmp := b.Timestamp.Compare(a.Timestamp); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.NodeID, b.NodeID); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Address, b.Address); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.ID, b.ID)
	})
}

func sortClusterExportJobs(jobs []clusterExportJob) {
	slices.SortFunc(jobs, func(a, b clusterExportJob) int {
		if cmp := b.CreatedAt.Compare(a.CreatedAt); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.NodeID, b.NodeID); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(a.Address, b.Address); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.ID, b.ID)
	})
}

func sortClusterWarnings(warnings []clusterAggregationWarning) {
	slices.SortFunc(warnings, func(a, b clusterAggregationWarning) int {
		if cmp := strings.Compare(a.Address, b.Address); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Error, b.Error)
	})
}

func applyAuditPagination(response auditLogsResponse, filters enterprisecfg.AuditSearchFilters) auditLogsResponse {
	offset := max(filters.Offset, 0)
	if offset >= len(response.Events) {
		response.Events = []clusterAuditEvent{}
		return response
	}
	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	end := min(offset+limit, len(response.Events))
	response.Events = response.Events[offset:end]
	return response
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clusterConfigChangeSummary(change *ClusterConfigChange) string {
	if change == nil {
		return "scope=unknown"
	}

	parts := []string{fmt.Sprintf("scope=%s", change.Scope)}
	if change.SourceNodeID != "" {
		parts = append(parts, fmt.Sprintf("source_node=%s", change.SourceNodeID))
	}
	switch change.Scope {
	case ClusterConfigScopeProvider:
		if change.Provider != "" {
			parts = append(parts, fmt.Sprintf("provider=%s", change.Provider))
		}
	case ClusterConfigScopeLoadBalancer:
		if change.LoadBalancerConfig != nil {
			parts = append(parts, fmt.Sprintf("adaptive_routing_enabled=%t", change.LoadBalancerConfig.Enabled))
		}
	case ClusterConfigScopeVirtualKey:
		if change.VirtualKeyID != "" {
			parts = append(parts, fmt.Sprintf("virtual_key_id=%s", change.VirtualKeyID))
		}
	case ClusterConfigScopeCustomer:
		if change.CustomerID != "" {
			parts = append(parts, fmt.Sprintf("customer_id=%s", change.CustomerID))
		}
	case ClusterConfigScopeTeam:
		if change.TeamID != "" {
			parts = append(parts, fmt.Sprintf("team_id=%s", change.TeamID))
		}
	case ClusterConfigScopeRoutingRule:
		if change.RoutingRuleID != "" {
			parts = append(parts, fmt.Sprintf("routing_rule_id=%s", change.RoutingRuleID))
		}
	case ClusterConfigScopeModelConfig:
		if change.ModelConfigID != "" {
			parts = append(parts, fmt.Sprintf("model_config_id=%s", change.ModelConfigID))
		}
	case ClusterConfigScopeMCPClient:
		if change.MCPClientID != "" {
			parts = append(parts, fmt.Sprintf("mcp_client_id=%s", change.MCPClientID))
		}
	case ClusterConfigScopePlugin:
		if change.PluginName != "" {
			parts = append(parts, fmt.Sprintf("plugin=%s", change.PluginName))
		}
	case ClusterConfigScopePrompt:
		if change.PromptID != "" {
			parts = append(parts, fmt.Sprintf("prompt_id=%s", change.PromptID))
		}
	case ClusterConfigScopeFolder:
		if change.FolderID != "" {
			parts = append(parts, fmt.Sprintf("folder_id=%s", change.FolderID))
		}
	case ClusterConfigScopePromptVersion:
		if change.PromptVersionID != 0 {
			parts = append(parts, fmt.Sprintf("prompt_version_id=%d", change.PromptVersionID))
		}
	case ClusterConfigScopePromptSession:
		if change.PromptSessionID != 0 {
			parts = append(parts, fmt.Sprintf("prompt_session_id=%d", change.PromptSessionID))
		}
	case ClusterConfigScopeOAuthConfig:
		if change.OAuthConfigID != "" {
			parts = append(parts, fmt.Sprintf("oauth_config_id=%s", change.OAuthConfigID))
		}
	case ClusterConfigScopeOAuthToken:
		if change.OAuthTokenID != "" {
			parts = append(parts, fmt.Sprintf("oauth_token_id=%s", change.OAuthTokenID))
		}
	case ClusterConfigScopeSession:
		if change.SessionToken != "" {
			parts = append(parts, "session=updated")
		}
	}

	if change.Delete {
		parts = append(parts, "delete=true")
	}
	if change.FlushSessions {
		parts = append(parts, "flush_sessions=true")
	}
	return strings.Join(parts, " ")
}

func sanitizeAttachmentName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "/" || name == "" {
		return "export.bin"
	}
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "\n", "_")
	name = strings.ReplaceAll(name, "\r", "_")
	return name
}
