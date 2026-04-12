/**
 * RBAC RTK Query API
 * Handles all API communication for RBAC roles, permissions, and user assignments
 */

import {
	RbacRole,
	RbacUserRole,
	GetRbacRolesResponse,
	GetRbacUserRolesResponse,
	GetRbacResourcesResponse,
	RbacCheckResponse,
	CreateRbacRoleRequest,
	UpdateRbacRoleRequest,
	SetUserRoleRequest,
} from "@/lib/types/rbac";
import { baseApi } from "./baseApi";

export const rbacApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		// ─── Roles ──────────────────────────────────────────────────────────────

		getRbacRoles: builder.query<GetRbacRolesResponse, void>({
			query: () => ({
				url: "/rbac/roles",
				method: "GET",
			}),
			providesTags: ["Roles"],
		}),

		getRbacRole: builder.query<RbacRole, string>({
			query: (id) => ({
				url: `/rbac/roles/${id}`,
				method: "GET",
			}),
			transformResponse: (response: { role: RbacRole }) => response.role,
			providesTags: (result, error, arg) => [{ type: "Roles", id: arg }],
		}),

		createRbacRole: builder.mutation<RbacRole, CreateRbacRoleRequest>({
			query: (body) => ({
				url: "/rbac/roles",
				method: "POST",
				body,
			}),
			transformResponse: (response: { role: RbacRole }) => response.role,
			async onQueryStarted(arg, { dispatch, queryFulfilled }) {
				try {
					const { data: newRole } = await queryFulfilled;
					dispatch(
						rbacApi.util.updateQueryData("getRbacRoles", undefined, (draft) => {
							if (!draft.roles) draft.roles = [];
							draft.roles.push(newRole);
							draft.count = (draft.count || 0) + 1;
						}),
					);
					dispatch(rbacApi.util.updateQueryData("getRbacRole", newRole.id, () => newRole));
				} catch {}
			},
		}),

		updateRbacRole: builder.mutation<RbacRole, { id: string; data: UpdateRbacRoleRequest }>({
			query: ({ id, data }) => ({
				url: `/rbac/roles/${id}`,
				method: "PUT",
				body: data,
			}),
			transformResponse: (response: { role: RbacRole }) => response.role,
			async onQueryStarted({ id }, { dispatch, queryFulfilled }) {
				try {
					const { data: updatedRole } = await queryFulfilled;
					dispatch(
						rbacApi.util.updateQueryData("getRbacRoles", undefined, (draft) => {
							if (!draft.roles) return;
							const index = draft.roles.findIndex((r) => r.id === id);
							if (index !== -1) {
								draft.roles[index] = updatedRole;
							}
						}),
					);
					dispatch(rbacApi.util.updateQueryData("getRbacRole", updatedRole.id, () => updatedRole));
				} catch {}
			},
		}),

		deleteRbacRole: builder.mutation<void, string>({
			query: (id) => ({
				url: `/rbac/roles/${id}`,
				method: "DELETE",
			}),
			async onQueryStarted(roleId, { dispatch, queryFulfilled }) {
				try {
					await queryFulfilled;
					dispatch(
						rbacApi.util.updateQueryData("getRbacRoles", undefined, (draft) => {
							if (!draft.roles) return;
							draft.roles = draft.roles.filter((r) => r.id !== roleId);
							draft.count = Math.max(0, (draft.count || 0) - 1);
						}),
					);
				} catch {}
			},
		}),

		// ─── User-Role assignments ───────────────────────────────────────────────

		getRbacUserRoles: builder.query<GetRbacUserRolesResponse, string>({
			query: (userId) => ({
				url: "/rbac/users",
				params: { user_id: userId },
				method: "GET",
			}),
			providesTags: (result, error, arg) => [{ type: "Users", id: `rbac-${arg}` }],
		}),

		setRbacUserRole: builder.mutation<{ message: string; user_id: string; role_id: string }, { userId: string; data: SetUserRoleRequest }>({
			query: ({ userId, data }) => ({
				url: `/rbac/users/${userId}`,
				method: "PUT",
				body: data,
			}),
			async onQueryStarted({ userId }, { dispatch, queryFulfilled }) {
				try {
					const { data: result } = await queryFulfilled;
					dispatch(
						rbacApi.util.updateQueryData("getRbacUserRoles", userId, (draft) => {
							draft.user_roles = [{ id: 0, user_id: userId, role_id: result.role_id }];
						}),
					);
				} catch {}
			},
		}),

		// ─── Metadata ────────────────────────────────────────────────────────────

		getRbacResources: builder.query<GetRbacResourcesResponse, void>({
			query: () => ({
				url: "/rbac/resources",
				method: "GET",
			}),
			providesTags: ["Resources"],
		}),

		checkRbacPermission: builder.query<RbacCheckResponse, { userId: string; resource: string; operation: string }>({
			query: ({ userId, resource, operation }) => ({
				url: "/rbac/check",
				params: { user_id: userId, resource, operation },
				method: "GET",
			}),
		}),
	}),
});

export const {
	useGetRbacRolesQuery,
	useGetRbacRoleQuery,
	useCreateRbacRoleMutation,
	useUpdateRbacRoleMutation,
	useDeleteRbacRoleMutation,
	useGetRbacUserRolesQuery,
	useSetRbacUserRoleMutation,
	useGetRbacResourcesQuery,
	useCheckRbacPermissionQuery,
	useLazyGetRbacRolesQuery,
} = rbacApi;
