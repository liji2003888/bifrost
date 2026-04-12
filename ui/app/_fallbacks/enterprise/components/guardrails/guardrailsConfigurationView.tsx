/**
 * Guardrails Rules Configuration View
 * Manages guardrail rules with full CRUD operations
 */

"use client";

import { useState, useEffect, useCallback } from "react";
import { toast } from "sonner";
import dynamic from "next/dynamic";
import { RuleGroupType } from "react-querybuilder";
import { Plus, Edit, Trash2, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { MultiSelect } from "@/components/ui/multiSelect";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
} from "@/components/ui/sheet";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alertDialog";

import {
	useGetGuardrailRulesQuery,
	useCreateGuardrailRuleMutation,
	useUpdateGuardrailRuleMutation,
	useDeleteGuardrailRuleMutation,
	useGetGuardrailProvidersQuery,
} from "@/lib/store/apis/guardrailsApi";
import { getErrorMessage } from "@/lib/store";
import {
	GuardrailRule,
	GuardrailRuleFormData,
	GuardrailApplyOn,
	DEFAULT_GUARDRAIL_RULE_FORM_DATA,
	GUARDRAIL_APPLY_ON_OPTIONS,
} from "@/lib/types/guardrails";

// Dynamically import CEL builder to avoid SSR issues
const CELRuleBuilder = dynamic(
	() =>
		import("@/app/workspace/routing-rules/components/celBuilder/celRuleBuilder").then((mod) => ({
			default: mod.CELRuleBuilder,
		})),
	{
		loading: () => (
			<div className="flex items-center gap-2 rounded-md border p-4 text-sm text-muted-foreground">
				<Loader2 className="h-4 w-4 animate-spin" />
				Loading CEL builder...
			</div>
		),
		ssr: false,
	},
);

const defaultQuery: RuleGroupType = { combinator: "and", rules: [] };

// ─── Rule Sheet ────────────────────────────────────────────────────────────────

interface GuardrailRuleSheetProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	editingRule?: GuardrailRule | null;
}

