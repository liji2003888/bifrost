package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	enterprisecfg "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

type EnterpriseHandler struct {
	cluster *enterprisecfg.ClusterService
	audit   *enterprisecfg.AuditService
	exports *enterprisecfg.LogExportService
	alerts  *enterprisecfg.AlertManager
}

func NewEnterpriseHandler(cluster *enterprisecfg.ClusterService, audit *enterprisecfg.AuditService, exports *enterprisecfg.LogExportService, alerts *enterprisecfg.AlertManager) *EnterpriseHandler {
	if cluster == nil && audit == nil && exports == nil && alerts == nil {
		return nil
	}
	return &EnterpriseHandler{
		cluster: cluster,
		audit:   audit,
		exports: exports,
		alerts:  alerts,
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
	}
	if h.alerts != nil {
		r.GET("/api/alerts", lib.ChainMiddlewares(h.getAlerts, middlewares...))
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
	SendJSON(ctx, h.cluster.Status())
}

func (h *EnterpriseHandler) applyClusterSet(ctx *fasthttp.RequestCtx) {
	if h.cluster == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "cluster service is not enabled")
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

	actorID, _ := ctx.UserValue(schemas.BifrostContextKeySessionToken).(string)
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

func (h *EnterpriseHandler) getAlerts(ctx *fasthttp.RequestCtx) {
	if h.alerts == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "alert service is not enabled")
		return
	}
	SendJSON(ctx, map[string]any{"alerts": h.alerts.ListActive()})
}
