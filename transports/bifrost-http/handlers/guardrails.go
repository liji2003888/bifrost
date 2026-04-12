// Package handlers provides HTTP request handlers for the Bifrost HTTP transport.
// This file contains all guardrail management functionality (providers and rules).
package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/router"
	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// GuardrailsHandler manages HTTP requests for guardrail provider and rule operations.
type GuardrailsHandler struct {
	configStore configstore.ConfigStore
	propagator  ClusterConfigPropagator
}

// NewGuardrailsHandler creates a new GuardrailsHandler instance.
func NewGuardrailsHandler(configStore configstore.ConfigStore, propagator ClusterConfigPropagator) (*GuardrailsHandler, error) {
	if configStore == nil {
		return nil, fmt.Errorf("config store is required")
	}
	return &GuardrailsHandler{
		configStore: configStore,
		propagator:  propagator,
	}, nil
}

// RegisterRoutes registers all guardrail-related routes.
func (h *GuardrailsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	// Guardrail Provider CRUD
	r.GET("/api/guardrails/providers", lib.ChainMiddlewares(h.getGuardrailProviders, middlewares...))
	r.POST("/api/guardrails/providers", lib.ChainMiddlewares(h.createGuardrailProvider, middlewares...))
	r.GET("/api/guardrails/providers/{provider_id}", lib.ChainMiddlewares(h.getGuardrailProvider, middlewares...))
	r.PUT("/api/guardrails/providers/{provider_id}", lib.ChainMiddlewares(h.updateGuardrailProvider, middlewares...))
	r.DELETE("/api/guardrails/providers/{provider_id}", lib.ChainMiddlewares(h.deleteGuardrailProvider, middlewares...))

	// Guardrail Rule CRUD
	r.GET("/api/guardrails/rules", lib.ChainMiddlewares(h.getGuardrailRules, middlewares...))
	r.POST("/api/guardrails/rules", lib.ChainMiddlewares(h.createGuardrailRule, middlewares...))
	r.GET("/api/guardrails/rules/{rule_id}", lib.ChainMiddlewares(h.getGuardrailRule, middlewares...))
	r.PUT("/api/guardrails/rules/{rule_id}", lib.ChainMiddlewares(h.updateGuardrailRule, middlewares...))
	r.DELETE("/api/guardrails/rules/{rule_id}", lib.ChainMiddlewares(h.deleteGuardrailRule, middlewares...))
}

// CreateGuardrailProviderRequest represents the request body for creating a guardrail provider.
type CreateGuardrailProviderRequest struct {
	Name           string         `json:"name"`
	ProviderType   string         `json:"provider_type"`
	Enabled        *bool          `json:"enabled,omitempty"`
	TimeoutSeconds *int           `json:"timeout_seconds,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
}

// UpdateGuardrailProviderRequest represents the request body for updating a guardrail provider.
type UpdateGuardrailProviderRequest struct {
	Name           *string        `json:"name,omitempty"`
	ProviderType   *string        `json:"provider_type,omitempty"`
	Enabled        *bool          `json:"enabled,omitempty"`
	TimeoutSeconds *int           `json:"timeout_seconds,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
}

// CreateGuardrailRuleRequest represents the request body for creating a guardrail rule.
type CreateGuardrailRuleRequest struct {
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Enabled        *bool          `json:"enabled,omitempty"`
	ApplyOn        string         `json:"apply_on,omitempty"`
	ProfileIDs     []string       `json:"profile_ids,omitempty"`
	SamplingRate   *int           `json:"sampling_rate,omitempty"`
	TimeoutSeconds *int           `json:"timeout_seconds,omitempty"`
	CelExpression  string         `json:"cel_expression,omitempty"`
	Query          map[string]any `json:"query,omitempty"`
	Scope          string         `json:"scope,omitempty"`
	ScopeID        *string        `json:"scope_id,omitempty"`
	Priority       int            `json:"priority,omitempty"`
}

// UpdateGuardrailRuleRequest represents the request body for updating a guardrail rule.
type UpdateGuardrailRuleRequest struct {
	Name           *string        `json:"name,omitempty"`
	Description    *string        `json:"description,omitempty"`
	Enabled        *bool          `json:"enabled,omitempty"`
	ApplyOn        *string        `json:"apply_on,omitempty"`
	ProfileIDs     []string       `json:"profile_ids,omitempty"`
	SamplingRate   *int           `json:"sampling_rate,omitempty"`
	TimeoutSeconds *int           `json:"timeout_seconds,omitempty"`
	CelExpression  *string        `json:"cel_expression,omitempty"`
	Query          map[string]any `json:"query,omitempty"`
	Scope          *string        `json:"scope,omitempty"`
	ScopeID        *string        `json:"scope_id,omitempty"`
	Priority       *int           `json:"priority,omitempty"`
}