function GuardrailRuleSheet({ open, onOpenChange, editingRule }: GuardrailRuleSheetProps) {
	const isEditing = !!editingRule;

	const { data: providersData } = useGetGuardrailProvidersQuery();
	const providers = providersData?.providers ?? [];

	const [createRule, { isLoading: isCreating }] = useCreateGuardrailRuleMutation();
	const [updateRule, { isLoading: isUpdating }] = useUpdateGuardrailRuleMutation();
	const isLoading = isCreating || isUpdating;

	// Form state
	const [formData, setFormData] = useState<GuardrailRuleFormData>({ ...DEFAULT_GUARDRAIL_RULE_FORM_DATA });
	const [query, setQuery] = useState<RuleGroupType>(defaultQuery);
	const [celExpression, setCelExpression] = useState("");
	const [builderKey, setBuilderKey] = useState(0);
	const [nameError, setNameError] = useState("");

	// Reset form when sheet opens/closes or editing rule changes
	useEffect(() => {
		if (open) {
			if (editingRule) {
				setFormData({
					id: editingRule.id,
					name: editingRule.name,
					description: editingRule.description ?? "",
					enabled: editingRule.enabled,
					apply_on: editingRule.apply_on,
					profile_ids: editingRule.profile_ids ?? [],
					sampling_rate: editingRule.sampling_rate ?? 100,
					timeout_seconds: editingRule.timeout_seconds ?? 60,
					cel_expression: editingRule.cel_expression ?? "",
					query: editingRule.query,
					scope: editingRule.scope,
					scope_id: editingRule.scope_id ?? "",
					priority: editingRule.priority ?? 0,
				});
				setQuery(editingRule.query ?? defaultQuery);
				setCelExpression(editingRule.cel_expression ?? "");
			} else {
				setFormData({ ...DEFAULT_GUARDRAIL_RULE_FORM_DATA });
				setQuery(defaultQuery);
				setCelExpression("");
			}
			setNameError("");
			setBuilderKey((k) => k + 1);
		}
	}, [open, editingRule]);

	const handleQueryChange = useCallback((expression: string, newQuery: RuleGroupType) => {
		setCelExpression(expression);
		setQuery(newQuery);
		setFormData((prev) => ({ ...prev, cel_expression: expression }));
	}, []);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();

		if (!formData.name.trim()) {
			setNameError("Rule name is required");
			return;
		}
		setNameError("");

		const payload = {
			name: formData.name.trim(),
			description: formData.description,
			enabled: formData.enabled,
			apply_on: formData.apply_on,
			profile_ids: formData.profile_ids,
			sampling_rate: formData.sampling_rate,
			timeout_seconds: formData.timeout_seconds,
			cel_expression: celExpression,
			query: query,
			scope: formData.scope,
			scope_id: formData.scope_id || undefined,
			priority: formData.priority,
		};

		try {
			if (isEditing && editingRule) {
				await updateRule({ id: editingRule.id, data: payload }).unwrap();
				toast.success("Guardrail rule updated successfully");
			} else {
				await createRule(payload).unwrap();
				toast.success("Guardrail rule created successfully");
			}
			onOpenChange(false);
		} catch (error: any) {
			toast.error(getErrorMessage(error));
		}
	};

	const handleCancel = () => {
		onOpenChange(false);
	};

	const providerOptions = providers.map((p) => ({
		label: p.name,
		value: p.id,
		description: p.provider_type,
	}));

	return (
		<Sheet open={open} onOpenChange={onOpenChange}>
			<SheetContent className="flex w-full flex-col min-w-1/2 gap-0 overflow-x-hidden p-0">
				<SheetHeader className="border-b p-6">
					<SheetTitle>{isEditing ? "Edit Guardrail Rule" : "Add New Rule"}</SheetTitle>
					<SheetDescription>
						{isEditing
							? "Update the guardrail rule configuration."
							: "Configure a new guardrail rule to control when guardrails are executed."}
					</SheetDescription>
				</SheetHeader>

				<form onSubmit={handleSubmit} className="flex flex-col flex-1 overflow-hidden">
					<div className="flex-1 overflow-y-auto p-6 space-y-6">
						{/* Rule Name */}
						<div className="space-y-2">
							<Label htmlFor="rule-name">
								Rule Name <span className="text-destructive">*</span>
							</Label>
							<Input
								id="rule-name"
								placeholder="e.g., Block harmful content"
								value={formData.name}
								onChange={(e) => {
									setFormData((prev) => ({ ...prev, name: e.target.value }));
									if (e.target.value.trim()) setNameError("");
								}}
							/>
							{nameError && <p className="text-destructive text-sm">{nameError}</p>}
						</div>

						{/* Description */}
						<div className="space-y-2">
							<Label htmlFor="rule-description">Description</Label>
							<Textarea
								id="rule-description"
								placeholder="Describe what this guardrail rule does..."
								rows={2}
								value={formData.description}
								onChange={(e) => setFormData((prev) => ({ ...prev, description: e.target.value }))}
							/>
						</div>

						{/* Enable Rule Toggle */}
						<div className="flex items-center justify-between rounded-lg border p-4">
							<div className="space-y-0.5">
								<Label htmlFor="rule-enabled">Enable Rule</Label>
								<p className="text-muted-foreground text-sm">
									Rule will be active and applied to matching requests
								</p>
							</div>
							<Switch
								id="rule-enabled"
								checked={formData.enabled}
								onCheckedChange={(checked) =>
									setFormData((prev) => ({ ...prev, enabled: checked }))
								}
							/>
						</div>

						{/* Apply On */}
						<div className="space-y-3">
							<Label>Apply On</Label>
							<div className="flex gap-3">
								{GUARDRAIL_APPLY_ON_OPTIONS.map((opt) => (
									<label
										key={opt.value}
										className={`flex flex-1 cursor-pointer items-center justify-center gap-2 rounded-md border px-4 py-2.5 text-sm font-medium transition-colors ${
											formData.apply_on === opt.value
												? "border-primary bg-primary/5 text-primary"
												: "border-border text-muted-foreground hover:border-primary/50 hover:text-foreground"
										}`}
									>
										<input
											type="radio"
											name="apply_on"
											value={opt.value}
											checked={formData.apply_on === opt.value}
											onChange={() =>
												setFormData((prev) => ({
													...prev,
													apply_on: opt.value as GuardrailApplyOn,
												}))
											}
											className="sr-only"
										/>
										{opt.label}
									</label>
								))}
							</div>
						</div>

						{/* Guardrail Profiles */}
						<div className="space-y-2">
							<Label>Guardrail Profiles</Label>
							<p className="text-muted-foreground text-xs">
								Select which guardrail provider configurations to apply.
							</p>
							<MultiSelect
								options={providerOptions}
								defaultValue={formData.profile_ids}
								onValueChange={(values) =>
									setFormData((prev) => ({ ...prev, profile_ids: values }))
								}
								placeholder={
									providers.length === 0
										? "No providers configured"
										: "Select guardrail profiles..."
								}
								disabled={providers.length === 0}
								hideSelectAll={providerOptions.length <= 3}
								modalPopover
							/>
							{providers.length === 0 && (
								<p className="text-muted-foreground text-xs">
									Configure guardrail providers first in the Providers tab.
								</p>
							)}
						</div>

						{/* Sampling Rate & Timeout */}
						<div className="grid grid-cols-2 gap-4">
							<div className="space-y-2">
								<Label htmlFor="sampling-rate">Sampling Rate (%)</Label>
								<Input
									id="sampling-rate"
									type="number"
									min={0}
									max={100}
									value={formData.sampling_rate}
									onChange={(e) =>
										setFormData((prev) => ({
											...prev,
											sampling_rate: Math.min(100, Math.max(0, Number(e.target.value))),
										}))
									}
								/>
								<p className="text-muted-foreground text-xs">
									Percentage of requests to apply guardrails to (0-100)
								</p>
							</div>
							<div className="space-y-2">
								<Label htmlFor="timeout-seconds">Timeout (Seconds)</Label>
								<Input
									id="timeout-seconds"
									type="number"
									min={1}
									value={formData.timeout_seconds}
									onChange={(e) =>
										setFormData((prev) => ({
											...prev,
											timeout_seconds: Math.max(1, Number(e.target.value)),
										}))
									}
								/>
								<p className="text-muted-foreground text-xs">
									Maximum time to wait for guardrail evaluation
								</p>
							</div>
						</div>

						{/* CEL Rule Builder */}
						<div className="space-y-3">
							<div>
								<Label>Rule Builder</Label>
								<p className="text-muted-foreground mt-1 text-xs">
									Build a CEL expression to define when this guardrail rule applies.
								</p>
							</div>
							<CELRuleBuilder
								key={builderKey}
								onChange={handleQueryChange}
								initialQuery={query}
							/>
						</div>
					</div>

					{/* Footer */}
					<div className="border-t p-6 flex items-center justify-end gap-3">
						<Button type="button" variant="outline" onClick={handleCancel} disabled={isLoading}>
							Cancel
						</Button>
						<Button type="submit" disabled={isLoading}>
							{isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
							{isEditing ? "Update Rule" : "Save Rule"}
						</Button>
					</div>
				</form>
			</SheetContent>
		</Sheet>
	);
}

