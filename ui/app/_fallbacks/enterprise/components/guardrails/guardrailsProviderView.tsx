/**
 * Guardrails Provider Configuration View
 * Manages guardrail provider configurations with a sidebar navigation + table layout
 */

"use client";

import { useState, useEffect } from "react";
import { toast } from "sonner";
import { MoreHorizontal, Plus, Edit, Trash2, Loader2, Shield } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
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
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdownMenu";
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
	useGetGuardrailProvidersQuery,
	useCreateGuardrailProviderMutation,
	useUpdateGuardrailProviderMutation,
	useDeleteGuardrailProviderMutation,
} from "@/lib/store/apis/guardrailsApi";
import { getErrorMessage } from "@/lib/store";
import { EnvVarInput } from "@/components/ui/envVarInput";
import { RenderProviderIcon } from "@/lib/constants/icons";
import {
	GuardrailProvider,
	GuardrailProviderType,
	GuardrailProviderFormData,
	DEFAULT_GUARDRAIL_PROVIDER_FORM_DATA,
	GUARDRAIL_PROVIDER_META,
} from "@/lib/types/guardrails";
import { EnvVar } from "@/lib/types/schemas";

// ─── Provider Icon Helper ──────────────────────────────────────────────────────

function ProviderTypeIcon({
	providerType,
	className,
}: {
	providerType: GuardrailProviderType;
	className?: string;
}) {
	const iconMap: Partial<Record<GuardrailProviderType, string>> = {
		bedrock: "bedrock",
		azure_content_moderation: "azure",
		mistral_moderation: "mistral",
	};

	const iconKey = iconMap[providerType];
	if (iconKey) {
		return (
			<RenderProviderIcon
				provider={iconKey as any}
				size="sm"
				className={className ?? "h-5 w-5"}
			/>
		);
	}

	// Fallback for providers without a dedicated icon (e.g., patronus)
	return <Shield className={className ?? "h-5 w-5 text-muted-foreground"} />;
}

// ─── Provider-specific config fields ──────────────────────────────────────────

interface ConfigState {
	// Bedrock
	guardrail_identifier?: string;
	guardrail_version?: string;
	bedrock_region?: string;
	access_key?: EnvVar;
	secret_key?: EnvVar;
	// Azure
	endpoint?: string;
	api_key?: EnvVar;
	categories?: string;
	// Patronus
	patronus_api_key?: EnvVar;
	evaluator_id?: string;
	// Mistral
	mistral_api_key?: EnvVar;
	model?: string;
}

function initConfigState(providerType: GuardrailProviderType, config: Record<string, any>): ConfigState {
	const envVar = (val: any): EnvVar => {
		if (!val) return { value: "", env_var: "", from_env: false };
		if (typeof val === "object" && "value" in val) return val as EnvVar;
		return { value: String(val), env_var: "", from_env: false };
	};

	switch (providerType) {
		case "bedrock":
			return {
				guardrail_identifier: config.guardrail_identifier ?? "",
				guardrail_version: config.guardrail_version ?? "",
				bedrock_region: config.region ?? "",
				access_key: envVar(config.access_key),
				secret_key: envVar(config.secret_key),
			};
		case "azure_content_moderation":
			return {
				endpoint: config.endpoint ?? "",
				api_key: envVar(config.api_key),
				categories: Array.isArray(config.categories)
					? config.categories.join(", ")
					: (config.categories ?? ""),
			};
		case "patronus":
			return {
				patronus_api_key: envVar(config.api_key),
				evaluator_id: config.evaluator_id ?? "",
			};
		case "mistral_moderation":
			return {
				mistral_api_key: envVar(config.api_key),
				model: config.model ?? "",
			};
		default:
			return {};
	}
}

function configStateToPayload(
	providerType: GuardrailProviderType,
	cfg: ConfigState,
): Record<string, any> {
	switch (providerType) {
		case "bedrock":
			return {
				guardrail_identifier: cfg.guardrail_identifier,
				guardrail_version: cfg.guardrail_version,
				region: cfg.bedrock_region,
				access_key: cfg.access_key,
				secret_key: cfg.secret_key,
			};
		case "azure_content_moderation":
			return {
				endpoint: cfg.endpoint,
				api_key: cfg.api_key,
				categories: cfg.categories
					? cfg.categories
							.split(",")
							.map((s) => s.trim())
							.filter(Boolean)
					: [],
			};
		case "patronus":
			return {
				api_key: cfg.patronus_api_key,
				evaluator_id: cfg.evaluator_id,
			};
		case "mistral_moderation":
			return {
				api_key: cfg.mistral_api_key,
				model: cfg.model,
			};
		default:
			return {};
	}
}

