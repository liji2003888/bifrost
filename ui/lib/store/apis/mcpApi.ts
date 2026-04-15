import {
	CreateMCPClientRequest,
	CreateMCPHostedToolRequest,
	GetMCPAuthConfigsParams,
	GetMCPAuthConfigsResponse,
	GetMCPClientsParams,
	GetMCPClientsResponse,
	GetMCPHostedToolsResponse,
	MCPClient,
	OAuthFlowResponse,
	OAuthStatusResponse,
	PreviewMCPHostedToolRequest,
	PreviewMCPHostedToolResponse,
	UpdateMCPHostedToolRequest,
	UpdateMCPClientRequest,
	ValidateMCPClientRequest,
	ValidateMCPClientResponse,
} from "@/lib/types/mcp";
import { baseApi } from "./baseApi";

type CreateMCPClientResponse = { status: "success"; message: string } | OAuthFlowResponse;

export const mcpApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		// Get MCP clients with pagination
		getMCPClients: builder.query<GetMCPClientsResponse, GetMCPClientsParams | void>({
			query: (params) => ({
				url: "/mcp/clients",
				params: {
					...(params?.limit && { limit: params.limit }),
					...(params?.offset !== undefined && { offset: params.offset }),
					...(params?.search && { search: params.search }),
				},
			}),
			providesTags: ["MCPClients"],
		}),

		getMCPHostedTools: builder.query<GetMCPHostedToolsResponse, void>({
			query: () => ({
				url: "/mcp/hosted-tools",
			}),
			providesTags: ["MCPHostedTools"],
		}),

		// Create new MCP client
		createMCPClient: builder.mutation<CreateMCPClientResponse, CreateMCPClientRequest>({
			query: (data) => ({
				url: "/mcp/client",
				method: "POST",
				body: data,
			}),
			async onQueryStarted(arg, { dispatch, getState, queryFulfilled }) {
				try {
					await queryFulfilled;
					// MCP create may return an OAuth flow response, so we can't optimistically
					// add the client — just invalidate to refetch
					const queries = (getState() as any).api.queries;
					for (const entry of Object.values(queries) as any[]) {
						if (entry?.endpointName !== "getMCPClients" || entry?.status !== "fulfilled") continue;
						dispatch(mcpApi.util.invalidateTags(["MCPClients"]));
						break;
					}
				} catch {}
			},
		}),

		validateMCPClient: builder.mutation<ValidateMCPClientResponse, ValidateMCPClientRequest>({
			query: (data) => ({
				url: "/mcp/client/validate",
				method: "POST",
				body: data,
			}),
		}),

		createMCPHostedTool: builder.mutation<{ status: "success"; message: string }, CreateMCPHostedToolRequest>({
			query: (data) => ({
				url: "/mcp/hosted-tool",
				method: "POST",
				body: data,
			}),
			invalidatesTags: ["MCPHostedTools"],
		}),

		updateMCPHostedTool: builder.mutation<{ status: "success"; message: string }, { id: string; data: UpdateMCPHostedToolRequest }>({
			query: ({ id, data }) => ({
				url: `/mcp/hosted-tool/${id}`,
				method: "PUT",
				body: data,
			}),
			invalidatesTags: ["MCPHostedTools"],
		}),

		previewMCPHostedTool: builder.mutation<PreviewMCPHostedToolResponse, { id: string; data?: PreviewMCPHostedToolRequest }>({
			query: ({ id, data }) => ({
				url: `/mcp/hosted-tool/${id}/preview`,
				method: "POST",
				body: data,
			}),
		}),

		// Update existing MCP client
		updateMCPClient: builder.mutation<any, { id: string; data: UpdateMCPClientRequest }>({
			query: ({ id, data }) => ({
				url: `/mcp/client/${id}`,
				method: "PUT",
				body: data,
			}),
			async onQueryStarted({ id, data }, { dispatch, getState, queryFulfilled }) {
				try {
					await queryFulfilled;
					const queries = (getState() as any).api.queries;
					for (const entry of Object.values(queries) as any[]) {
						if (entry?.endpointName !== "getMCPClients" || entry?.status !== "fulfilled") continue;
						dispatch(
							mcpApi.util.updateQueryData("getMCPClients", entry.originalArgs, (draft) => {
								if (!draft.clients) return;
								const index = draft.clients.findIndex((c) => c.config.client_id === id);
								if (index !== -1) {
									// Merge the updated fields into the existing client
									if (data.name !== undefined) draft.clients[index].config.name = data.name;
									if (data.is_code_mode_client !== undefined) draft.clients[index].config.is_code_mode_client = data.is_code_mode_client;
									if (data.headers !== undefined) draft.clients[index].config.headers = data.headers;
									if (data.tools_to_execute !== undefined) draft.clients[index].config.tools_to_execute = data.tools_to_execute;
									if (data.tools_to_auto_execute !== undefined) draft.clients[index].config.tools_to_auto_execute = data.tools_to_auto_execute;
									if (data.is_ping_available !== undefined) draft.clients[index].config.is_ping_available = data.is_ping_available;
									if (data.tool_pricing !== undefined) draft.clients[index].config.tool_pricing = data.tool_pricing;
									if (data.tool_sync_interval !== undefined) draft.clients[index].config.tool_sync_interval = data.tool_sync_interval;
								}
							}),
						);
					}
				} catch {}
			},
		}),

		// Delete MCP client
		deleteMCPClient: builder.mutation<any, string>({
			query: (id) => ({
				url: `/mcp/client/${id}`,
				method: "DELETE",
			}),
			async onQueryStarted(id, { dispatch, getState, queryFulfilled }) {
				try {
					await queryFulfilled;
					const queries = (getState() as any).api.queries;
					for (const entry of Object.values(queries) as any[]) {
						if (entry?.endpointName !== "getMCPClients" || entry?.status !== "fulfilled") continue;
						dispatch(
							mcpApi.util.updateQueryData("getMCPClients", entry.originalArgs, (draft) => {
								if (!draft.clients) return;
								const before = draft.clients.length;
								draft.clients = draft.clients.filter((c) => c.config.client_id !== id);
								if (draft.clients.length < before) {
									draft.count = Math.max(0, (draft.count || 0) - 1);
									draft.total_count = Math.max(0, (draft.total_count || 0) - 1);
								}
							}),
						);
					}
				} catch {}
			},
		}),

		deleteMCPHostedTool: builder.mutation<any, string>({
			query: (id) => ({
				url: `/mcp/hosted-tool/${id}`,
				method: "DELETE",
			}),
			invalidatesTags: ["MCPHostedTools"],
		}),

		// Reconnect MCP client
		reconnectMCPClient: builder.mutation<any, string>({
			query: (id) => ({
				url: `/mcp/client/${id}/reconnect`,
				method: "POST",
			}),
			invalidatesTags: ["MCPClients"],
		}),

		// Get OAuth config status (for polling)
		getOAuthConfigStatus: builder.query<OAuthStatusResponse, string>({
			query: (oauthConfigId) => `/oauth/config/${oauthConfigId}/status`,
			providesTags: (result, error, id) => [{ type: "OAuth2Config", id }],
		}),

		getMCPAuthConfigs: builder.query<GetMCPAuthConfigsResponse, GetMCPAuthConfigsParams | void>({
			query: (params) => ({
				url: "/oauth/configs",
				params: {
					...(params?.limit && { limit: params.limit }),
					...(params?.offset !== undefined && { offset: params.offset }),
					...(params?.search && { search: params.search }),
					...(params?.status && params.status !== "all" && { status: params.status }),
				},
			}),
			providesTags: (result) => {
				const tags: ({ type: "OAuth2Config"; id: string } | "OAuth2Config")[] = ["OAuth2Config"];
				for (const config of result?.configs ?? []) {
					tags.push({ type: "OAuth2Config", id: config.id });
				}
				return tags;
			},
		}),

		revokeOAuthConfig: builder.mutation<{ message: string }, string>({
			query: (oauthConfigId) => ({
				url: `/oauth/config/${oauthConfigId}`,
				method: "DELETE",
			}),
			invalidatesTags: (result, error, id) => ["OAuth2Config", "MCPClients", { type: "OAuth2Config", id }],
		}),

		// Complete OAuth flow for MCP client
		completeOAuthFlow: builder.mutation<{ status: string; message: string }, string>({
			query: (oauthConfigId) => ({
				url: `/mcp/client/${oauthConfigId}/complete-oauth`,
				method: "POST",
			}),
			invalidatesTags: (result, error, id) => ["MCPClients", "OAuth2Config", { type: "OAuth2Config", id }],
		}),
	}),
});

export const {
	useGetMCPClientsQuery,
	useGetMCPHostedToolsQuery,
	useCreateMCPClientMutation,
	useCreateMCPHostedToolMutation,
	useUpdateMCPHostedToolMutation,
	usePreviewMCPHostedToolMutation,
	useValidateMCPClientMutation,
	useUpdateMCPClientMutation,
	useDeleteMCPClientMutation,
	useDeleteMCPHostedToolMutation,
	useReconnectMCPClientMutation,
	useLazyGetMCPClientsQuery,
	useLazyGetOAuthConfigStatusQuery,
	useGetMCPAuthConfigsQuery,
	useRevokeOAuthConfigMutation,
	useCompleteOAuthFlowMutation,
} = mcpApi;
