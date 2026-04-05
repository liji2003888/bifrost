package handlers

import (
	"context"
	"fmt"
	"path/filepath"
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

type loadBalancerStatusProvider interface {
	ListSnapshots(provider schemas.ModelProvider, model string) []loadbalancer.RouteStatus
	ListDirectionSnapshots(provider schemas.ModelProvider, model string) []loadbalancer.DirectionStatus
}

type EnterpriseHandler struct {
	cluster *enterprisecfg.ClusterService
	audit   *enterprisecfg.AuditService
	exports *enterprisecfg.LogExportService
	alerts  *enterprisecfg.AlertManager
	vault   *enterprisecfg.VaultService
	lb      loadBalancerStatusProvider
}

func NewEnterpriseHandler(cluster *enterprisecfg.ClusterService, audit *enterprisecfg.AuditService, exports *enterprisecfg.LogExportService, alerts *enterprisecfg.AlertManager, vault *enterprisecfg.VaultService, lb loadBalancerStatusProvider) *EnterpriseHandler {
	if cluster == nil && audit == nil && exports == nil && alerts == nil && vault == nil && lb == nil {
		return nil
	}
	return &EnterpriseHandler{
		cluster: cluster,
		audit:   audit,
		exports: exports,
		alerts:  alerts,
		vault:   vault,
		lb:      lb,
	}
}

func (h *EnterpriseHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	if h == nil {
		return
	}

	if h.cluster != nil {
		r.GET("/api/cluster/status", lib.ChainMiddlewares(h.getClusterStatus, middlewares...))
		r.GET("/_cluster/status", h.getInternalClusterStatus)
		r.POST("/_cluster/kv/set", h.applyClusterSet)
		r.POST("/_cluster/kv/delete", h.applyClusterDelete)
	}
	if h.audit != nil {
		r.GET("/api/audit-logs", lib.ChainMiddlewares(h.getAuditLogs, middlewares...))
	}
	if h.exports != nil {
		r.POST("/api/logs/exports", lib.ChainMiddlewares(h.createLogsExport, middlewares...))
		r.POST("/api/mcp-logs/exports", lib.ChainMiddlewares(h.createMCPLogsExport, middlewares...))
		r.GET("/api/log-exports", lib.ChainMiddlewares(h.listExportJobs, middlewares...))
		r.GET("/api/log-exports/{id}", lib.ChainMiddlewares(h.getExportJob, middlewares...))
		r.GET("/api/log-exports/{id}/download", lib.ChainMiddlewares(h.downloadExportJob, middlewares...))
	}
	if h.alerts != nil {
		r.GET("/api/alerts", lib.ChainMiddlewares(h.getAlerts, middlewares...))
	}
	if h.vault != nil {
		r.GET("/api/vault/status", lib.ChainMiddlewares(h.getVaultStatus, middlewares...))
	}
	if h.lb != nil {
		r.GET("/api/adaptive-routing/status", lib.ChainMiddlewares(h.getAdaptiveRoutingStatus, middlewares...))
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

func (h *EnterpriseHandler) getAuditLogs(ctx *fasthttp.RequestCtx) {
	if h.audit == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "audit service is not enabled")
		return
	}

	filters := enterprisecfg.AuditSearchFilters{
		Category:     enterprisecfg.AuditCategory(string(ctx.QueryArgs().Peek("category"))),
		Action:       string(ctx.QueryArgs().Peek("action")),
		ResourceType: string(ctx.QueryArgs().Peek("resource_type")),
		ActorID:      string(ctx.QueryArgs().Peek("actor_id")),
		Limit:        100,
	}

	if limitStr := string(ctx.QueryArgs().Peek("limit")); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filters.Limit = limit
		}
	}
	if offsetStr := string(ctx.QueryArgs().Peek("offset")); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filters.Offset = offset
		}
	}
	if startStr := string(ctx.QueryArgs().Peek("start_time")); startStr != "" {
		if start, err := time.Parse(time.RFC3339, startStr); err == nil {
			filters.StartTime = &start
		}
	}
	if endStr := string(ctx.QueryArgs().Peek("end_time")); endStr != "" {
		if end, err := time.Parse(time.RFC3339, endStr); err == nil {
			filters.EndTime = &end
		}
	}

	result, err := h.audit.Search(filters)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to search audit logs: %v", err))
		return
	}
	SendJSON(ctx, result)
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
	SendJSON(ctx, map[string]any{"jobs": h.exports.ListJobs()})
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
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to stat export file: %v", err))
		return
	}

	ctx.Response.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sanitizeAttachmentName(h.exports.DownloadFileName(job))))
	ctx.SetContentType(h.exports.DownloadContentType(job))
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.Header.Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	ctx.Response.SetBodyStream(file, int(info.Size()))
}

func (h *EnterpriseHandler) getAlerts(ctx *fasthttp.RequestCtx) {
	if h.alerts == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "alert service is not enabled")
		return
	}
	SendJSON(ctx, map[string]any{"alerts": h.alerts.ListActive()})
}

func (h *EnterpriseHandler) getVaultStatus(ctx *fasthttp.RequestCtx) {
	if h.vault == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "vault service is not enabled")
		return
	}
	SendJSON(ctx, h.vault.Status())
}

func (h *EnterpriseHandler) getAdaptiveRoutingStatus(ctx *fasthttp.RequestCtx) {
	if h.lb == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "adaptive routing is not enabled")
		return
	}

	provider := schemas.ModelProvider(strings.TrimSpace(string(ctx.QueryArgs().Peek("provider"))))
	model := strings.TrimSpace(string(ctx.QueryArgs().Peek("model")))
	SendJSON(ctx, map[string]any{
		"routes":     h.lb.ListSnapshots(provider, model),
		"directions": h.lb.ListDirectionSnapshots(provider, model),
	})
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
