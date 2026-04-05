import type {
	AdaptiveRoutingStatusResponse,
	AlertsResponse,
	AuditCategory,
	AuditSearchResult,
	ClusterStatus,
	CreateLogExportRequest,
	ExportJob,
	LogExportsResponse,
	VaultStatus,
} from "@/lib/types/enterprise";
import { baseApi } from "./baseApi";

export interface GetAdaptiveRoutingStatusArgs {
	provider?: string;
	model?: string;
	cluster?: boolean;
}

export interface GetAlertsArgs {
	cluster?: boolean;
}

export interface GetAuditLogsArgs {
	category?: AuditCategory | "";
	action?: string;
	resource_type?: string;
	actor_id?: string;
	start_time?: string;
	end_time?: string;
	limit?: number;
	offset?: number;
}

function compactParams<T extends Record<string, string | number | undefined>>(params: T) {
	return Object.fromEntries(Object.entries(params).filter(([, value]) => value !== undefined && value !== "")) as Record<
		string,
		string | number
	>;
}

export const enterpriseApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		getClusterStatus: builder.query<ClusterStatus, void>({
			query: () => ({
				url: "/cluster/status",
			}),
			providesTags: ["ClusterNodes"],
		}),
		getAdaptiveRoutingStatus: builder.query<AdaptiveRoutingStatusResponse, GetAdaptiveRoutingStatusArgs | void>({
			query: (args) => ({
				url: "/adaptive-routing/status",
				params: compactParams({
					provider: args?.provider,
					model: args?.model,
					cluster: args?.cluster ? "true" : undefined,
				}),
			}),
			providesTags: ["AdaptiveRouting"],
		}),
		getAuditLogs: builder.query<AuditSearchResult, GetAuditLogsArgs | void>({
			query: (args) => ({
				url: "/audit-logs",
				params: compactParams({
					category: args?.category,
					action: args?.action,
					resource_type: args?.resource_type,
					actor_id: args?.actor_id,
					start_time: args?.start_time,
					end_time: args?.end_time,
					limit: args?.limit,
					offset: args?.offset,
				}),
			}),
			providesTags: ["AuditLogs"],
		}),
		getAlerts: builder.query<AlertsResponse, GetAlertsArgs | void>({
			query: (args) => ({
				url: "/alerts",
				params: compactParams({
					cluster: args?.cluster ? "true" : undefined,
				}),
			}),
			providesTags: ["Alerts"],
		}),
		getVaultStatus: builder.query<VaultStatus, void>({
			query: () => ({
				url: "/vault/status",
			}),
			providesTags: ["VaultStatus"],
		}),
		getLogExports: builder.query<LogExportsResponse, void>({
			query: () => ({
				url: "/log-exports",
			}),
			providesTags: ["LogExports"],
		}),
		createLogExport: builder.mutation<ExportJob, CreateLogExportRequest | void>({
			query: (body) => ({
				url: "/logs/exports",
				method: "POST",
				body,
			}),
			invalidatesTags: ["LogExports"],
		}),
		createMCPLogExport: builder.mutation<ExportJob, CreateLogExportRequest | void>({
			query: (body) => ({
				url: "/mcp-logs/exports",
				method: "POST",
				body,
			}),
			invalidatesTags: ["LogExports"],
		}),
	}),
});

export const {
	useCreateLogExportMutation,
	useCreateMCPLogExportMutation,
	useGetAdaptiveRoutingStatusQuery,
	useGetAlertsQuery,
	useGetAuditLogsQuery,
	useGetClusterStatusQuery,
	useGetLogExportsQuery,
	useGetVaultStatusQuery,
} = enterpriseApi;