// validGuardrailProviderTypes contains the allowed types for guardrail providers.
var validGuardrailProviderTypes = map[string]bool{
	"bedrock":                  true,
	"azure_content_moderation": true,
	"patronus":                 true,
	"mistral_moderation":       true,
	"pangea":                   true,
}

// validGuardrailApplyOn contains the allowed values for apply_on.
var validGuardrailApplyOn = map[string]bool{
	"input":  true,
	"output": true,
	"both":   true,
}

// validGuardrailScopes contains the allowed scope values for guardrail rules.
var validGuardrailScopes = map[string]bool{
	"global":      true,
	"team":        true,
	"customer":    true,
	"virtual_key": true,
}

// --- Guardrail Provider handlers ---

// getGuardrailProviders handles GET /api/guardrails/providers
func (h *GuardrailsHandler) getGuardrailProviders(ctx *fasthttp.RequestCtx) {
	limitStr := string(ctx.QueryArgs().Peek("limit"))
	offsetStr := string(ctx.QueryArgs().Peek("offset"))
	search := string(ctx.QueryArgs().Peek("search"))

	if limitStr != "" || offsetStr != "" || search != "" {
		params := configstore.GuardrailProvidersQueryParams{
			Search: search,
		}
		if limitStr != "" {
			n, err := strconv.Atoi(limitStr)
			if err != nil {
				SendError(ctx, 400, "Invalid limit parameter: must be a number")
				return
			}
			if n < 0 {
				SendError(ctx, 400, "Invalid limit parameter: must be non-negative")
				return
			}
			params.Limit = n
		}
		if offsetStr != "" {
			n, err := strconv.Atoi(offsetStr)
			if err != nil {
				SendError(ctx, 400, "Invalid offset parameter: must be a number")
				return
			}
			if n < 0 {
				SendError(ctx, 400, "Invalid offset parameter: must be non-negative")
				return
			}
			params.Offset = n
		}

		params.Limit, params.Offset = ClampPaginationParams(params.Limit, params.Offset)
		providers, totalCount, err := h.configStore.GetGuardrailProvidersPaginated(ctx, params)
		if err != nil {
			logger.Error("failed to retrieve guardrail providers: %v", err)
			SendError(ctx, 500, "Failed to retrieve guardrail providers")
			return
		}
		SendJSON(ctx, map[string]interface{}{
			"providers":   providers,
			"count":       len(providers),
			"total_count": totalCount,
			"limit":       params.Limit,
			"offset":      params.Offset,
		})
		return
	}

	providers, err := h.configStore.GetGuardrailProviders(ctx)
	if err != nil {
		logger.Error("failed to retrieve guardrail providers: %v", err)
		SendError(ctx, 500, "Failed to retrieve guardrail providers")
		return
	}
	SendJSON(ctx, map[string]interface{}{
		"providers":   providers,
		"count":       len(providers),
		"total_count": len(providers),
		"limit":       len(providers),
		"offset":      0,
	})
}

// getGuardrailProvider handles GET /api/guardrails/providers/{provider_id}
func (h *GuardrailsHandler) getGuardrailProvider(ctx *fasthttp.RequestCtx) {
	providerID := ctx.UserValue("provider_id").(string)

	provider, err := h.configStore.GetGuardrailProvider(ctx, providerID)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Guardrail provider not found")
			return
		}
		logger.Error("failed to get guardrail provider: %v", err)
		SendError(ctx, 500, "Failed to retrieve guardrail provider")
		return
	}

	SendJSON(ctx, map[string]interface{}{
		"provider": provider,
	})
}

// createGuardrailProvider handles POST /api/guardrails/providers
func (h *GuardrailsHandler) createGuardrailProvider(ctx *fasthttp.RequestCtx) {
	var req CreateGuardrailProviderRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}

	if req.Name == "" {
		SendError(ctx, 400, "name field is required")
		return
	}
	if req.ProviderType == "" {
		SendError(ctx, 400, "provider_type field is required")
		return
	}
	if !validGuardrailProviderTypes[req.ProviderType] {
		SendError(ctx, 400, fmt.Sprintf("invalid provider_type %q: must be one of: bedrock, azure_content_moderation, patronus, mistral_moderation, pangea", req.ProviderType))
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	timeoutSeconds := 30
	if req.TimeoutSeconds != nil {
		timeoutSeconds = *req.TimeoutSeconds
	}

	now := time.Now()
	provider := &configstoreTables.TableGuardrailProvider{
		ID:             uuid.NewString(),
		Name:           req.Name,
		ProviderType:   req.ProviderType,
		Enabled:        enabled,
		TimeoutSeconds: timeoutSeconds,
		ParsedConfig:   req.Config,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.configStore.CreateGuardrailProvider(ctx, provider); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to create guardrail provider: %v", err))
		return
	}

	h.propagateGuardrailChange(ctx, &ClusterConfigChange{
		Scope:              ClusterConfigScopeGuardrailProvider,
		GuardrailProviderID: provider.ID,
		GuardrailProvider:  cloneGuardrailProvider(provider),
	})

	SendJSON(ctx, map[string]interface{}{
		"message":  "Guardrail provider created successfully",
		"provider": provider,
	})
}

