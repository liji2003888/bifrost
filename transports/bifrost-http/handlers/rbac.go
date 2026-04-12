// Package handlers provides HTTP request handlers for the Bifrost HTTP transport.
// This file contains RBAC (Role-Based Access Control) management functionality.
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
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// RBACHandler manages HTTP requests for RBAC role and permission operations.
type RBACHandler struct {
	configStore configstore.ConfigStore
	propagator  ClusterConfigPropagator
}

// NewRBACHandler creates a new RBACHandler instance.
func NewRBACHandler(configStore configstore.ConfigStore, propagator ClusterConfigPropagator) (*RBACHandler, error) {
	if configStore == nil {
		return nil, fmt.Errorf("config store is required")
	}
	return &RBACHandler{
		configStore: configStore,
		propagator:  propagator,
	}, nil
}

// RegisterRoutes registers all RBAC-related routes.
func (h *RBACHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	// Role CRUD
	r.GET("/api/rbac/roles", lib.ChainMiddlewares(h.getRoles, middlewares...))
	r.POST("/api/rbac/roles", lib.ChainMiddlewares(h.createRole, middlewares...))
	r.GET("/api/rbac/roles/{id}", lib.ChainMiddlewares(h.getRole, middlewares...))
	r.PUT("/api/rbac/roles/{id}", lib.ChainMiddlewares(h.updateRole, middlewares...))
	r.DELETE("/api/rbac/roles/{id}", lib.ChainMiddlewares(h.deleteRole, middlewares...))

	// User-role assignments
	r.GET("/api/rbac/users", lib.ChainMiddlewares(h.getUserRoles, middlewares...))
	r.PUT("/api/rbac/users/{user_id}", lib.ChainMiddlewares(h.setUserRole, middlewares...))

	// Metadata
	r.GET("/api/rbac/resources", lib.ChainMiddlewares(h.getResources, middlewares...))
	r.GET("/api/rbac/check", lib.ChainMiddlewares(h.checkPermission, middlewares...))
}

// ─── Request / Response types ─────────────────────────────────────────────────

// RbacPermissionInput represents a single permission (resource + operation) in a request.
type RbacPermissionInput struct {
	Resource  string `json:"resource"`
	Operation string `json:"operation"`
}

// CreateRbacRoleRequest is the request body for creating a custom RBAC role.
type CreateRbacRoleRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	IsDefault   bool                  `json:"is_default,omitempty"`
	Permissions []RbacPermissionInput `json:"permissions,omitempty"`
}

// UpdateRbacRoleRequest is the request body for updating an RBAC role.
type UpdateRbacRoleRequest struct {
	Name        *string               `json:"name,omitempty"`
	Description *string               `json:"description,omitempty"`
	IsDefault   *bool                 `json:"is_default,omitempty"`
	Permissions []RbacPermissionInput `json:"permissions,omitempty"`
}

// SetUserRoleRequest is the request body for assigning a role to a user.
type SetUserRoleRequest struct {
	RoleID string `json:"role_id"`
}

// ─── Validation ───────────────────────────────────────────────────────────────

var validRbacResources = map[string]bool{
	"GuardrailsConfig":         true,
	"GuardrailsProviders":      true,
	"GuardrailRules":           true,
	"UserProvisioning":         true,
	"Cluster":                  true,
	"Settings":                 true,
	"Users":                    true,
	"Logs":                     true,
	"Observability":            true,
	"VirtualKeys":              true,
	"ModelProvider":            true,
	"Plugins":                  true,
	"MCPGateway":               true,
	"AdaptiveRouter":           true,
	"AuditLogs":                true,
	"Customers":                true,
	"Teams":                    true,
	"RBAC":                     true,
	"Governance":               true,
	"RoutingRules":             true,
	"PIIRedactor":              true,
	"PromptRepository":         true,
	"PromptDeploymentStrategy": true,
}

var validRbacOperations = map[string]bool{
	"Read":     true,
	"View":     true,
	"Create":   true,
	"Update":   true,
	"Delete":   true,
	"Download": true,
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// getRoles handles GET /api/rbac/roles — returns all roles with their permissions.
func (h *RBACHandler) getRoles(ctx *fasthttp.RequestCtx) {
	roles, err := h.configStore.GetAllRbacRoles(ctx)
	if err != nil {
		logger.Error("failed to retrieve RBAC roles: %v", err)
		SendError(ctx, 500, "Failed to retrieve RBAC roles")
		return
	}
	SendJSON(ctx, map[string]interface{}{
		"roles": roles,
		"count": len(roles),
	})
}

// getRole handles GET /api/rbac/roles/{id} — returns a single role with permissions.
func (h *RBACHandler) getRole(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	role, err := h.configStore.GetRbacRole(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "RBAC role not found")
			return
		}
		logger.Error("failed to get RBAC role: %v", err)
		SendError(ctx, 500, "Failed to retrieve RBAC role")
		return
	}
	SendJSON(ctx, map[string]interface{}{"role": role})
}

