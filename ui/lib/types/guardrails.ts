/**
 * Guardrails Type Definitions
 * Defines all TypeScript interfaces for guardrails feature (providers and rules)
 */

import { RuleGroupType } from "react-querybuilder";

// ─── Provider Types ────────────────────────────────────────────────────────────

export type GuardrailProviderType =
	| "bedrock"
	| "azure_content_moderation"
	| "patronus"
	| "mistral_moderation";

export interface GuardrailProvider {
	id: string;
	name: string;
	provider_type: GuardrailProviderType;
	enabled: boolean;
	timeout_seconds: number;
	config: Record<string, any>;
	created_at: string;
	updated_at: string;
}

export interface CreateGuardrailProviderRequest {
	name: string;
	provider_type: GuardrailProviderType;
	enabled?: boolean;
	timeout_seconds?: number;
	config: Record<string, any>;
}

export type UpdateGuardrailProviderRequest = Partial<CreateGuardrailProviderRequest>;

export interface GetGuardrailProvidersResponse {
	providers: GuardrailProvider[];
}

// ─── Rule Types ────────────────────────────────────────────────────────────────

export type GuardrailApplyOn = "input" | "output" | "both";
export type GuardrailScope = "global" | "team" | "customer" | "virtual_key";

export interface GuardrailRule {
	id: string;
	name: string;
	description: string;
	enabled: boolean;
	apply_on: GuardrailApplyOn;
	profile_ids: string[];
	sampling_rate: number;
	timeout_seconds: number;
	cel_expression: string;
	query?: RuleGroupType;
	scope: GuardrailScope;
	scope_id?: string;
	priority: number;
	created_at: string;
	updated_at: string;
}

export interface CreateGuardrailRuleRequest {
	name: string;
	description?: string;
	enabled?: boolean;
	apply_on: GuardrailApplyOn;
	profile_ids: string[];
	sampling_rate?: number;
	timeout_seconds?: number;
	cel_expression?: string;
	query?: RuleGroupType;
	scope: GuardrailScope;
	scope_id?: string;
	priority?: number;
}

export type UpdateGuardrailRuleRequest = Partial<CreateGuardrailRuleRequest>;

export interface GetGuardrailRulesResponse {
	rules: GuardrailRule[];
}

// ─── Form Data ─────────────────────────────────────────────────────────────────

export interface GuardrailRuleFormData {
	id?: string;
	name: string;
	description: string;
	enabled: boolean;
	apply_on: GuardrailApplyOn;
	profile_ids: string[];
	sampling_rate: number;
	timeout_seconds: number;
	cel_expression: string;
	query?: RuleGroupType;
	scope: GuardrailScope;
	scope_id: string;
	priority: number;
}

export const DEFAULT_GUARDRAIL_RULE_FORM_DATA: GuardrailRuleFormData = {
	name: "",
	description: "",
	enabled: true,
	apply_on: "both",
	profile_ids: [],
	sampling_rate: 100,
	timeout_seconds: 60,
	cel_expression: "",
	scope: "global",
	scope_id: "",
	priority: 0,
};

export interface GuardrailProviderFormData {
	id?: string;
	name: string;
	provider_type: GuardrailProviderType;
	enabled: boolean;
	timeout_seconds: number;
	config: Record<string, any>;
}

export const DEFAULT_GUARDRAIL_PROVIDER_FORM_DATA: GuardrailProviderFormData = {
	name: "",
	provider_type: "bedrock",
	enabled: true,
	timeout_seconds: 30,
	config: {},
};

// ─── Provider Display Metadata ─────────────────────────────────────────────────

export interface GuardrailProviderMeta {
	type: GuardrailProviderType;
	label: string;
	description: string;
}

export const GUARDRAIL_PROVIDER_META: GuardrailProviderMeta[] = [
	{
		type: "bedrock",
		label: "AWS Bedrock",
		description: "AWS Bedrock Guardrails for content moderation",
	},
	{
		type: "azure_content_moderation",
		label: "Azure Content Moderation",
		description: "Azure AI Content Safety service",
	},
	{
		type: "patronus",
		label: "Patronus AI",
		description: "Patronus AI evaluation and safety platform",
	},
	{
		type: "mistral_moderation",
		label: "Mistral Moderation",
		description: "Mistral AI content moderation model",
	},
];

export const GUARDRAIL_APPLY_ON_OPTIONS: { value: GuardrailApplyOn; label: string }[] = [
	{ value: "input", label: "Input Only" },
	{ value: "output", label: "Output Only" },
	{ value: "both", label: "Both" },
];