// updateGuardrailProvider handles PUT /api/guardrails/providers/{provider_id}
func (h *GuardrailsHandler) updateGuardrailProvider(ctx *fasthttp.RequestCtx) {
	providerID := ctx.UserValue("provider_id").(string)

	var req UpdateGuardrailProviderRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}

	provider, err := h.configStore.GetGuardrailProvider(ctx, providerID)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Guardrail provider not found")
			return
		}
		logger.Error("failed to get guardrail provider: %v", err)
		SendError(ctx, 500, "Failed to retrieve guardrail provider")
		return
	}

	if req.Name != nil && *req.Name != "" {
		provider.Name = *req.Name
	}
	if req.ProviderType != nil {
		if !validGuardrailProviderTypes[*req.ProviderType] {
			SendError(ctx, 400, fmt.Sprintf("invalid provider_type %q: must be one of: bedrock, azure_content_moderation, patronus, mistral_moderation, pangea", *req.ProviderType))
			return
		}
		provider.ProviderType = *req.ProviderType
	}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}
	if req.TimeoutSeconds != nil {
		provider.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.Config != nil {
		provider.ParsedConfig = req.Config
	}
	provider.UpdatedAt = time.Now()

	if err := h.configStore.UpdateGuardrailProvider(ctx, provider); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to update guardrail provider: %v", err))
		return
	}

	h.propagateGuardrailChange(ctx, &ClusterConfigChange{
		Scope:              ClusterConfigScopeGuardrailProvider,
		GuardrailProviderID: provider.ID,
		GuardrailProvider:  cloneGuardrailProvider(provider),
	})

	SendJSON(ctx, map[string]interface{}{
		"message":  "Guardrail provider updated successfully",
		"provider": provider,
	})
}

// deleteGuardrailProvider handles DELETE /api/guardrails/providers/{provider_id}
func (h *GuardrailsHandler) deleteGuardrailProvider(ctx *fasthttp.RequestCtx) {
	providerID := ctx.UserValue("provider_id").(string)

	if err := h.configStore.DeleteGuardrailProvider(ctx, providerID); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Guardrail provider not found")
			return
		}
		SendError(ctx, 500, fmt.Sprintf("Failed to delete guardrail provider: %v", err))
		return
	}

	h.propagateGuardrailChange(ctx, &ClusterConfigChange{
		Scope:              ClusterConfigScopeGuardrailProvider,
		GuardrailProviderID: providerID,
		Delete:             true,
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "Guardrail provider deleted successfully",
	})
}

// --- Guardrail Rule handlers ---

// getGuardrailRules handles GET /api/guardrails/rules
func (h *GuardrailsHandler) getGuardrailRules(ctx *fasthttp.RequestCtx) {
	limitStr := string(ctx.QueryArgs().Peek("limit"))
	offsetStr := string(ctx.QueryArgs().Peek("offset"))
	search := string(ctx.QueryArgs().Peek("search"))

	if limitStr != "" || offsetStr != "" || search != "" {
		params := configstore.GuardrailRulesQueryParams{
			Search: search,
		}
		if limitStr != "" {
			n, err := strconv.Atoi(limitStr)
			if err != nil {
				SendError(ctx, 400, "Invalid limit parameter: must be a number")
				return
			}
			if n < 0 {
				SendError(ctx, 400, "Invalid limit parameter: must be non-negative")
				return
			}
			params.Limit = n
		}
		if offsetStr != "" {
			n, err := strconv.Atoi(offsetStr)
			if err != nil {
				SendError(ctx, 400, "Invalid offset parameter: must be a number")
				return
			}
			if n < 0 {
				SendError(ctx, 400, "Invalid offset parameter: must be non-negative")
				return
			}
			params.Offset = n
		}

		params.Limit, params.Offset = ClampPaginationParams(params.Limit, params.Offset)
		rules, totalCount, err := h.configStore.GetGuardrailRulesPaginated(ctx, params)
		if err != nil {
			logger.Error("failed to retrieve guardrail rules: %v", err)
			SendError(ctx, 500, "Failed to retrieve guardrail rules")
			return
		}
		SendJSON(ctx, map[string]interface{}{
			"rules":       rules,
			"count":       len(rules),
			"total_count": totalCount,
			"limit":       params.Limit,
			"offset":      params.Offset,
		})
		return
	}

	rules, err := h.configStore.GetGuardrailRules(ctx)
	if err != nil {
		logger.Error("failed to retrieve guardrail rules: %v", err)
		SendError(ctx, 500, "Failed to retrieve guardrail rules")
		return
	}
	SendJSON(ctx, map[string]interface{}{
		"rules":       rules,
		"count":       len(rules),
		"total_count": len(rules),
		"limit":       len(rules),
		"offset":      0,
	})
}