// createRole handles POST /api/rbac/roles — creates a new custom role.
func (h *RBACHandler) createRole(ctx *fasthttp.RequestCtx) {
	var req CreateRbacRoleRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}
	if req.Name == "" {
		SendError(ctx, 400, "name field is required")
		return
	}
	// Validate permissions
	for _, p := range req.Permissions {
		if !validRbacResources[p.Resource] {
			SendError(ctx, 400, fmt.Sprintf("invalid resource %q", p.Resource))
			return
		}
		if !validRbacOperations[p.Operation] {
			SendError(ctx, 400, fmt.Sprintf("invalid operation %q", p.Operation))
			return
		}
	}

	now := time.Now()
	roleID := uuid.NewString()
	role := &configstoreTables.TableRbacRole{
		ID:          roleID,
		Name:        req.Name,
		Description: req.Description,
		IsDefault:   req.IsDefault,
		IsSystem:    false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.configStore.CreateRbacRole(ctx, role); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to create RBAC role: %v", err))
		return
	}

	// Set permissions
	perms := make([]configstoreTables.TableRbacPermission, 0, len(req.Permissions))
	for _, p := range req.Permissions {
		perms = append(perms, configstoreTables.TableRbacPermission{
			RoleID:    roleID,
			Resource:  p.Resource,
			Operation: p.Operation,
		})
	}
	if err := h.configStore.SetRbacPermissions(ctx, roleID, perms); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to set RBAC permissions: %v", err))
		return
	}

	role.Permissions = perms

	h.propagateChange(ctx, &ClusterConfigChange{
		Scope:      ClusterConfigScopeRbac,
		RbacRoleID: roleID,
		RbacRole:   role,
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "RBAC role created successfully",
		"role":    role,
	})
}

// updateRole handles PUT /api/rbac/roles/{id} — updates an existing role.
func (h *RBACHandler) updateRole(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	var req UpdateRbacRoleRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}

	role, err := h.configStore.GetRbacRole(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "RBAC role not found")
			return
		}
		logger.Error("failed to get RBAC role for update: %v", err)
		SendError(ctx, 500, "Failed to retrieve RBAC role")
		return
	}

	// System roles: only allow permission changes, not name/description
	if role.IsSystem {
		if req.Name != nil && *req.Name != role.Name {
			SendError(ctx, 400, "Cannot rename a system role")
			return
		}
	} else {
		if req.Name != nil && *req.Name != "" {
			role.Name = *req.Name
		}
		if req.Description != nil {
			role.Description = *req.Description
		}
	}
	if req.IsDefault != nil {
		role.IsDefault = *req.IsDefault
	}
	role.UpdatedAt = time.Now()

	if err := h.configStore.UpdateRbacRole(ctx, role); err != nil {
		SendError(ctx, 500, fmt.Sprintf("Failed to update RBAC role: %v", err))
		return
	}

	// Update permissions if provided
	if req.Permissions != nil {
		for _, p := range req.Permissions {
			if !validRbacResources[p.Resource] {
				SendError(ctx, 400, fmt.Sprintf("invalid resource %q", p.Resource))
				return
			}
			if !validRbacOperations[p.Operation] {
				SendError(ctx, 400, fmt.Sprintf("invalid operation %q", p.Operation))
				return
			}
		}
		perms := make([]configstoreTables.TableRbacPermission, 0, len(req.Permissions))
		for _, p := range req.Permissions {
			perms = append(perms, configstoreTables.TableRbacPermission{
				RoleID:    id,
				Resource:  p.Resource,
				Operation: p.Operation,
			})
		}
		if err := h.configStore.SetRbacPermissions(ctx, id, perms); err != nil {
			SendError(ctx, 500, fmt.Sprintf("Failed to update RBAC permissions: %v", err))
			return
		}
		role.Permissions = perms
	}

	h.propagateChange(ctx, &ClusterConfigChange{
		Scope:      ClusterConfigScopeRbac,
		RbacRoleID: id,
		RbacRole:   role,
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "RBAC role updated successfully",
		"role":    role,
	})
}

// deleteRole handles DELETE /api/rbac/roles/{id} — deletes a custom role.
func (h *RBACHandler) deleteRole(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	role, err := h.configStore.GetRbacRole(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "RBAC role not found")
			return
		}
		logger.Error("failed to get RBAC role for delete: %v", err)
		SendError(ctx, 500, "Failed to retrieve RBAC role")
		return
	}
	if role.IsSystem {
		SendError(ctx, 400, "Cannot delete a system role")
		return
	}

	if err := h.configStore.DeleteRbacRole(ctx, id); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "RBAC role not found")
			return
		}
		logger.Error("failed to delete RBAC role: %v", err)
		SendError(ctx, 500, "Failed to delete RBAC role")
		return
	}

	h.propagateChange(ctx, &ClusterConfigChange{
		Scope:      ClusterConfigScopeRbac,
		RbacRoleID: id,
		Delete:     true,
	})

	SendJSON(ctx, map[string]interface{}{
		"message": "RBAC role deleted successfully",
	})
}

