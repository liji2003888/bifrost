/**
 * Guardrails RTK Query API
 * Handles all API communication for guardrail providers and rules CRUD operations
 */

import {
	GuardrailProvider,
	GuardrailRule,
	GetGuardrailProvidersResponse,
	GetGuardrailRulesResponse,
	CreateGuardrailProviderRequest,
	UpdateGuardrailProviderRequest,
	CreateGuardrailRuleRequest,
	UpdateGuardrailRuleRequest,
} from "@/lib/types/guardrails";
import { baseApi } from "./baseApi";

export const guardrailsApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		// ─── Providers ──────────────────────────────────────────────────────────

		getGuardrailProviders: builder.query<GetGuardrailProvidersResponse, void>({
			query: () => ({
				url: "/guardrails/providers",
				method: "GET",
			}),
			providesTags: ["GuardrailProviders"],
		}),

		getGuardrailProvider: builder.query<GuardrailProvider, string>({
			query: (id) => ({
				url: `/guardrails/providers/${id}`,
				method: "GET",
			}),
			providesTags: (result, error, arg) => [{ type: "GuardrailProviders", id: arg }],
		}),

		createGuardrailProvider: builder.mutation<GuardrailProvider, CreateGuardrailProviderRequest>({
			query: (body) => ({
				url: "/guardrails/providers",
				method: "POST",
				body,
			}),
			async onQueryStarted(arg, { dispatch, queryFulfilled }) {
				try {
					const { data: newProvider } = await queryFulfilled;
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailProviders", undefined, (draft) => {
							if (!draft.providers) draft.providers = [];
							draft.providers.unshift(newProvider);
						}),
					);
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailProvider", newProvider.id, () => newProvider),
					);
				} catch {}
			},
		}),

		updateGuardrailProvider: builder.mutation<GuardrailProvider, { id: string; data: UpdateGuardrailProviderRequest }>({
			query: ({ id, data }) => ({
				url: `/guardrails/providers/${id}`,
				method: "PUT",
				body: data,
			}),
			async onQueryStarted({ id }, { dispatch, queryFulfilled }) {
				try {
					const { data: updatedProvider } = await queryFulfilled;
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailProviders", undefined, (draft) => {
							if (!draft.providers) return;
							const index = draft.providers.findIndex((p) => p.id === id);
							if (index !== -1) {
								draft.providers[index] = updatedProvider;
							}
						}),
					);
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailProvider", updatedProvider.id, () => updatedProvider),
					);
				} catch {}
			},
		}),

		deleteGuardrailProvider: builder.mutation<void, string>({
			query: (id) => ({
				url: `/guardrails/providers/${id}`,
				method: "DELETE",
			}),
			async onQueryStarted(providerId, { dispatch, queryFulfilled }) {
				try {
					await queryFulfilled;
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailProviders", undefined, (draft) => {
							if (!draft.providers) return;
							draft.providers = draft.providers.filter((p) => p.id !== providerId);
						}),
					);
				} catch {}
			},
		}),

		// ─── Rules ──────────────────────────────────────────────────────────────

		getGuardrailRules: builder.query<GetGuardrailRulesResponse, void>({
			query: () => ({
				url: "/guardrails/rules",
				method: "GET",
			}),
			providesTags: ["GuardrailRules"],
		}),

		getGuardrailRule: builder.query<GuardrailRule, string>({
			query: (id) => ({
				url: `/guardrails/rules/${id}`,
				method: "GET",
			}),
			providesTags: (result, error, arg) => [{ type: "GuardrailRules", id: arg }],
		}),

		createGuardrailRule: builder.mutation<GuardrailRule, CreateGuardrailRuleRequest>({
			query: (body) => ({
				url: "/guardrails/rules",
				method: "POST",
				body,
			}),
			async onQueryStarted(arg, { dispatch, queryFulfilled }) {
				try {
					const { data: newRule } = await queryFulfilled;
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailRules", undefined, (draft) => {
							if (!draft.rules) draft.rules = [];
							draft.rules.unshift(newRule);
						}),
					);
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailRule", newRule.id, () => newRule),
					);
				} catch {}
			},
		}),

		updateGuardrailRule: builder.mutation<GuardrailRule, { id: string; data: UpdateGuardrailRuleRequest }>({
			query: ({ id, data }) => ({
				url: `/guardrails/rules/${id}`,
				method: "PUT",
				body: data,
			}),
			async onQueryStarted({ id }, { dispatch, queryFulfilled }) {
				try {
					const { data: updatedRule } = await queryFulfilled;
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailRules", undefined, (draft) => {
							if (!draft.rules) return;
							const index = draft.rules.findIndex((r) => r.id === id);
							if (index !== -1) {
								draft.rules[index] = updatedRule;
							}
						}),
					);
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailRule", updatedRule.id, () => updatedRule),
					);
				} catch {}
			},
		}),

		deleteGuardrailRule: builder.mutation<void, string>({
			query: (id) => ({
				url: `/guardrails/rules/${id}`,
				method: "DELETE",
			}),
			async onQueryStarted(ruleId, { dispatch, queryFulfilled }) {
				try {
					await queryFulfilled;
					dispatch(
						guardrailsApi.util.updateQueryData("getGuardrailRules", undefined, (draft) => {
							if (!draft.rules) return;
							draft.rules = draft.rules.filter((r) => r.id !== ruleId);
						}),
					);
				} catch {}
			},
		}),
	}),
});

export const {
	// Provider hooks
	useGetGuardrailProvidersQuery,
	useGetGuardrailProviderQuery,
	useCreateGuardrailProviderMutation,
	useUpdateGuardrailProviderMutation,
	useDeleteGuardrailProviderMutation,
	// Rule hooks
	useGetGuardrailRulesQuery,
	useGetGuardrailRuleQuery,
	useCreateGuardrailRuleMutation,
	useUpdateGuardrailRuleMutation,
	useDeleteGuardrailRuleMutation,
} = guardrailsApi;