function ProviderConfigFields({
	providerType,
	configState,
	onChange,
}: {
	providerType: GuardrailProviderType;
	configState: ConfigState;
	onChange: (update: Partial<ConfigState>) => void;
}) {
	switch (providerType) {
		case "bedrock":
			return (
				<div className="space-y-4">
					<div className="space-y-2">
						<Label htmlFor="guardrail_identifier">Guardrail Identifier</Label>
						<Input
							id="guardrail_identifier"
							placeholder="e.g., my-guardrail-id"
							value={configState.guardrail_identifier ?? ""}
							onChange={(e) => onChange({ guardrail_identifier: e.target.value })}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="guardrail_version">Guardrail Version</Label>
						<Input
							id="guardrail_version"
							placeholder="e.g., DRAFT or 1"
							value={configState.guardrail_version ?? ""}
							onChange={(e) => onChange({ guardrail_version: e.target.value })}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="bedrock_region">Region</Label>
						<Input
							id="bedrock_region"
							placeholder="e.g., us-east-1"
							value={configState.bedrock_region ?? ""}
							onChange={(e) => onChange({ bedrock_region: e.target.value })}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="access_key">Access Key</Label>
						<EnvVarInput
							id="access_key"
							placeholder="Access key or env.MY_AWS_ACCESS_KEY"
							value={configState.access_key}
							onChange={(v) => onChange({ access_key: v })}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="secret_key">Secret Key</Label>
						<EnvVarInput
							id="secret_key"
							placeholder="Secret key or env.MY_AWS_SECRET_KEY"
							value={configState.secret_key}
							onChange={(v) => onChange({ secret_key: v })}
						/>
					</div>
				</div>
			);

		case "azure_content_moderation":
			return (
				<div className="space-y-4">
					<div className="space-y-2">
						<Label htmlFor="az_endpoint">Endpoint</Label>
						<Input
							id="az_endpoint"
							placeholder="https://your-resource.cognitiveservices.azure.com"
							value={configState.endpoint ?? ""}
							onChange={(e) => onChange({ endpoint: e.target.value })}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="az_api_key">API Key</Label>
						<EnvVarInput
							id="az_api_key"
							placeholder="API key or env.AZURE_CONTENT_SAFETY_KEY"
							value={configState.api_key}
							onChange={(v) => onChange({ api_key: v })}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="az_categories">Categories</Label>
						<Input
							id="az_categories"
							placeholder="e.g., Hate, Violence, SelfHarm (comma-separated)"
							value={configState.categories ?? ""}
							onChange={(e) => onChange({ categories: e.target.value })}
						/>
						<p className="text-muted-foreground text-xs">
							Comma-separated list of content categories to moderate.
						</p>
					</div>
				</div>
			);

		case "patronus":
			return (
				<div className="space-y-4">
					<div className="space-y-2">
						<Label htmlFor="patronus_api_key">API Key</Label>
						<EnvVarInput
							id="patronus_api_key"
							placeholder="API key or env.PATRONUS_API_KEY"
							value={configState.patronus_api_key}
							onChange={(v) => onChange({ patronus_api_key: v })}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="evaluator_id">Evaluator ID</Label>
						<Input
							id="evaluator_id"
							placeholder="e.g., lynx-small-1-0"
							value={configState.evaluator_id ?? ""}
							onChange={(e) => onChange({ evaluator_id: e.target.value })}
						/>
					</div>
				</div>
			);

		case "mistral_moderation":
			return (
				<div className="space-y-4">
					<div className="space-y-2">
						<Label htmlFor="mistral_api_key">API Key</Label>
						<EnvVarInput
							id="mistral_api_key"
							placeholder="API key or env.MISTRAL_API_KEY"
							value={configState.mistral_api_key}
							onChange={(v) => onChange({ mistral_api_key: v })}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="mistral_model">Model</Label>
						<Input
							id="mistral_model"
							placeholder="e.g., mistral-moderation-latest"
							value={configState.model ?? ""}
							onChange={(e) => onChange({ model: e.target.value })}
						/>
					</div>
				</div>
			);

		default:
			return null;
	}
}