// getUserRoles handles GET /api/rbac/users — returns all user-role mappings.
// Optionally filter by ?user_id=...
func (h *RBACHandler) getUserRoles(ctx *fasthttp.RequestCtx) {
	userID := string(ctx.QueryArgs().Peek("user_id"))
	if userID != "" {
		userRoles, err := h.configStore.GetRbacUserRoles(ctx, userID)
		if err != nil {
			logger.Error("failed to retrieve user RBAC roles: %v", err)
			SendError(ctx, 500, "Failed to retrieve user RBAC roles")
			return
		}
		SendJSON(ctx, map[string]interface{}{
			"user_roles": userRoles,
			"count":      len(userRoles),
		})
		return
	}
	// Return all user-role mappings by returning an empty slice with a note
	// (full listing not supported without a dedicated ListAllUserRoles method)
	SendJSON(ctx, map[string]interface{}{
		"user_roles": []interface{}{},
		"count":      0,
		"message":    "Provide ?user_id= to filter by user",
	})
}

// setUserRole handles PUT /api/rbac/users/{user_id} — assigns a role to a user.
func (h *RBACHandler) setUserRole(ctx *fasthttp.RequestCtx) {
	userID := ctx.UserValue("user_id").(string)
	if userID == "" {
		SendError(ctx, 400, "user_id is required")
		return
	}

	var req SetUserRoleRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, 400, "Invalid JSON")
		return
	}
	if req.RoleID == "" {
		SendError(ctx, 400, "role_id field is required")
		return
	}

	// Verify the role exists
	if _, err := h.configStore.GetRbacRole(ctx, req.RoleID); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, 404, "RBAC role not found")
			return
		}
		logger.Error("failed to verify RBAC role: %v", err)
		SendError(ctx, 500, "Failed to verify RBAC role")
		return
	}

	if err := h.configStore.SetRbacUserRole(ctx, userID, req.RoleID); err != nil {
		logger.Error("failed to set user RBAC role: %v", err)
		SendError(ctx, 500, "Failed to assign RBAC role to user")
		return
	}

	SendJSON(ctx, map[string]interface{}{
		"message": "RBAC role assigned to user successfully",
		"user_id": userID,
		"role_id": req.RoleID,
	})
}

// getResources handles GET /api/rbac/resources — returns all valid resources and operations.
func (h *RBACHandler) getResources(ctx *fasthttp.RequestCtx) {
	resources := make([]string, 0, len(validRbacResources))
	for r := range validRbacResources {
		resources = append(resources, r)
	}
	operations := make([]string, 0, len(validRbacOperations))
	for o := range validRbacOperations {
		operations = append(operations, o)
	}
	SendJSON(ctx, map[string]interface{}{
		"resources":  resources,
		"operations": operations,
	})
}

// checkPermission handles GET /api/rbac/check?user_id=...&resource=...&operation=...
func (h *RBACHandler) checkPermission(ctx *fasthttp.RequestCtx) {
	userID := string(ctx.QueryArgs().Peek("user_id"))
	resource := string(ctx.QueryArgs().Peek("resource"))
	operation := string(ctx.QueryArgs().Peek("operation"))

	if userID == "" || resource == "" || operation == "" {
		SendError(ctx, 400, "user_id, resource, and operation query parameters are required")
		return
	}

	userRoles, err := h.configStore.GetRbacUserRoles(ctx, userID)
	if err != nil {
		logger.Error("failed to get user RBAC roles for permission check: %v", err)
		SendError(ctx, 500, "Failed to check permission")
		return
	}

	allowed := false
	for _, ur := range userRoles {
		perms, err := h.configStore.GetRbacPermissionsByRole(ctx, ur.RoleID)
		if err != nil {
			continue
		}
		for _, p := range perms {
			if p.Resource == resource && p.Operation == operation {
				allowed = true
				break
			}
		}
		if allowed {
			break
		}
	}

	SendJSON(ctx, map[string]interface{}{
		"allowed":   allowed,
		"user_id":   userID,
		"resource":  resource,
		"operation": operation,
	})
}

// propagateChange sends a cluster config change notification if a propagator is configured.
func (h *RBACHandler) propagateChange(ctx *fasthttp.RequestCtx, change *ClusterConfigChange) {
	if h.propagator == nil {
		return
	}
	if err := h.propagator.PropagateClusterConfigChange(ctx, change); err != nil {
		logger.Error("failed to propagate RBAC cluster config change: %v", err)
	}
}