// getGuardrailRule handles GET /api/guardrails/rules/{rule_id}
func (h *GuardrailsHandler) getGuardrailRule(ctx *fasthttp.RequestCtx) {
	ruleID := ctx.UserValue("rule_id").(string)

	rule, err := h.configStore.GetGuardrailRule(ctx, ruleID)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Guardrail rule not found")
			return
		}
		logger.Error("failed to get guardrail rule: %v", err)
		SendError(ctx, 500, "Failed to retrieve guardrail rule")
		return
	}

	SendJSON(ctx, map[string]interface{}{
		"rule": rule,
	})
}

// createGuardrailRule handles POST /api/guardrails/rules
func (h *GuardrailsHandler) createGuardrailRule(ctx *fasthttp.RequestCtx) {
	var req CreateGuardrailRuleRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}

	if req.Name == "" {
		SendError(ctx, 400, "name field is required")
		return
	}

	// Normalize and validate apply_on
	applyOn := req.ApplyOn
	if applyOn == "" {
		applyOn = "both"
	}
	if !validGuardrailApplyOn[applyOn] {
		SendError(ctx, 400, "apply_on must be one of: input, output, both")
		return
	}

	// Normalize and validate scope
	scope := req.Scope
	if scope == "" {
		scope = "global"
	}
	if !validGuardrailScopes[scope] {
		SendError(ctx, 400, fmt.Sprintf("invalid scope %q: must be one of: global, team, customer, virtual_key", scope))
		return
	}

	// Validate scope_id
	if scope == "global" {
		req.ScopeID = nil
	} else if req.ScopeID == nil || *req.ScopeID == "" {
		SendError(ctx, 400, "scope_id field is required when scope is not global")
		return
	}

	// Validate sampling_rate
	samplingRate := 100
	if req.SamplingRate != nil {
		samplingRate = *req.SamplingRate
		if samplingRate < 0 || samplingRate > 100 {
			SendError(ctx, 400, "sampling_rate must be between 0 and 100")
			return
		}
	}

	timeoutSeconds := 60
	if req.TimeoutSeconds != nil {
		timeoutSeconds = *req.TimeoutSeconds
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now()
	rule := &configstoreTables.TableGuardrailRule{
		ID:               uuid.NewString(),
		Name:             req.Name,
		Description:      req.Description,
		Enabled:          enabled,
		ApplyOn:          applyOn,
		ParsedProfileIDs: req.ProfileIDs,
		SamplingRate:     samplingRate,
		TimeoutSeconds:   timeoutSeconds,
		CelExpression:    req.CelExpression,
		ParsedQuery:      req.Query,
		Scope:            scope,
		ScopeID:          req.ScopeID,
		Priority:         req.Priority,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := h.configStore.CreateGuardrailRule(ctx, rule); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to create guardrail rule: %v", err))
		return
	}

	h.propagateGuardrailChange(ctx, &ClusterConfigChange{
		Scope:          ClusterConfigScopeGuardrailRule,
		GuardrailRuleID: rule.ID,
		GuardrailRule:  cloneGuardrailRule(rule),
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "Guardrail rule created successfully",
		"rule":    rule,
	})
}