// ─── Main View ─────────────────────────────────────────────────────────────────

export default function GuardrailsConfigurationView() {
	const [sheetOpen, setSheetOpen] = useState(false);
	const [editingRule, setEditingRule] = useState<GuardrailRule | null>(null);
	const [deleteRuleId, setDeleteRuleId] = useState<string | null>(null);

	const { data: rulesData, isLoading } = useGetGuardrailRulesQuery();
	const rules = rulesData?.rules ?? [];

	const [deleteRule, { isLoading: isDeleting }] = useDeleteGuardrailRuleMutation();

	const ruleToDelete = rules.find((r) => r.id === deleteRuleId);

	const handleCreateNew = () => {
		setEditingRule(null);
		setSheetOpen(true);
	};

	const handleEdit = (rule: GuardrailRule) => {
		setEditingRule(rule);
		setSheetOpen(true);
	};

	const handleSheetOpenChange = (open: boolean) => {
		setSheetOpen(open);
		if (!open) setEditingRule(null);
	};

	const handleDelete = async () => {
		if (!deleteRuleId) return;
		try {
			await deleteRule(deleteRuleId).unwrap();
			toast.success("Guardrail rule deleted successfully");
			setDeleteRuleId(null);
		} catch (error: any) {
			toast.error(getErrorMessage(error));
		}
	};

	const getApplyOnLabel = (applyOn: string) => {
		return GUARDRAIL_APPLY_ON_OPTIONS.find((o) => o.value === applyOn)?.label ?? applyOn;
	};

	return (
		<div className="space-y-4">
			{/* Header */}
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-foreground text-lg font-semibold">Guardrail Rules</h1>
					<p className="text-muted-foreground text-sm">
						Configure guardrail rules to control when to execute guardrails.
					</p>
				</div>
				<Button onClick={handleCreateNew} className="gap-2">
					<Plus className="h-4 w-4" />
					Add New Rule
				</Button>
			</div>

			{/* Table */}
			<div className="rounded-sm border overflow-hidden">
				<Table>
					<TableHeader>
						<TableRow className="bg-muted/50">
							<TableHead className="font-semibold">Rule Name</TableHead>
							<TableHead className="font-semibold">Description</TableHead>
							<TableHead className="font-semibold">Apply On</TableHead>
							<TableHead className="font-semibold">Sampling Rate</TableHead>
							<TableHead className="font-semibold">Status</TableHead>
							<TableHead className="text-right font-semibold">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading ? (
							[...Array(4)].map((_, i) => (
								<TableRow key={i}>
									<TableCell colSpan={6} className="h-10">
										<div className="h-2 w-48 animate-pulse rounded bg-muted" />
									</TableCell>
								</TableRow>
							))
						) : rules.length === 0 ? (
							<TableRow>
								<TableCell colSpan={6} className="h-32 text-center">
									<div className="flex flex-col items-center gap-2">
										<p className="text-muted-foreground text-sm">
											No guardrail rules configured yet.
										</p>
										<Button variant="outline" size="sm" onClick={handleCreateNew} className="gap-1">
											<Plus className="h-3.5 w-3.5" />
											Add your first rule
										</Button>
									</div>
								</TableCell>
							</TableRow>
						) : (
							rules.map((rule) => (
								<TableRow key={rule.id} className="hover:bg-muted/50 transition-colors">
									<TableCell className="font-medium">
										<span className="truncate block max-w-[180px]" title={rule.name}>
											{rule.name}
										</span>
									</TableCell>
									<TableCell>
										<span
											className="text-muted-foreground text-sm truncate block max-w-[220px]"
											title={rule.description}
										>
											{rule.description || <span className="italic">—</span>}
										</span>
									</TableCell>
									<TableCell>
										<Badge variant="secondary">{getApplyOnLabel(rule.apply_on)}</Badge>
									</TableCell>
									<TableCell>
										<span className="text-sm">{rule.sampling_rate ?? 100}%</span>
									</TableCell>
									<TableCell>
										<Badge variant={rule.enabled ? "default" : "secondary"}>
											{rule.enabled ? "Enabled" : "Disabled"}
										</Badge>
									</TableCell>
									<TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
										<div className="flex items-center justify-end gap-2">
											<Button
												variant="ghost"
												size="sm"
												onClick={() => handleEdit(rule)}
												aria-label="Edit guardrail rule"
											>
												<Edit className="h-4 w-4" />
											</Button>
											<Button
												variant="ghost"
												size="sm"
												className="text-destructive hover:bg-destructive/10 hover:text-destructive"
												onClick={() => setDeleteRuleId(rule.id)}
												aria-label="Delete guardrail rule"
											>
												<Trash2 className="h-4 w-4" />
											</Button>
										</div>
									</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</Table>
			</div>

			{/* Rule Sheet */}
			<GuardrailRuleSheet
				open={sheetOpen}
				onOpenChange={handleSheetOpenChange}
				editingRule={editingRule}
			/>

			{/* Delete Confirmation */}
			<AlertDialog open={!!deleteRuleId} onOpenChange={(open) => !open && setDeleteRuleId(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete Guardrail Rule</AlertDialogTitle>
						<AlertDialogDescription>
							Are you sure you want to delete &quot;{ruleToDelete?.name}&quot;? This action cannot
							be undone.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={handleDelete}
							disabled={isDeleting}
							className="bg-destructive hover:bg-destructive/90"
						>
							{isDeleting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}
