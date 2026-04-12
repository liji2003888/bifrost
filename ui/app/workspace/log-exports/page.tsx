"use client";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
} from "@/components/ui/sheet";
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
	useGetLogExportsQuery,
	useCreateLogExportMutation,
	useCreateMCPLogExportMutation,
	useGetLogExportConfigsQuery,
	useCreateLogExportConfigMutation,
	useUpdateLogExportConfigMutation,
	useDeleteLogExportConfigMutation,
	useTriggerLogExportConfigMutation,
} from "@/lib/store/apis/enterpriseApi";
import { getErrorMessage } from "@/lib/store";
import type { ExportJob, ExportJobStatus, ExportScope, LogExportConfig, LogExportDestinationType, LogExportFrequency } from "@/lib/types/enterprise";
import { formatRelativeTimestamp } from "@/lib/utils/enterprise";
import { AlertCircle, Calendar, Clock, Download, Edit, FileText, Loader2, Play, Plus, RefreshCw, Settings, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

function statusBadgeVariant(status: ExportJobStatus | string): "default" | "secondary" | "destructive" | "outline" {
	switch (status) {
		case "completed":
		case "success":
			return "default";
		case "running":
			return "outline";
		case "pending":
			return "secondary";
		case "failed":
			return "destructive";
		default:
			return "secondary";
	}
}

const DEST_LABELS: Record<string, string> = {
	local: "Local Disk",
	s3: "Amazon S3",
	gcs: "Google Cloud Storage",
	azure_blob: "Azure Blob Storage",
};

const FREQ_LABELS: Record<string, string> = {
	daily: "Daily",
	weekly: "Weekly",
	monthly: "Monthly",
};

// ─── Config Form State ───────────────────────────────────────────────────────

interface ConfigFormState {
	name: string;
	description: string;
	enabled: boolean;
	frequency: LogExportFrequency;
	schedule_time: string;
	schedule_day: string;
	timezone: string;
	destination_type: LogExportDestinationType;
	// S3
	s3_bucket: string;
	s3_region: string;
	s3_prefix: string;
	s3_access_key: string;
	s3_secret_key: string;
	// GCS
	gcs_bucket: string;
	gcs_prefix: string;
	gcs_service_account_key: string;
	// Azure
	azure_container: string;
	azure_account_name: string;
	azure_account_key: string;
	azure_prefix: string;
	// Local
	local_path: string;
	// Data
	format: "jsonl" | "csv";
	compression: "none" | "gzip";
	max_rows: number;
	data_scope: "logs" | "mcp_logs";
}

const DEFAULT_FORM: ConfigFormState = {
	name: "",
	description: "",
	enabled: true,
	frequency: "daily",
	schedule_time: "02:00",
	schedule_day: "",
	timezone: "UTC",
	destination_type: "local",
	s3_bucket: "", s3_region: "us-east-1", s3_prefix: "logs/{year}/{month}/{day}/", s3_access_key: "", s3_secret_key: "",
	gcs_bucket: "", gcs_prefix: "logs/{year}/{month}/{day}/", gcs_service_account_key: "",
	azure_container: "", azure_account_name: "", azure_account_key: "", azure_prefix: "logs/{year}/{month}/{day}/",
	local_path: "",
	format: "jsonl",
	compression: "gzip",
	max_rows: 100000,
	data_scope: "logs",
};

function formToPayload(form: ConfigFormState): Partial<LogExportConfig> {
	let destConfig: Record<string, any> = {};
	switch (form.destination_type) {
		case "s3":
			destConfig = { bucket: form.s3_bucket, region: form.s3_region, prefix: form.s3_prefix, credentials: { access_key_id: form.s3_access_key, secret_access_key: form.s3_secret_key } };
			break;
		case "gcs":
			destConfig = { bucket: form.gcs_bucket, prefix: form.gcs_prefix, credentials: { service_account_key: form.gcs_service_account_key } };
			break;
		case "azure_blob":
			destConfig = { container: form.azure_container, account_name: form.azure_account_name, account_key: form.azure_account_key, prefix: form.azure_prefix };
			break;
		case "local":
			destConfig = { storage_path: form.local_path };
			break;
	}
	return {
		name: form.name,
		description: form.description,
		enabled: form.enabled,
		frequency: form.frequency,
		schedule_time: form.schedule_time,
		schedule_day: form.schedule_day,
		timezone: form.timezone,
		destination_type: form.destination_type,
		destination_config: destConfig,
		format: form.format,
		compression: form.compression,
		max_rows: form.max_rows,
		data_scope: form.data_scope,
	};
}

function configToForm(cfg: LogExportConfig): ConfigFormState {
	const dc = cfg.destination_config || {};
	return {
		name: cfg.name,
		description: cfg.description || "",
		enabled: cfg.enabled,
		frequency: cfg.frequency,
		schedule_time: cfg.schedule_time || "02:00",
		schedule_day: cfg.schedule_day || "",
		timezone: cfg.timezone || "UTC",
		destination_type: cfg.destination_type,
		s3_bucket: dc.bucket || "", s3_region: dc.region || "us-east-1", s3_prefix: dc.prefix || "",
		s3_access_key: dc.credentials?.access_key_id || "", s3_secret_key: dc.credentials?.secret_access_key || "",
		gcs_bucket: dc.bucket || "", gcs_prefix: dc.prefix || "", gcs_service_account_key: dc.credentials?.service_account_key || "",
		azure_container: dc.container || "", azure_account_name: dc.account_name || "", azure_account_key: dc.account_key || "", azure_prefix: dc.prefix || "",
		local_path: dc.storage_path || "",
		format: cfg.format as any || "jsonl",
		compression: cfg.compression as any || "gzip",
		max_rows: cfg.max_rows || 100000,
		data_scope: cfg.data_scope as any || "logs",
	};
}

// ─── Config Sheet ────────────────────────────────────────────────────────────

function ConfigSheet({ open, onOpenChange, editing }: { open: boolean; onOpenChange: (o: boolean) => void; editing?: LogExportConfig | null }) {
	const isEditing = !!editing;
	const [form, setForm] = useState<ConfigFormState>({ ...DEFAULT_FORM });
	const [createConfig, { isLoading: isCreating }] = useCreateLogExportConfigMutation();
	const [updateConfig, { isLoading: isUpdating }] = useUpdateLogExportConfigMutation();
	const isLoading = isCreating || isUpdating;

	useEffect(() => {
		if (open) {
			setForm(editing ? configToForm(editing) : { ...DEFAULT_FORM });
		}
	}, [open, editing]);

	const update = (partial: Partial<ConfigFormState>) => setForm((prev) => ({ ...prev, ...partial }));

	const handleSubmit = async () => {
		if (!form.name.trim()) { toast.error("Name is required"); return; }
		try {
			const payload = formToPayload(form);
			if (isEditing && editing) {
				await updateConfig({ id: editing.id, data: payload }).unwrap();
				toast.success("Export configuration updated");
			} else {
				await createConfig(payload).unwrap();
				toast.success("Export configuration created");
			}
			onOpenChange(false);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	return (
		<Sheet open={open} onOpenChange={onOpenChange}>
			<SheetContent className="flex w-full flex-col min-w-1/2 gap-0 overflow-x-hidden p-0">
				<SheetHeader className="border-b p-6">
					<SheetTitle>{isEditing ? "Edit Export Configuration" : "New Export Configuration"}</SheetTitle>
					<SheetDescription>Configure a scheduled log export to a storage destination.</SheetDescription>
				</SheetHeader>
				<div className="flex-1 overflow-y-auto p-6 space-y-6">
					{/* General */}
					<div className="space-y-4">
						<h3 className="text-sm font-semibold">General</h3>
						<div className="grid grid-cols-2 gap-4">
							<div className="space-y-2">
								<Label>Name <span className="text-destructive">*</span></Label>
								<Input value={form.name} onChange={(e) => update({ name: e.target.value })} placeholder="e.g., daily_s3_export" />
							</div>
							<div className="space-y-2">
								<Label>Data Scope</Label>
								<Select value={form.data_scope} onValueChange={(v) => update({ data_scope: v as any })}>
									<SelectTrigger><SelectValue /></SelectTrigger>
									<SelectContent>
										<SelectItem value="logs">LLM Logs</SelectItem>
										<SelectItem value="mcp_logs">MCP Logs</SelectItem>
									</SelectContent>
								</Select>
							</div>
						</div>
						<div className="space-y-2">
							<Label>Description</Label>
							<Textarea value={form.description} onChange={(e) => update({ description: e.target.value })} rows={2} placeholder="Optional description..." />
						</div>
						<div className="flex items-center justify-between rounded-lg border p-4">
							<div><Label>Enabled</Label><p className="text-muted-foreground text-xs">Enable scheduled exports</p></div>
							<Switch checked={form.enabled} onCheckedChange={(v) => update({ enabled: v })} />
						</div>
					</div>

					<Separator />

					{/* Schedule */}
					<div className="space-y-4">
						<h3 className="text-sm font-semibold flex items-center gap-2"><Calendar className="h-4 w-4" /> Schedule</h3>
						<div className="grid grid-cols-3 gap-4">
							<div className="space-y-2">
								<Label>Frequency</Label>
								<Select value={form.frequency} onValueChange={(v) => update({ frequency: v as LogExportFrequency })}>
									<SelectTrigger><SelectValue /></SelectTrigger>
									<SelectContent>
										<SelectItem value="daily">Daily</SelectItem>
										<SelectItem value="weekly">Weekly</SelectItem>
										<SelectItem value="monthly">Monthly</SelectItem>
									</SelectContent>
								</Select>
							</div>
							<div className="space-y-2">
								<Label>Time (HH:MM)</Label>
								<Input type="time" value={form.schedule_time} onChange={(e) => update({ schedule_time: e.target.value })} />
							</div>
							{form.frequency === "weekly" && (
								<div className="space-y-2">
									<Label>Day of Week</Label>
									<Select value={form.schedule_day} onValueChange={(v) => update({ schedule_day: v })}>
										<SelectTrigger><SelectValue placeholder="Select day" /></SelectTrigger>
										<SelectContent>
											{["sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"].map((d) => (
												<SelectItem key={d} value={d}>{d.charAt(0).toUpperCase() + d.slice(1)}</SelectItem>
											))}
										</SelectContent>
									</Select>
								</div>
							)}
							{form.frequency === "monthly" && (
								<div className="space-y-2">
									<Label>Day of Month</Label>
									<Input type="number" min={1} max={28} value={form.schedule_day} onChange={(e) => update({ schedule_day: e.target.value })} placeholder="1-28" />
								</div>
							)}
						</div>
						<div className="space-y-2">
							<Label>Timezone</Label>
							<Input value={form.timezone} onChange={(e) => update({ timezone: e.target.value })} placeholder="UTC" />
						</div>
					</div>

					<Separator />

					{/* Destination */}
					<div className="space-y-4">
						<h3 className="text-sm font-semibold flex items-center gap-2"><Settings className="h-4 w-4" /> Destination</h3>
						<div className="space-y-2">
							<Label>Destination Type</Label>
							<Select value={form.destination_type} onValueChange={(v) => update({ destination_type: v as LogExportDestinationType })}>
								<SelectTrigger><SelectValue /></SelectTrigger>
								<SelectContent>
									<SelectItem value="local">Local Disk</SelectItem>
									<SelectItem value="s3">Amazon S3</SelectItem>
									<SelectItem value="gcs">Google Cloud Storage</SelectItem>
									<SelectItem value="azure_blob">Azure Blob Storage</SelectItem>
								</SelectContent>
							</Select>
						</div>

						{form.destination_type === "s3" && (
							<div className="grid grid-cols-2 gap-4 rounded-lg border p-4">
								<div className="space-y-2"><Label>Bucket</Label><Input value={form.s3_bucket} onChange={(e) => update({ s3_bucket: e.target.value })} placeholder="bifrost-logs" /></div>
								<div className="space-y-2"><Label>Region</Label><Input value={form.s3_region} onChange={(e) => update({ s3_region: e.target.value })} placeholder="us-west-2" /></div>
								<div className="col-span-2 space-y-2"><Label>Prefix</Label><Input value={form.s3_prefix} onChange={(e) => update({ s3_prefix: e.target.value })} placeholder="logs/{year}/{month}/{day}/" /></div>
								<div className="space-y-2"><Label>Access Key ID</Label><Input value={form.s3_access_key} onChange={(e) => update({ s3_access_key: e.target.value })} placeholder="AKIA..." /></div>
								<div className="space-y-2"><Label>Secret Access Key</Label><Input type="password" value={form.s3_secret_key} onChange={(e) => update({ s3_secret_key: e.target.value })} placeholder="Secret key" /></div>
							</div>
						)}

						{form.destination_type === "gcs" && (
							<div className="grid grid-cols-2 gap-4 rounded-lg border p-4">
								<div className="space-y-2"><Label>Bucket</Label><Input value={form.gcs_bucket} onChange={(e) => update({ gcs_bucket: e.target.value })} placeholder="bifrost-logs" /></div>
								<div className="space-y-2"><Label>Prefix</Label><Input value={form.gcs_prefix} onChange={(e) => update({ gcs_prefix: e.target.value })} placeholder="logs/{year}/{month}/{day}/" /></div>
								<div className="col-span-2 space-y-2"><Label>Service Account Key</Label><Textarea value={form.gcs_service_account_key} onChange={(e) => update({ gcs_service_account_key: e.target.value })} rows={3} placeholder="Paste JSON key..." /></div>
							</div>
						)}

						{form.destination_type === "azure_blob" && (
							<div className="grid grid-cols-2 gap-4 rounded-lg border p-4">
								<div className="space-y-2"><Label>Container</Label><Input value={form.azure_container} onChange={(e) => update({ azure_container: e.target.value })} placeholder="bifrost-logs" /></div>
								<div className="space-y-2"><Label>Account Name</Label><Input value={form.azure_account_name} onChange={(e) => update({ azure_account_name: e.target.value })} /></div>
								<div className="space-y-2"><Label>Account Key</Label><Input type="password" value={form.azure_account_key} onChange={(e) => update({ azure_account_key: e.target.value })} /></div>
								<div className="space-y-2"><Label>Prefix</Label><Input value={form.azure_prefix} onChange={(e) => update({ azure_prefix: e.target.value })} placeholder="logs/{year}/{month}/{day}/" /></div>
							</div>
						)}

						{form.destination_type === "local" && (
							<div className="rounded-lg border p-4 space-y-2">
								<Label>Storage Path</Label>
								<Input value={form.local_path} onChange={(e) => update({ local_path: e.target.value })} placeholder="./exports (leave empty for default)" />
							</div>
						)}
					</div>

					<Separator />

					{/* Data Format */}
					<div className="space-y-4">
						<h3 className="text-sm font-semibold">Data Format</h3>
						<div className="grid grid-cols-3 gap-4">
							<div className="space-y-2">
								<Label>Format</Label>
								<Select value={form.format} onValueChange={(v) => update({ format: v as any })}>
									<SelectTrigger><SelectValue /></SelectTrigger>
									<SelectContent>
										<SelectItem value="jsonl">JSONL</SelectItem>
										<SelectItem value="csv">CSV</SelectItem>
									</SelectContent>
								</Select>
							</div>
							<div className="space-y-2">
								<Label>Compression</Label>
								<Select value={form.compression} onValueChange={(v) => update({ compression: v as any })}>
									<SelectTrigger><SelectValue /></SelectTrigger>
									<SelectContent>
										<SelectItem value="none">None</SelectItem>
										<SelectItem value="gzip">Gzip</SelectItem>
									</SelectContent>
								</Select>
							</div>
							<div className="space-y-2">
								<Label>Max Rows</Label>
								<Input type="number" value={form.max_rows} onChange={(e) => update({ max_rows: parseInt(e.target.value) || 100000 })} min={1} />
							</div>
						</div>
					</div>
				</div>

				{/* Footer */}
				<div className="border-t p-6 flex justify-end gap-3">
					<Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading}>Cancel</Button>
					<Button onClick={handleSubmit} disabled={isLoading}>
						{isLoading && <Loader2 className="h-4 w-4 animate-spin" />}
						{isEditing ? "Update" : "Create"}
					</Button>
				</div>
			</SheetContent>
		</Sheet>
	);
}

// ─── Main Page ───────────────────────────────────────────────────────────────

export default function LogExportsPage() {
	// Manual export state
	const [exportScope, setExportScope] = useState<ExportScope>("logs");
	const [exportFormat, setExportFormat] = useState<"jsonl" | "csv">("jsonl");
	const [compression, setCompression] = useState<"" | "gzip">("");
	const [maxRows, setMaxRows] = useState(10000);
	const [startTime, setStartTime] = useState("");
	const [endTime, setEndTime] = useState("");

	// Config sheet state
	const [configSheetOpen, setConfigSheetOpen] = useState(false);
	const [editingConfig, setEditingConfig] = useState<LogExportConfig | null>(null);
	const [deleteConfigId, setDeleteConfigId] = useState<string | null>(null);

	// Queries
	const { data: exportsResponse, error: exportsError, isFetching, refetch } = useGetLogExportsQuery({ cluster: true }, { pollingInterval: 10000, skipPollingIfUnfocused: true });
	const { data: configsResponse, error: configsError } = useGetLogExportConfigsQuery(undefined, { pollingInterval: 30000, skipPollingIfUnfocused: true });
	const [createLogExport, { isLoading: isCreatingLog }] = useCreateLogExportMutation();
	const [createMCPLogExport, { isLoading: isCreatingMCP }] = useCreateMCPLogExportMutation();
	const [deleteConfig] = useDeleteLogExportConfigMutation();
	const [triggerConfig] = useTriggerLogExportConfigMutation();
	const isCreating = isCreatingLog || isCreatingMCP;

	const jobs = exportsResponse?.jobs ?? [];
	const configs = configsResponse?.configs ?? [];

	const handleExport = async () => {
		try {
			const logFilters: Record<string, any> = {};
			if (startTime) logFilters.start_time = new Date(startTime).toISOString();
			if (endTime) logFilters.end_time = new Date(endTime).toISOString();
			const payload: any = {
				format: exportFormat,
				compression: compression || undefined,
				max_rows: maxRows > 0 ? maxRows : undefined,
			};
			if (Object.keys(logFilters).length > 0) payload.log_filters = logFilters;
			if (exportScope === "mcp_logs") {
				await createMCPLogExport(payload).unwrap();
			} else {
				await createLogExport(payload).unwrap();
			}
			toast.success("Export job submitted successfully");
		} catch (error) {
			toast.error(getErrorMessage(error));
		}
	};

	const handleDownload = (job: ExportJob) => {
		if (job.status !== "completed") return;
		const baseUrl = typeof window !== "undefined" ? `${window.location.origin}/api` : "/api";
		window.open(`${baseUrl}/log-exports/${job.id}/download`, "_blank");
	};

	const handleDeleteConfig = async () => {
		if (!deleteConfigId) return;
		try {
			await deleteConfig(deleteConfigId).unwrap();
			toast.success("Export configuration deleted");
			setDeleteConfigId(null);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	const handleTrigger = async (id: string) => {
		try {
			await triggerConfig(id).unwrap();
			toast.success("Export triggered successfully");
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	return (
		<div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h1 className="text-2xl font-semibold tracking-tight">Log Exports</h1>
					<p className="text-muted-foreground mt-1 text-sm">
						Export request logs and MCP tool logs. Configure scheduled exports to cloud storage or run one-time exports.
					</p>
				</div>
				<Button variant="outline" onClick={() => void refetch()} isLoading={isFetching} dataTestId="log-exports-refresh">
					{!isFetching && <RefreshCw className="h-4 w-4" />}
					Refresh
				</Button>
			</div>

			{/* ─── Scheduled Export Configurations ─────────────────────────────── */}
			<Card className="shadow-none">
				<CardHeader className="flex flex-row items-center justify-between pb-3">
					<CardTitle className="text-base flex items-center gap-2"><Clock className="h-4 w-4" /> Export Configurations</CardTitle>
					<Button size="sm" onClick={() => { setEditingConfig(null); setConfigSheetOpen(true); }}>
						<Plus className="h-4 w-4" /> Add Configuration
					</Button>
				</CardHeader>
				<CardContent>
					{Boolean(configsError) && (
						<Alert variant="destructive" className="mb-4">
							<AlertCircle />
							<AlertDescription>{getErrorMessage(configsError)}</AlertDescription>
						</Alert>
					)}
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Name</TableHead>
								<TableHead>Destination</TableHead>
								<TableHead>Schedule</TableHead>
								<TableHead>Format</TableHead>
								<TableHead>Last Run</TableHead>
								<TableHead>Next Run</TableHead>
								<TableHead>Status</TableHead>
								<TableHead className="text-right">Actions</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{configs.length === 0 ? (
								<TableRow>
									<TableCell colSpan={8} className="h-20 text-center">
										<span className="text-muted-foreground text-sm">No export configurations. Create one to set up scheduled exports.</span>
									</TableCell>
								</TableRow>
							) : (
								configs.map((cfg) => (
									<TableRow key={cfg.id}>
										<TableCell>
											<div className="flex flex-col">
												<span className="font-medium">{cfg.name}</span>
												{cfg.description && <span className="text-muted-foreground text-xs">{cfg.description}</span>}
											</div>
										</TableCell>
										<TableCell><Badge variant="outline">{DEST_LABELS[cfg.destination_type] || cfg.destination_type}</Badge></TableCell>
										<TableCell className="text-xs">{FREQ_LABELS[cfg.frequency] || cfg.frequency} at {cfg.schedule_time} {cfg.timezone}</TableCell>
										<TableCell className="text-xs">{cfg.format.toUpperCase()}{cfg.compression === "gzip" ? " + gzip" : ""}</TableCell>
										<TableCell className="text-xs">{cfg.last_run_at ? formatRelativeTimestamp(cfg.last_run_at) : "-"}</TableCell>
										<TableCell className="text-xs">{cfg.next_run_at ? formatRelativeTimestamp(cfg.next_run_at) : "-"}</TableCell>
										<TableCell>
											{cfg.last_run_status ? (
												<Badge variant={statusBadgeVariant(cfg.last_run_status)}>{cfg.last_run_status}</Badge>
											) : (
												<Badge variant={cfg.enabled ? "outline" : "secondary"}>{cfg.enabled ? "Enabled" : "Disabled"}</Badge>
											)}
										</TableCell>
										<TableCell className="text-right">
											<div className="flex items-center justify-end gap-1">
												<Button variant="ghost" size="sm" onClick={() => handleTrigger(cfg.id)} title="Run Now"><Play className="h-4 w-4" /></Button>
												<Button variant="ghost" size="sm" onClick={() => { setEditingConfig(cfg); setConfigSheetOpen(true); }} title="Edit"><Edit className="h-4 w-4" /></Button>
												<Button variant="ghost" size="sm" onClick={() => setDeleteConfigId(cfg.id)} title="Delete"><Trash2 className="h-4 w-4" /></Button>
											</div>
										</TableCell>
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				</CardContent>
			</Card>

			{/* ─── One-Time Export ─────────────────────────────────────────────── */}
			<Card className="shadow-none">
				<CardHeader className="pb-3">
					<CardTitle className="text-base">One-Time Export</CardTitle>
				</CardHeader>
				<CardContent className="space-y-4">
					<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
						<div className="space-y-2">
							<Label>Start Time</Label>
							<Input type="datetime-local" value={startTime} onChange={(e) => setStartTime(e.target.value)} />
							<p className="text-muted-foreground text-xs">Leave empty for no lower bound</p>
						</div>
						<div className="space-y-2">
							<Label>End Time</Label>
							<Input type="datetime-local" value={endTime} onChange={(e) => setEndTime(e.target.value)} />
							<p className="text-muted-foreground text-xs">Leave empty for no upper bound</p>
						</div>
						<div className="space-y-2">
							<Label>Scope</Label>
							<Select value={exportScope} onValueChange={(v) => setExportScope(v as ExportScope)}>
								<SelectTrigger><SelectValue /></SelectTrigger>
								<SelectContent>
									<SelectItem value="logs">LLM Logs</SelectItem>
									<SelectItem value="mcp_logs">MCP Logs</SelectItem>
								</SelectContent>
							</Select>
						</div>
					</div>
					<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
						<div className="space-y-2">
							<Label>Format</Label>
							<Select value={exportFormat} onValueChange={(v) => setExportFormat(v as "jsonl" | "csv")}>
								<SelectTrigger><SelectValue /></SelectTrigger>
								<SelectContent>
									<SelectItem value="jsonl">JSONL</SelectItem>
									<SelectItem value="csv">CSV</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div className="space-y-2">
							<Label>Compression</Label>
							<Select value={compression || "none"} onValueChange={(v) => setCompression(v === "none" ? "" : "gzip")}>
								<SelectTrigger><SelectValue /></SelectTrigger>
								<SelectContent>
									<SelectItem value="none">None</SelectItem>
									<SelectItem value="gzip">Gzip</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div className="space-y-2">
							<Label>Max Rows</Label>
							<Input type="number" value={maxRows} min={1} max={1000000} onChange={(e) => setMaxRows(parseInt(e.target.value) || 10000)} />
						</div>
					</div>
					<div className="flex justify-end">
						<Button onClick={() => void handleExport()} disabled={isCreating}>
							{isCreating ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
							Start Export
						</Button>
					</div>
				</CardContent>
			</Card>

			{/* ─── Export History ──────────────────────────────────────────────── */}
			<Card className="shadow-none">
				<CardHeader className="pb-3">
					<CardTitle className="text-base">Export History</CardTitle>
				</CardHeader>
				<CardContent>
					{Boolean(exportsError) && (
						<Alert variant="destructive" className="mb-4">
							<AlertCircle />
							<AlertTitle>Unable to load export jobs</AlertTitle>
							<AlertDescription>{getErrorMessage(exportsError)}</AlertDescription>
						</Alert>
					)}
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>ID</TableHead>
								<TableHead>Scope</TableHead>
								<TableHead>Format</TableHead>
								<TableHead>Status</TableHead>
								<TableHead>Rows</TableHead>
								<TableHead>Node</TableHead>
								<TableHead>Created</TableHead>
								<TableHead>Completed</TableHead>
								<TableHead className="text-right">Actions</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{jobs.length === 0 ? (
								<TableRow>
									<TableCell colSpan={9} className="h-20 text-center">
										<FileText className="text-muted-foreground mx-auto h-8 w-8 mb-2" />
										<span className="text-muted-foreground text-sm">No export jobs yet.</span>
									</TableCell>
								</TableRow>
							) : (
								jobs.map((job) => (
									<TableRow key={`${job.node_id || "local"}-${job.id}`}>
										<TableCell className="font-mono text-xs">{job.id.replace("export_", "").slice(0, 12)}...</TableCell>
										<TableCell><Badge variant="secondary">{job.scope === "mcp_logs" ? "MCP" : "LLM"}</Badge></TableCell>
										<TableCell className="text-xs">{job.format}{job.compression ? ` + ${job.compression}` : ""}</TableCell>
										<TableCell>
											<Badge variant={statusBadgeVariant(job.status)}>{job.status}</Badge>
											{job.error && <p className="text-destructive mt-1 text-xs">{job.error}</p>}
										</TableCell>
										<TableCell>{job.rows_exported.toLocaleString()}</TableCell>
										<TableCell className="font-mono text-xs">{job.node_id || job.source || "local"}</TableCell>
										<TableCell className="text-xs">{formatRelativeTimestamp(job.created_at)}</TableCell>
										<TableCell className="text-xs">{job.completed_at ? formatRelativeTimestamp(job.completed_at) : "-"}</TableCell>
										<TableCell className="text-right">
											{job.status === "completed" && job.source !== "peer" && (
												<Button variant="ghost" size="sm" onClick={() => handleDownload(job)}><Download className="h-4 w-4" /></Button>
											)}
											{(job.status === "pending" || job.status === "running") && (
												<Loader2 className="text-muted-foreground h-4 w-4 animate-spin" />
											)}
										</TableCell>
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				</CardContent>
			</Card>

			{/* Config Sheet */}
			<ConfigSheet open={configSheetOpen} onOpenChange={setConfigSheetOpen} editing={editingConfig} />

			{/* Delete Confirmation */}
			<AlertDialog open={!!deleteConfigId} onOpenChange={(open) => { if (!open) setDeleteConfigId(null); }}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete Export Configuration</AlertDialogTitle>
						<AlertDialogDescription>This will permanently delete this export configuration and stop its scheduled runs.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction onClick={handleDeleteConfig}>Delete</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}
