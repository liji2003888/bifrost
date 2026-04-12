/**
 * RBAC Type Definitions
 * Defines all TypeScript interfaces for the Role-Based Access Control feature
 */

// ─── Permission ────────────────────────────────────────────────────────────────

export interface RbacPermission {
	id: number;
	role_id: string;
	resource: string;
	operation: string;
}

export interface RbacPermissionInput {
	resource: string;
	operation: string;
}

// ─── Role ──────────────────────────────────────────────────────────────────────

export interface RbacRole {
	id: string;
	name: string;
	description: string;
	is_default: boolean;
	is_system: boolean;
	permissions: RbacPermission[];
	created_at: string;
	updated_at: string;
}

export interface CreateRbacRoleRequest {
	name: string;
	description?: string;
	is_default?: boolean;
	permissions?: RbacPermissionInput[];
}

export interface UpdateRbacRoleRequest {
	name?: string;
	description?: string;
	is_default?: boolean;
	permissions?: RbacPermissionInput[];
}

export interface GetRbacRolesResponse {
	roles: RbacRole[];
	count: number;
}

// ─── User-Role mapping ─────────────────────────────────────────────────────────

export interface RbacUserRole {
	id: number;
	user_id: string;
	role_id: string;
}

export interface SetUserRoleRequest {
	role_id: string;
}

export interface GetRbacUserRolesResponse {
	user_roles: RbacUserRole[];
	count: number;
}

// ─── Resources and Operations ──────────────────────────────────────────────────

export interface GetRbacResourcesResponse {
	resources: string[];
	operations: string[];
}

// ─── Permission check ──────────────────────────────────────────────────────────

export interface RbacCheckResponse {
	allowed: boolean;
	user_id: string;
	resource: string;
	operation: string;
}

// ─── Form state ────────────────────────────────────────────────────────────────

export interface RbacRoleFormData {
	id?: string;
	name: string;
	description: string;
	is_default: boolean;
	permissions: RbacPermissionInput[];
}

export const DEFAULT_RBAC_ROLE_FORM_DATA: RbacRoleFormData = {
	name: "",
	description: "",
	is_default: false,
	permissions: [],
};

// ─── Constants ─────────────────────────────────────────────────────────────────

export const RBAC_RESOURCES = [
	"GuardrailsConfig",
	"GuardrailsProviders",
	"GuardrailRules",
	"UserProvisioning",
	"Cluster",
	"Settings",
	"Users",
	"Logs",
	"Observability",
	"VirtualKeys",
	"ModelProvider",
	"Plugins",
	"MCPGateway",
	"AdaptiveRouter",
	"AuditLogs",
	"Customers",
	"Teams",
	"RBAC",
	"Governance",
	"RoutingRules",
	"PIIRedactor",
	"PromptRepository",
	"PromptDeploymentStrategy",
] as const;

export const RBAC_OPERATIONS = ["Read", "View", "Create", "Update", "Delete", "Download"] as const;

export type RbacResourceName = (typeof RBAC_RESOURCES)[number];
export type RbacOperationName = (typeof RBAC_OPERATIONS)[number];
