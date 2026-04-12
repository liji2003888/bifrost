// Package handlers provides HTTP request handlers for the Bifrost HTTP transport.
// This file contains all log export configuration management functionality.
package handlers

import (
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/router"
	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	enterprise "github.com/maximhq/bifrost/transports/bifrost-http/enterprise"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// LogExportConfigsHandler manages HTTP requests for log export configuration CRUD and manual triggers.
type LogExportConfigsHandler struct {
	configStore configstore.ConfigStore
	scheduler   *enterprise.ExportScheduler
	exporter    *enterprise.LogExportService
	propagator  ClusterConfigPropagator
}

// NewLogExportConfigsHandler creates a new LogExportConfigsHandler.
func NewLogExportConfigsHandler(
	configStore configstore.ConfigStore,
	scheduler *enterprise.ExportScheduler,
	exporter *enterprise.LogExportService,
	propagator ClusterConfigPropagator,
) (*LogExportConfigsHandler, error) {
	if configStore == nil {
		return nil, fmt.Errorf("config store is required")
	}
	return &LogExportConfigsHandler{
		configStore: configStore,
		scheduler:   scheduler,
		exporter:    exporter,
		propagator:  propagator,
	}, nil
}

// RegisterRoutes registers all log export config routes.
func (h *LogExportConfigsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/log-export-configs", lib.ChainMiddlewares(h.listConfigs, middlewares...))
	r.POST("/api/log-export-configs", lib.ChainMiddlewares(h.createConfig, middlewares...))
	r.GET("/api/log-export-configs/{id}", lib.ChainMiddlewares(h.getConfig, middlewares...))
	r.PUT("/api/log-export-configs/{id}", lib.ChainMiddlewares(h.updateConfig, middlewares...))
	r.DELETE("/api/log-export-configs/{id}", lib.ChainMiddlewares(h.deleteConfig, middlewares...))
	r.POST("/api/log-export-configs/{id}/run", lib.ChainMiddlewares(h.runConfig, middlewares...))
}

// ─────────────────────────────────────────────────────────────────────────────
// Request / Response types
// ─────────────────────────────────────────────────────────────────────────────

// CreateLogExportConfigRequest is the request body for creating a log export config.
type CreateLogExportConfigRequest struct {
	Name              string         `json:"name"`
	Description       string         `json:"description,omitempty"`
	Enabled           *bool          `json:"enabled,omitempty"`
	Frequency         string         `json:"frequency,omitempty"`
	ScheduleTime      string         `json:"schedule_time,omitempty"`
	ScheduleDay       string         `json:"schedule_day,omitempty"`
	Timezone          string         `json:"timezone,omitempty"`
	DestinationType   string         `json:"destination_type"`
	DestinationConfig map[string]any `json:"destination_config,omitempty"`
	Format            string         `json:"format,omitempty"`
	Compression       string         `json:"compression,omitempty"`
	MaxRows           int            `json:"max_rows,omitempty"`
	DataScope         string         `json:"data_scope,omitempty"`
	Filters           map[string]any `json:"filters,omitempty"`
}

// UpdateLogExportConfigRequest is the request body for updating a log export config.
type UpdateLogExportConfigRequest struct {
	Name              *string        `json:"name,omitempty"`
	Description       *string        `json:"description,omitempty"`
	Enabled           *bool          `json:"enabled,omitempty"`
	Frequency         *string        `json:"frequency,omitempty"`
	ScheduleTime      *string        `json:"schedule_time,omitempty"`
	ScheduleDay       *string        `json:"schedule_day,omitempty"`
	Timezone          *string        `json:"timezone,omitempty"`
	DestinationType   *string        `json:"destination_type,omitempty"`
	DestinationConfig map[string]any `json:"destination_config,omitempty"`
	Format            *string        `json:"format,omitempty"`
	Compression       *string        `json:"compression,omitempty"`
	MaxRows           *int           `json:"max_rows,omitempty"`
	DataScope         *string        `json:"data_scope,omitempty"`
	Filters           map[string]any `json:"filters,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Validation helpers
// ─────────────────────────────────────────────────────────────────────────────

var validExportFrequencies = map[string]bool{
	"daily":   true,
	"weekly":  true,
	"monthly": true,
}

var validExportFormats = map[string]bool{
	"jsonl": true,
	"csv":   true,
}

var validExportCompressions = map[string]bool{
	"":     true,
	"none": true,
	"gzip": true,
}

var validExportDestTypes = map[string]bool{
	"local":      true,
	"s3":         true,
	"gcs":        true,
	"azure_blob": true,
}

var validExportScopes = map[string]bool{
	"logs":     true,
	"mcp_logs": true,
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

// listConfigs handles GET /api/log-export-configs
func (h *LogExportConfigsHandler) listConfigs(ctx *fasthttp.RequestCtx) {
	configs, err := h.configStore.GetLogExportConfigs(ctx)
	if err != nil {
		logger.Error("failed to retrieve log export configs: %v", err)
		SendError(ctx, 500, "Failed to retrieve log export configs")
		return
	}
	SendJSON(ctx, map[string]interface{}{
		"configs":     configs,
		"count":       len(configs),
		"total_count": len(configs),
	})
}

// getConfig handles GET /api/log-export-configs/{id}
func (h *LogExportConfigsHandler) getConfig(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	cfg, err := h.configStore.GetLogExportConfig(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Log export config not found")
			return
		}
		logger.Error("failed to get log export config: %v", err)
		SendError(ctx, 500, "Failed to retrieve log export config")
		return
	}

	SendJSON(ctx, map[string]interface{}{
		"config": cfg,
	})
}

// createConfig handles POST /api/log-export-configs
func (h *LogExportConfigsHandler) createConfig(ctx *fasthttp.RequestCtx) {
	var req CreateLogExportConfigRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}

	if req.Name == "" {
		SendError(ctx, 400, "name field is required")
		return
	}
	if req.DestinationType == "" {
		SendError(ctx, 400, "destination_type field is required")
		return
	}
	if !validExportDestTypes[req.DestinationType] {
		SendError(ctx, 400, fmt.Sprintf("invalid destination_type %q: must be one of: local, s3, gcs, azure_blob", req.DestinationType))
		return
	}

	frequency := req.Frequency
	if frequency == "" {
		frequency = "daily"
	}
	if !validExportFrequencies[frequency] {
		SendError(ctx, 400, "frequency must be one of: daily, weekly, monthly")
		return
	}

	scheduleTime := req.ScheduleTime
	if scheduleTime == "" {
		scheduleTime = "02:00"
	}

	timezone := req.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	format := req.Format
	if format == "" {
		format = "jsonl"
	}
	if !validExportFormats[format] {
		SendError(ctx, 400, "format must be one of: jsonl, csv")
		return
	}

	compression := req.Compression
	if !validExportCompressions[compression] {
		SendError(ctx, 400, "compression must be one of: none, gzip")
		return
	}
	if compression == "none" {
		compression = ""
	}

	dataScope := req.DataScope
	if dataScope == "" {
		dataScope = "logs"
	}
	if !validExportScopes[dataScope] {
		SendError(ctx, 400, "data_scope must be one of: logs, mcp_logs")
		return
	}

	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = 100000
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now()
	config := &configstoreTables.TableLogExportConfig{
		ID:                     uuid.NewString(),
		Name:                   req.Name,
		Description:            req.Description,
		Enabled:                enabled,
		Frequency:              frequency,
		ScheduleTime:           scheduleTime,
		ScheduleDay:            req.ScheduleDay,
		Timezone:               timezone,
		DestinationType:        req.DestinationType,
		ParsedDestinationConfig: req.DestinationConfig,
		Format:                 format,
		Compression:            compression,
		MaxRows:                maxRows,
		DataScope:              dataScope,
		ParsedFilters:          req.Filters,
		LastRunStatus:          "",
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := h.configStore.CreateLogExportConfig(ctx, config); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to create log export config: %v", err))
		return
	}

	h.propagateChange(ctx, &ClusterConfigChange{
		Scope:               ClusterConfigScopeLogExportConfig,
		LogExportConfigID:   config.ID,
		LogExportConfig:     cloneLogExportConfig(config),
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "Log export config created successfully",
		"config":  config,
	})
}

// updateConfig handles PUT /api/log-export-configs/{id}
func (h *LogExportConfigsHandler) updateConfig(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	var req UpdateLogExportConfigRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}

	cfg, err := h.configStore.GetLogExportConfig(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Log export config not found")
			return
		}
		logger.Error("failed to get log export config: %v", err)
		SendError(ctx, 500, "Failed to retrieve log export config")
		return
	}

	if req.Name != nil && *req.Name != "" {
		cfg.Name = *req.Name
	}
	if req.Description != nil {
		cfg.Description = *req.Description
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.Frequency != nil {
		if !validExportFrequencies[*req.Frequency] {
			SendError(ctx, 400, "frequency must be one of: daily, weekly, monthly")
			return
		}
		cfg.Frequency = *req.Frequency
		// Reset NextRunAt so scheduler recalculates.
		cfg.NextRunAt = nil
	}
	if req.ScheduleTime != nil {
		cfg.ScheduleTime = *req.ScheduleTime
		cfg.NextRunAt = nil
	}
	if req.ScheduleDay != nil {
		cfg.ScheduleDay = *req.ScheduleDay
		cfg.NextRunAt = nil
	}
	if req.Timezone != nil {
		cfg.Timezone = *req.Timezone
		cfg.NextRunAt = nil
	}
	if req.DestinationType != nil {
		if !validExportDestTypes[*req.DestinationType] {
			SendError(ctx, 400, fmt.Sprintf("invalid destination_type %q: must be one of: local, s3, gcs, azure_blob", *req.DestinationType))
			return
		}
		cfg.DestinationType = *req.DestinationType
	}
	if req.DestinationConfig != nil {
		cfg.ParsedDestinationConfig = req.DestinationConfig
	}
	if req.Format != nil {
		if !validExportFormats[*req.Format] {
			SendError(ctx, 400, "format must be one of: jsonl, csv")
			return
		}
		cfg.Format = *req.Format
	}
	if req.Compression != nil {
		if !validExportCompressions[*req.Compression] {
			SendError(ctx, 400, "compression must be one of: none, gzip")
			return
		}
		comp := *req.Compression
		if comp == "none" {
			comp = ""
		}
		cfg.Compression = comp
	}
	if req.MaxRows != nil {
		if *req.MaxRows > 0 {
			cfg.MaxRows = *req.MaxRows
		}
	}
	if req.DataScope != nil {
		if !validExportScopes[*req.DataScope] {
			SendError(ctx, 400, "data_scope must be one of: logs, mcp_logs")
			return
		}
		cfg.DataScope = *req.DataScope
	}
	if req.Filters != nil {
		cfg.ParsedFilters = req.Filters
	}

	cfg.UpdatedAt = time.Now()

	if err := h.configStore.UpdateLogExportConfig(ctx, cfg); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to update log export config: %v", err))
		return
	}

	h.propagateChange(ctx, &ClusterConfigChange{
		Scope:             ClusterConfigScopeLogExportConfig,
		LogExportConfigID: cfg.ID,
		LogExportConfig:   cloneLogExportConfig(cfg),
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "Log export config updated successfully",
		"config":  cfg,
	})
}

// deleteConfig handles DELETE /api/log-export-configs/{id}
func (h *LogExportConfigsHandler) deleteConfig(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	if err := h.configStore.DeleteLogExportConfig(ctx, id); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Log export config not found")
			return
		}
		SendError(ctx, 500, fmt.Sprintf("Failed to delete log export config: %v", err))
		return
	}

	h.propagateChange(ctx, &ClusterConfigChange{
		Scope:             ClusterConfigScopeLogExportConfig,
		LogExportConfigID: id,
		Delete:            true,
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "Log export config deleted successfully",
	})
}

// runConfig handles POST /api/log-export-configs/{id}/run
// Triggers an immediate (manual) export run for the given config.
func (h *LogExportConfigsHandler) runConfig(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	if h.exporter == nil {
		SendError(ctx, 503, "Log export service is not enabled")
		return
	}

	cfg, err := h.configStore.GetLogExportConfig(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Log export config not found")
			return
		}
		logger.Error("failed to get log export config: %v", err)
		SendError(ctx, 500, "Failed to retrieve log export config")
		return
	}

	req := enterprise.LogExportRequest{
		Scope:       enterprise.ExportScope(cfg.DataScope),
		Format:      cfg.Format,
		Compression: cfg.Compression,
		MaxRows:     cfg.MaxRows,
	}

	actorID := "manual:" + id
	job, err := h.exporter.Submit(ctx, req, actorID)
	if err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to submit export job: %v", err))
		return
	}

	SendJSON(ctx, map[string]interface{}{
		"message": "Export job submitted successfully",
		"job":     job,
	})
}

// propagateChange propagates a log export config change to cluster peers.
func (h *LogExportConfigsHandler) propagateChange(ctx *fasthttp.RequestCtx, change *ClusterConfigChange) {
	if h == nil || h.propagator == nil || change == nil {
		return
	}
	if err := h.propagator.PropagateClusterConfigChange(ctx, change); err != nil {
		logger.Warn("failed to propagate log export config cluster change for scope %s: %v", change.Scope, err)
	}
}

// cloneLogExportConfig creates a shallow clone of a TableLogExportConfig for safe cluster propagation.
func cloneLogExportConfig(c *configstoreTables.TableLogExportConfig) *configstoreTables.TableLogExportConfig {
	if c == nil {
		return nil
	}
	clone := *c
	if c.ParsedDestinationConfig != nil {
		cloned := make(map[string]any, len(c.ParsedDestinationConfig))
		for k, v := range c.ParsedDestinationConfig {
			cloned[k] = v
		}
		clone.ParsedDestinationConfig = cloned
	}
	if c.ParsedFilters != nil {
		cloned := make(map[string]any, len(c.ParsedFilters))
		for k, v := range c.ParsedFilters {
			cloned[k] = v
		}
		clone.ParsedFilters = cloned
	}
	return &clone
}