// ─── Provider Configuration Sheet ─────────────────────────────────────────────

interface ProviderSheetProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	providerType: GuardrailProviderType;
	editingProvider?: GuardrailProvider | null;
}

function ProviderConfigSheet({
	open,
	onOpenChange,
	providerType,
	editingProvider,
}: ProviderSheetProps) {
	const isEditing = !!editingProvider;
	const meta = GUARDRAIL_PROVIDER_META.find((m) => m.type === providerType)!;

	const [createProvider, { isLoading: isCreating }] = useCreateGuardrailProviderMutation();
	const [updateProvider, { isLoading: isUpdating }] = useUpdateGuardrailProviderMutation();
	const isLoading = isCreating || isUpdating;

	const [name, setName] = useState("");
	const [enabled, setEnabled] = useState(true);
	const [timeoutSeconds, setTimeoutSeconds] = useState(30);
	const [configState, setConfigState] = useState<ConfigState>({});
	const [nameError, setNameError] = useState("");

	useEffect(() => {
		if (open) {
			if (editingProvider) {
				setName(editingProvider.name);
				setEnabled(editingProvider.enabled);
				setTimeoutSeconds(editingProvider.timeout_seconds ?? 30);
				setConfigState(initConfigState(providerType, editingProvider.config ?? {}));
			} else {
				setName("");
				setEnabled(true);
				setTimeoutSeconds(30);
				setConfigState(initConfigState(providerType, {}));
			}
			setNameError("");
		}
	}, [open, editingProvider, providerType]);

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();

		if (!name.trim()) {
			setNameError("Name is required");
			return;
		}
		setNameError("");

		const payload = {
			name: name.trim(),
			provider_type: providerType,
			enabled,
			timeout_seconds: timeoutSeconds,
			config: configStateToPayload(providerType, configState),
		};

		try {
			if (isEditing && editingProvider) {
				await updateProvider({ id: editingProvider.id, data: payload }).unwrap();
				toast.success("Provider configuration updated successfully");
			} else {
				await createProvider(payload).unwrap();
				toast.success("Provider configuration created successfully");
			}
			onOpenChange(false);
		} catch (error: any) {
			toast.error(getErrorMessage(error));
		}
	};

	return (
		<Sheet open={open} onOpenChange={onOpenChange}>
			<SheetContent className="flex w-full flex-col min-w-1/2 gap-0 overflow-x-hidden p-0">
				<SheetHeader className="border-b p-6">
					<div className="flex items-center gap-3">
						<ProviderTypeIcon providerType={providerType} className="h-6 w-6 shrink-0" />
						<div>
							<SheetTitle>
								{isEditing ? `Edit ${meta.label} Configuration` : `Add ${meta.label} Configuration`}
							</SheetTitle>
							<SheetDescription className="mt-0.5">{meta.description}</SheetDescription>
						</div>
					</div>
				</SheetHeader>

				<form onSubmit={handleSubmit} className="flex flex-col flex-1 overflow-hidden">
					<div className="flex-1 overflow-y-auto p-6 space-y-6">
						{/* Name */}
						<div className="space-y-2">
							<Label htmlFor="provider-name">
								Name <span className="text-destructive">*</span>
							</Label>
							<Input
								id="provider-name"
								placeholder={`e.g., Production ${meta.label}`}
								value={name}
								onChange={(e) => {
									setName(e.target.value);
									if (e.target.value.trim()) setNameError("");
								}}
							/>
							{nameError && <p className="text-destructive text-sm">{nameError}</p>}
						</div>

						{/* Enabled */}
						<div className="flex items-center justify-between rounded-lg border p-4">
							<div className="space-y-0.5">
								<Label htmlFor="provider-enabled">Enable Provider</Label>
								<p className="text-muted-foreground text-sm">
									Provider will be available for guardrail rules
								</p>
							</div>
							<Switch
								id="provider-enabled"
								checked={enabled}
								onCheckedChange={setEnabled}
							/>
						</div>

						{/* Timeout */}
						<div className="space-y-2">
							<Label htmlFor="provider-timeout">Timeout (Seconds)</Label>
							<Input
								id="provider-timeout"
								type="number"
								min={1}
								value={timeoutSeconds}
								onChange={(e) => setTimeoutSeconds(Math.max(1, Number(e.target.value)))}
							/>
						</div>

						{/* Provider-specific config */}
						<div className="space-y-4">
							<div>
								<h3 className="text-sm font-medium">Provider Configuration</h3>
								<p className="text-muted-foreground mt-0.5 text-xs">
									Use <code className="bg-muted rounded px-1 text-xs">env.VAR_NAME</code> to
									reference environment variables.
								</p>
							</div>
							<ProviderConfigFields
								providerType={providerType}
								configState={configState}
								onChange={(update) => setConfigState((prev) => ({ ...prev, ...update }))}
							/>
						</div>
					</div>

					{/* Footer */}
					<div className="border-t p-6 flex items-center justify-end gap-3">
						<Button
							type="button"
							variant="outline"
							onClick={() => onOpenChange(false)}
							disabled={isLoading}
						>
							Cancel
						</Button>
						<Button type="submit" disabled={isLoading}>
							{isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
							{isEditing ? "Update Configuration" : "Save Configuration"}
						</Button>
					</div>
				</form>
			</SheetContent>
		</Sheet>
	);
}