// updateGuardrailRule handles PUT /api/guardrails/rules/{rule_id}
func (h *GuardrailsHandler) updateGuardrailRule(ctx *fasthttp.RequestCtx) {
	ruleID := ctx.UserValue("rule_id").(string)

	var req UpdateGuardrailRuleRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}

	rule, err := h.configStore.GetGuardrailRule(ctx, ruleID)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Guardrail rule not found")
			return
		}
		logger.Error("failed to get guardrail rule: %v", err)
		SendError(ctx, 500, "Failed to retrieve guardrail rule")
		return
	}

	if req.Name != nil && *req.Name != "" {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.ApplyOn != nil {
		if !validGuardrailApplyOn[*req.ApplyOn] {
			SendError(ctx, 400, "apply_on must be one of: input, output, both")
			return
		}
		rule.ApplyOn = *req.ApplyOn
	}
	if req.ProfileIDs != nil {
		rule.ParsedProfileIDs = req.ProfileIDs
	}
	if req.SamplingRate != nil {
		if *req.SamplingRate < 0 || *req.SamplingRate > 100 {
			SendError(ctx, 400, "sampling_rate must be between 0 and 100")
			return
		}
		rule.SamplingRate = *req.SamplingRate
	}
	if req.TimeoutSeconds != nil {
		rule.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.CelExpression != nil {
		rule.CelExpression = *req.CelExpression
	}
	if req.Query != nil {
		rule.ParsedQuery = req.Query
	}
	if req.Scope != nil && *req.Scope != "" {
		if !validGuardrailScopes[*req.Scope] {
			SendError(ctx, 400, fmt.Sprintf("invalid scope %q: must be one of: global, team, customer, virtual_key", *req.Scope))
			return
		}
		rule.Scope = *req.Scope
	}
	if req.ScopeID != nil {
		rule.ScopeID = req.ScopeID
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}

	// Validate scope_id consistency
	if rule.Scope == "global" {
		rule.ScopeID = nil
	} else if rule.ScopeID == nil || *rule.ScopeID == "" {
		SendError(ctx, 400, "scope_id field is required when scope is not global")
		return
	}

	rule.UpdatedAt = time.Now()

	if err := h.configStore.UpdateGuardrailRule(ctx, rule); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to update guardrail rule: %v", err))
		return
	}

	h.propagateGuardrailChange(ctx, &ClusterConfigChange{
		Scope:          ClusterConfigScopeGuardrailRule,
		GuardrailRuleID: rule.ID,
		GuardrailRule:  cloneGuardrailRule(rule),
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "Guardrail rule updated successfully",
		"rule":    rule,
	})
}

// deleteGuardrailRule handles DELETE /api/guardrails/rules/{rule_id}
func (h *GuardrailsHandler) deleteGuardrailRule(ctx *fasthttp.RequestCtx) {
	ruleID := ctx.UserValue("rule_id").(string)

	if err := h.configStore.DeleteGuardrailRule(ctx, ruleID); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "Guardrail rule not found")
			return
		}
		SendError(ctx, 500, fmt.Sprintf("Failed to delete guardrail rule: %v", err))
		return
	}

	h.propagateGuardrailChange(ctx, &ClusterConfigChange{
		Scope:          ClusterConfigScopeGuardrailRule,
		GuardrailRuleID: ruleID,
		Delete:         true,
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "Guardrail rule deleted successfully",
	})
}

// propagateGuardrailChange propagates a guardrail config change to cluster peers.
func (h *GuardrailsHandler) propagateGuardrailChange(ctx *fasthttp.RequestCtx, change *ClusterConfigChange) {
	if h == nil || h.propagator == nil || change == nil {
		return
	}
	if err := h.propagator.PropagateClusterConfigChange(ctx, change); err != nil {
		logger.Warn("failed to propagate guardrail cluster config change for scope %s: %v", change.Scope, err)
	}
}

// cloneGuardrailProvider creates a shallow clone of a TableGuardrailProvider for safe cluster propagation.
func cloneGuardrailProvider(p *configstoreTables.TableGuardrailProvider) *configstoreTables.TableGuardrailProvider {
	if p == nil {
		return nil
	}
	clone := *p
	if p.ParsedConfig != nil {
		clonedConfig := make(map[string]any, len(p.ParsedConfig))
		for k, v := range p.ParsedConfig {
			clonedConfig[k] = v
		}
		clone.ParsedConfig = clonedConfig
	}
	return &clone
}

// cloneGuardrailRule creates a shallow clone of a TableGuardrailRule for safe cluster propagation.
func cloneGuardrailRule(r *configstoreTables.TableGuardrailRule) *configstoreTables.TableGuardrailRule {
	if r == nil {
		return nil
	}
	clone := *r
	if len(r.ParsedProfileIDs) > 0 {
		clone.ParsedProfileIDs = append([]string(nil), r.ParsedProfileIDs...)
	} else {
		clone.ParsedProfileIDs = nil
	}
	if r.ParsedQuery != nil {
		clonedQuery := make(map[string]any, len(r.ParsedQuery))
		for k, v := range r.ParsedQuery {
			clonedQuery[k] = v
		}
		clone.ParsedQuery = clonedQuery
	}
	return &clone
}
