"use client";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
	useGetLogExportsQuery,
	useCreateLogExportMutation,
	useCreateMCPLogExportMutation,
} from "@/lib/store/apis/enterpriseApi";
import { getErrorMessage } from "@/lib/store";
import type { ExportJob, ExportJobStatus, ExportScope } from "@/lib/types/enterprise";
import { formatRelativeTimestamp } from "@/lib/utils/enterprise";
import { AlertCircle, Download, FileText, Loader2, Plus, RefreshCw } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

function statusBadgeVariant(status: ExportJobStatus): "default" | "secondary" | "destructive" | "outline" {
	switch (status) {
		case "completed":
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

export default function LogExportsPage() {
	const [exportScope, setExportScope] = useState<ExportScope>("logs");
	const [exportFormat, setExportFormat] = useState<"jsonl" | "csv">("jsonl");
	const [compression, setCompression] = useState<"" | "gzip">("");
	const [maxRows, setMaxRows] = useState(10000);

	const {
		data: exportsResponse,
		error: exportsError,
		isFetching,
		refetch,
	} = useGetLogExportsQuery(
		{ cluster: true },
		{ pollingInterval: 10000, skipPollingIfUnfocused: true },
	);

	const [createLogExport, { isLoading: isCreatingLog }] = useCreateLogExportMutation();
	const [createMCPLogExport, { isLoading: isCreatingMCP }] = useCreateMCPLogExportMutation();
	const isCreating = isCreatingLog || isCreatingMCP;

	const jobs = exportsResponse?.jobs ?? [];

	const handleExport = async () => {
		try {
			const payload = {
				format: exportFormat,
				compression: compression || undefined,
				max_rows: maxRows > 0 ? maxRows : undefined,
			};
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
		// Use the API base URL for download
		const baseUrl = typeof window !== "undefined" ? `${window.location.origin}/api` : "/api";
		window.open(`${baseUrl}/log-exports/${job.id}/download`, "_blank");
	};

	return (
		<div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h1 className="text-2xl font-semibold tracking-tight">Log Exports</h1>
					<p className="text-muted-foreground mt-1 text-sm">
						Export request logs and MCP tool logs in JSONL or CSV format with optional gzip compression.
					</p>
				</div>
				<Button
					variant="outline"
					onClick={() => void refetch()}
					isLoading={isFetching}
					dataTestId="log-exports-refresh"
				>
					{!isFetching && <RefreshCw className="h-4 w-4" />}
					Refresh
				</Button>
			</div>

			{Boolean(exportsError) && (
				<Alert variant="destructive">
					<AlertCircle />
					<AlertTitle>Unable to load export jobs</AlertTitle>
					<AlertDescription>{getErrorMessage(exportsError)}</AlertDescription>
				</Alert>
			)}

			{/* New Export Form */}
			<Card className="shadow-none">
				<CardHeader className="pb-3">
					<CardTitle className="text-base">Create New Export</CardTitle>
				</CardHeader>
				<CardContent className="space-y-4">
					<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
						<div className="space-y-2">
							<Label>Scope</Label>
							<Select value={exportScope} onValueChange={(v) => setExportScope(v as ExportScope)}>
								<SelectTrigger>
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="logs">LLM Logs</SelectItem>
									<SelectItem value="mcp_logs">MCP Logs</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div className="space-y-2">
							<Label>Format</Label>
							<Select value={exportFormat} onValueChange={(v) => setExportFormat(v as "jsonl" | "csv")}>
								<SelectTrigger>
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="jsonl">JSONL</SelectItem>
									<SelectItem value="csv">CSV</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div className="space-y-2">
							<Label>Compression</Label>
							<Select value={compression || "none"} onValueChange={(v) => setCompression(v === "none" ? "" : "gzip")}>
								<SelectTrigger>
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="none">None</SelectItem>
									<SelectItem value="gzip">Gzip</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div className="space-y-2">
							<Label>Max Rows</Label>
							<Input
								type="number"
								value={maxRows}
								min={1}
								max={1000000}
								onChange={(e) => setMaxRows(parseInt(e.target.value) || 10000)}
							/>
						</div>
					</div>
					<div className="flex justify-end">
						<Button onClick={() => void handleExport()} disabled={isCreating}>
							{isCreating ? (
								<Loader2 className="h-4 w-4 animate-spin" />
							) : (
								<Plus className="h-4 w-4" />
							)}
							Start Export
						</Button>
					</div>
				</CardContent>
			</Card>

			{/* Export Jobs List */}
			<Card className="shadow-none">
				<CardHeader className="pb-3">
					<CardTitle className="text-base">Export History</CardTitle>
				</CardHeader>
				<CardContent>
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
									<TableCell colSpan={9} className="h-24 text-center">
										<div className="flex flex-col items-center gap-2">
											<FileText className="text-muted-foreground h-8 w-8" />
											<span className="text-muted-foreground text-sm">No export jobs yet. Create one above.</span>
										</div>
									</TableCell>
								</TableRow>
							) : (
								jobs.map((job) => (
									<TableRow key={`${job.node_id || "local"}-${job.id}`}>
										<TableCell className="font-mono text-xs">{job.id.replace("export_", "").slice(0, 12)}...</TableCell>
										<TableCell>
											<Badge variant="secondary">{job.scope === "mcp_logs" ? "MCP" : "LLM"}</Badge>
										</TableCell>
										<TableCell className="text-xs">
											{job.format}{job.compression ? ` + ${job.compression}` : ""}
										</TableCell>
										<TableCell>
											<Badge variant={statusBadgeVariant(job.status)}>{job.status}</Badge>
											{job.error && (
												<p className="text-destructive mt-1 text-xs">{job.error}</p>
											)}
										</TableCell>
										<TableCell>{job.rows_exported.toLocaleString()}</TableCell>
										<TableCell className="font-mono text-xs">{job.node_id || job.source || "local"}</TableCell>
										<TableCell className="text-xs">{formatRelativeTimestamp(job.created_at)}</TableCell>
										<TableCell className="text-xs">{job.completed_at ? formatRelativeTimestamp(job.completed_at) : "-"}</TableCell>
										<TableCell className="text-right">
											{job.status === "completed" && job.source !== "peer" && (
												<Button
													variant="ghost"
													size="sm"
													onClick={() => handleDownload(job)}
													aria-label="Download export"
												>
													<Download className="h-4 w-4" />
												</Button>
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
		</div>
	);
}