// ─── Providers Table ───────────────────────────────────────────────────────────

interface ProvidersTableProps {
	providers: GuardrailProvider[];
	isLoading: boolean;
	providerType: GuardrailProviderType;
	onEdit: (provider: GuardrailProvider) => void;
	onDelete: (providerId: string) => void;
	onAddNew: () => void;
}

function ProvidersTable({
	providers,
	isLoading,
	providerType,
	onEdit,
	onDelete,
	onAddNew,
}: ProvidersTableProps) {
	const meta = GUARDRAIL_PROVIDER_META.find((m) => m.type === providerType)!;

	const filtered = providers.filter((p) => p.provider_type === providerType);

	return (
		<div className="space-y-4 flex-1">
			<div className="flex items-center justify-between">
				<div>
					<h2 className="text-foreground text-base font-semibold">
						{meta.label} Guardrail Configurations
					</h2>
					<p className="text-muted-foreground text-sm">{meta.description}</p>
				</div>
				<Button onClick={onAddNew} className="gap-2" size="sm">
					<Plus className="h-4 w-4" />
					Add new configuration
				</Button>
			</div>

			<div className="rounded-sm border overflow-hidden">
				<Table>
					<TableHeader>
						<TableRow className="bg-muted/50">
							<TableHead className="font-semibold">ID</TableHead>
							<TableHead className="font-semibold">Name</TableHead>
							<TableHead className="font-semibold">Is Enabled</TableHead>
							<TableHead className="font-semibold">Timeout (s)</TableHead>
							<TableHead className="text-right font-semibold">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading ? (
							[...Array(3)].map((_, i) => (
								<TableRow key={i}>
									<TableCell colSpan={5} className="h-10">
										<div className="h-2 w-48 animate-pulse rounded bg-muted" />
									</TableCell>
								</TableRow>
							))
						) : filtered.length === 0 ? (
							<TableRow>
								<TableCell colSpan={5} className="h-32 text-center">
									<div className="flex flex-col items-center gap-2">
										<p className="text-muted-foreground text-sm">
											No {meta.label} configurations yet.
										</p>
										<Button
											variant="outline"
											size="sm"
											onClick={onAddNew}
											className="gap-1"
										>
											<Plus className="h-3.5 w-3.5" />
											Add configuration
										</Button>
									</div>
								</TableCell>
							</TableRow>
						) : (
							filtered.map((provider) => (
								<TableRow
									key={provider.id}
									className="hover:bg-muted/50 transition-colors"
								>
									<TableCell>
										<span
											className="font-mono text-xs text-muted-foreground truncate block max-w-[140px]"
											title={provider.id}
										>
											{provider.id}
										</span>
									</TableCell>
									<TableCell className="font-medium">{provider.name}</TableCell>
									<TableCell>
										<Badge variant={provider.enabled ? "default" : "secondary"}>
											{provider.enabled ? "Enabled" : "Disabled"}
										</Badge>
									</TableCell>
									<TableCell>{provider.timeout_seconds ?? "—"}s</TableCell>
									<TableCell className="text-right">
										<DropdownMenu>
											<DropdownMenuTrigger asChild>
												<Button variant="ghost" size="sm" aria-label="Provider actions">
													<MoreHorizontal className="h-4 w-4" />
												</Button>
											</DropdownMenuTrigger>
											<DropdownMenuContent align="end">
												<DropdownMenuItem onClick={() => onEdit(provider)}>
													<Edit className="mr-2 h-4 w-4" />
													Edit
												</DropdownMenuItem>
												<DropdownMenuItem
													onClick={() => onDelete(provider.id)}
													className="text-destructive focus:text-destructive"
												>
													<Trash2 className="mr-2 h-4 w-4" />
													Delete
												</DropdownMenuItem>
											</DropdownMenuContent>
										</DropdownMenu>
									</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</Table>
			</div>
		</div>
	);
}

// ─── Main View ─────────────────────────────────────────────────────────────────

export default function GuardrailsProviderView() {
	const [selectedType, setSelectedType] = useState<GuardrailProviderType>("bedrock");
	const [sheetOpen, setSheetOpen] = useState(false);
	const [editingProvider, setEditingProvider] = useState<GuardrailProvider | null>(null);
	const [deleteProviderId, setDeleteProviderId] = useState<string | null>(null);

	const { data: providersData, isLoading } = useGetGuardrailProvidersQuery();
	const providers = providersData?.providers ?? [];

	const [deleteProvider, { isLoading: isDeleting }] = useDeleteGuardrailProviderMutation();

	const providerToDelete = providers.find((p) => p.id === deleteProviderId);

	const handleAddNew = () => {
		setEditingProvider(null);
		setSheetOpen(true);
	};

	const handleEdit = (provider: GuardrailProvider) => {
		setEditingProvider(provider);
		setSheetOpen(true);
	};

	const handleSheetOpenChange = (open: boolean) => {
		setSheetOpen(open);
		if (!open) setEditingProvider(null);
	};

	const handleDelete = async () => {
		if (!deleteProviderId) return;
		try {
			await deleteProvider(deleteProviderId).unwrap();
			toast.success("Provider configuration deleted successfully");
			setDeleteProviderId(null);
		} catch (error: any) {
			toast.error(getErrorMessage(error));
		}
	};

	// Count providers per type for sidebar badges
	const countByType = (type: GuardrailProviderType) =>
		providers.filter((p) => p.provider_type === type).length;

	return (
		<div className="flex h-full gap-0">
			{/* Left Sidebar */}
			<aside className="w-56 shrink-0 border-r">
				<div className="p-4 border-b">
					<h1 className="text-foreground text-sm font-semibold">Guardrail Providers</h1>
					<p className="text-muted-foreground text-xs mt-0.5">Select a provider type</p>
				</div>
				<nav className="p-2 space-y-1">
					{GUARDRAIL_PROVIDER_META.map((meta) => {
						const isSelected = selectedType === meta.type;
						const count = countByType(meta.type);
						return (
							<button
								key={meta.type}
								type="button"
								onClick={() => setSelectedType(meta.type)}
								className={`w-full flex items-center gap-3 rounded-md px-3 py-2.5 text-sm text-left transition-colors ${
									isSelected
										? "bg-primary/10 text-primary font-medium"
										: "text-muted-foreground hover:bg-muted hover:text-foreground"
								}`}
							>
								<ProviderTypeIcon
									providerType={meta.type}
									className={`h-4 w-4 shrink-0 ${isSelected ? "text-primary" : ""}`}
								/>
								<span className="flex-1 truncate">{meta.label}</span>
								{count > 0 && (
									<Badge
										variant={isSelected ? "default" : "secondary"}
										className="text-xs px-1.5 py-0 h-4 min-w-[16px] flex items-center justify-center"
									>
										{count}
									</Badge>
								)}
							</button>
						);
					})}
				</nav>
			</aside>

			{/* Main Content */}
			<main className="flex-1 overflow-auto p-6">
				<ProvidersTable
					providers={providers}
					isLoading={isLoading}
					providerType={selectedType}
					onEdit={handleEdit}
					onDelete={setDeleteProviderId}
					onAddNew={handleAddNew}
				/>
			</main>

			{/* Provider Config Sheet */}
			<ProviderConfigSheet
				open={sheetOpen}
				onOpenChange={handleSheetOpenChange}
				providerType={selectedType}
				editingProvider={editingProvider}
			/>

			{/* Delete Confirmation */}
			<AlertDialog
				open={!!deleteProviderId}
				onOpenChange={(open) => !open && setDeleteProviderId(null)}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete Provider Configuration</AlertDialogTitle>
						<AlertDialogDescription>
							Are you sure you want to delete &quot;{providerToDelete?.name}&quot;? This may
							affect guardrail rules that reference this provider.
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
