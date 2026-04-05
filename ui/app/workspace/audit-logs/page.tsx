"use client";

import FullPageLoader from "@/components/fullPageLoader";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useDebouncedValue } from "@/hooks/useDebounce";
import { getApiBaseUrl } from "@/lib/utils/port";
import {
	getErrorMessage,
	useCreateLogExportMutation,
	useCreateMCPLogExportMutation,
	useGetAuditLogsQuery,
	useGetLogExportsQuery,
} from "@/lib/store";
import type { AuditCategory, ExportJob } from "@/lib/types/enterprise";
import { formatRelativeTimestamp, formatTimestamp, fromDateTimeLocalValue, isServiceDisabledError } from "@/lib/utils/enterprise";
import { AlertCircle, Download, FileOutput, RefreshCw, ScrollText } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

const auditCategories: { label: string; value: AuditCategory }[] = [
	{ label: "Authentication", value: "authentication" },
	{ label: "Configuration", value: "configuration_change" },
	{ label: "Data Access", value: "data_access" },
	{ label: "Export", value: "export" },
	{ label: "Cluster", value: "cluster" },
	{ label: "Security", value: "security_event" },
	{ label: "System", value: "system" },
];

function exportStatusVariant(status: ExportJob["status"]): "default" | "secondary" | "destructive" | "outline" {
	switch (status) {
		case "completed":
			return "default";
		case "running":
			return "secondary";
		case "failed":
			return "destructive";
		default:
			return "outline";
	}
}

export default function AuditLogsPage() {
	const [category, setCategory] = useState<AuditCategory | "all">("all");
	const [actionInput, setActionInput] = useState("");
	const [actorInput, setActorInput] = useState("");
	const [startTime, setStartTime] = useState("");
	const [endTime, setEndTime] = useState("");
	const [limit, setLimit] = useState("50");
	const [offset, setOffset] = useState(0);

	const [exportScope, setExportScope] = useState<"logs" | "mcp_logs">("logs");
	const [exportFormat, setExportFormat] = useState<"jsonl" | "csv">("jsonl");
	const [exportCompression, setExportCompression] = useState<"" | "gzip">("");
	const [exportMaxRows, setExportMaxRows] = useState("5000");

	const debouncedAction = useDebouncedValue(actionInput, 250);
	const debouncedActor = useDebouncedValue(actorInput, 250);
	const pageSize = Number(limit) || 50;

	useEffect(() => {
		setOffset(0);
	}, [category, debouncedAction, debouncedActor, startTime, endTime, pageSize]);

	const {
		data: auditData,
		error: auditError,
		isLoading: auditLoading,
		isFetching: auditFetching,
		refetch: refetchAudit,
	} = useGetAuditLogsQuery(
		{
			category: category === "all" ? "" : category,
			action: debouncedAction.trim() || undefined,
			actor_id: debouncedActor.trim() || undefined,
			start_time: fromDateTimeLocalValue(startTime),
			end_time: fromDateTimeLocalValue(endTime),
			limit: pageSize,
			offset,
		},
		{
			pollingInterval: 10000,
			skipPollingIfUnfocused: true,
		},
	);
	const {
		data: exportsData,
		error: exportsError,
		isLoading: exportsLoading,
		isFetching: exportsFetching,
		refetch: refetchExports,
	} = useGetLogExportsQuery(undefined, {
		pollingInterval: 5000,
		skipPollingIfUnfocused: true,
	});

	const [createLogExport, { isLoading: creatingLogExport }] = useCreateLogExportMutation();
	const [createMCPLogExport, { isLoading: creatingMCPLogExport }] = useCreateMCPLogExportMutation();

	const auditDisabled = isServiceDisabledError(auditError);
	const exportsDisabled = isServiceDisabledError(exportsError);

	const events = auditData?.events ?? [];
	const jobs = exportsData?.jobs ?? [];
	const completedJobs = jobs.filter((job) => job.status === "completed").length;
	const canPrevious = offset > 0;
	const canNext = offset + pageSize < (auditData?.total ?? 0);

	const summaryCards = useMemo(
		() => [
			{
				label: "Matching Audit Events",
				value: auditData?.total.toLocaleString() ?? "-",
				icon: ScrollText,
			},
			{
				label: "Visible Rows",
				value: events.length.toLocaleString(),
				icon: ScrollText,
			},
			{
				label: "Export Jobs",
				value: jobs.length.toLocaleString(),
				icon: FileOutput,
			},
			{
				label: "Completed Exports",
				value: completedJobs.toLocaleString(),
				icon: Download,
			},
		],
		[auditData?.total, completedJobs, events.length, jobs.length],
	);

	const handleCreateExport = async () => {
		const maxRows = Number(exportMaxRows);
		const payload = {
			format: exportFormat,
			compression: exportCompression || undefined,
			max_rows: Number.isFinite(maxRows) && maxRows > 0 ? maxRows : undefined,
		};

		try {
			if (exportScope === "logs") {
				await createLogExport(payload).unwrap();
			} else {
				await createMCPLogExport(payload).unwrap();
			}
			toast.success(`${exportScope === "logs" ? "Logs" : "MCP logs"} export started`);
			void refetchExports();
		} catch (error) {
			toast.error(getErrorMessage(error));
		}
	};

	if (auditLoading && !auditData && exportsLoading && !exportsData) {
		return <FullPageLoader />;
	}

	return (
		<div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h1 className="text-2xl font-semibold tracking-tight">Audit Logs</h1>
					<p className="text-muted-foreground mt-1 text-sm">
						Review compliance events and manage enterprise export jobs for logs and MCP telemetry.
					</p>
				</div>
				<Button
					variant="outline"
					onClick={() => {
						void refetchAudit();
						void refetchExports();
					}}
					isLoading={auditFetching || exportsFetching}
					dataTestId="audit-logs-refresh"
				>
					{!(auditFetching || exportsFetching) && <RefreshCw />}
					Refresh
				</Button>
			</div>

			<div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
				{summaryCards.map((card) => (
					<Card key={card.label} className="shadow-none">
						<CardContent className="flex items-start justify-between px-4 py-4">
							<div>
								<p className="text-muted-foreground text-xs">{card.label}</p>
								<p className="mt-1 text-xl font-semibold">{card.value}</p>
							</div>
							<card.icon className="text-muted-foreground mt-0.5 h-4 w-4" />
						</CardContent>
					</Card>
				))}
			</div>

			<Tabs defaultValue="audit" className="w-full">
				<TabsList className="grid w-full max-w-[360px] grid-cols-2">
					<TabsTrigger value="audit">Audit Events</TabsTrigger>
					<TabsTrigger value="exports">Log Exports</TabsTrigger>
				</TabsList>

				<TabsContent value="audit" className="mt-4">
					<Card className="shadow-none">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">Audit Event Stream</CardTitle>
						</CardHeader>
						<CardContent className="space-y-4">
							<div className="grid gap-3 lg:grid-cols-6">
								<Select value={category} onValueChange={(value) => setCategory(value as AuditCategory | "all")}>
									<SelectTrigger data-testid="audit-category-filter">
										<SelectValue placeholder="All categories" />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="all">All categories</SelectItem>
										{auditCategories.map((item) => (
											<SelectItem key={item.value} value={item.value}>
												{item.label}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<Input
									value={actionInput}
									onChange={(event) => setActionInput(event.target.value)}
									placeholder="Filter action"
									data-testid="audit-action-filter"
								/>
								<Input
									value={actorInput}
									onChange={(event) => setActorInput(event.target.value)}
									placeholder="Filter actor"
									data-testid="audit-actor-filter"
								/>
								<Input type="datetime-local" value={startTime} onChange={(event) => setStartTime(event.target.value)} />
								<Input type="datetime-local" value={endTime} onChange={(event) => setEndTime(event.target.value)} />
								<Select value={limit} onValueChange={setLimit}>
									<SelectTrigger data-testid="audit-limit-filter">
										<SelectValue placeholder="Rows" />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="25">25 rows</SelectItem>
										<SelectItem value="50">50 rows</SelectItem>
										<SelectItem value="100">100 rows</SelectItem>
									</SelectContent>
								</Select>
							</div>

							{Boolean(auditError) && !auditDisabled && (
								<Alert variant="destructive">
									<AlertCircle />
									<AlertTitle>Unable to load audit logs</AlertTitle>
									<AlertDescription>{getErrorMessage(auditError)}</AlertDescription>
								</Alert>
							)}

							{auditDisabled ? (
								<Alert variant="info">
									<AlertCircle />
									<AlertTitle>Audit logs are not enabled</AlertTitle>
									<AlertDescription>Enable `audit_logs` in the enterprise config to persist and query audit events.</AlertDescription>
								</Alert>
							) : (
								<>
									<Table containerClassName="max-h-[34rem]" data-testid="audit-events-table">
										<TableHeader>
											<TableRow>
												<TableHead>Time</TableHead>
												<TableHead>Category</TableHead>
												<TableHead>Action</TableHead>
												<TableHead>Actor</TableHead>
												<TableHead>Path</TableHead>
												<TableHead>Message</TableHead>
											</TableRow>
										</TableHeader>
										<TableBody>
											{events.length === 0 ? (
												<TableRow>
													<TableCell colSpan={6} className="h-24 text-center">
														<span className="text-muted-foreground text-sm">No audit events match the current filters.</span>
													</TableCell>
												</TableRow>
											) : (
												events.map((event) => (
													<TableRow key={event.id}>
														<TableCell>
															<div className="flex flex-col gap-0.5">
																<span>{formatTimestamp(event.timestamp)}</span>
																<span className="text-muted-foreground text-xs">{formatRelativeTimestamp(event.timestamp)}</span>
															</div>
														</TableCell>
														<TableCell>
															<Badge variant="outline">{event.category}</Badge>
														</TableCell>
														<TableCell className="font-mono text-xs">{event.action}</TableCell>
														<TableCell className="font-mono text-xs">{event.actor_id || "-"}</TableCell>
														<TableCell className="max-w-[18rem] truncate text-sm">{event.path || event.resource_type || "-"}</TableCell>
														<TableCell className="max-w-[26rem] truncate text-sm">{event.message || event.request_id || "-"}</TableCell>
													</TableRow>
												))
											)}
										</TableBody>
									</Table>

									<div className="flex items-center justify-between">
										<p className="text-muted-foreground text-sm">
											Showing {events.length === 0 ? 0 : offset + 1}-{Math.min(offset + pageSize, auditData?.total ?? 0)} of{" "}
											{(auditData?.total ?? 0).toLocaleString()} events
										</p>
										<div className="flex gap-2">
											<Button
												variant="outline"
												onClick={() => setOffset((current) => Math.max(0, current - pageSize))}
												disabled={!canPrevious}
											>
												Previous
											</Button>
											<Button variant="outline" onClick={() => setOffset((current) => current + pageSize)} disabled={!canNext}>
												Next
											</Button>
										</div>
									</div>
								</>
							)}
						</CardContent>
					</Card>
				</TabsContent>

				<TabsContent value="exports" className="mt-4">
					<div className="grid gap-4 xl:grid-cols-[0.95fr_1.4fr]">
						<Card className="shadow-none">
							<CardHeader className="pb-3">
								<CardTitle className="text-base">Create Export Job</CardTitle>
							</CardHeader>
							<CardContent className="space-y-4">
								{Boolean(exportsError) && !exportsDisabled && (
									<Alert variant="destructive">
										<AlertCircle />
										<AlertDescription>{getErrorMessage(exportsError)}</AlertDescription>
									</Alert>
								)}
								{exportsDisabled ? (
									<Alert variant="info">
										<AlertCircle />
										<AlertTitle>Log exports are not enabled</AlertTitle>
										<AlertDescription>Enable `log_exports.enabled` to generate downloadable export files.</AlertDescription>
									</Alert>
								) : (
									<>
										<div className="space-y-2">
											<p className="text-sm font-medium">Scope</p>
											<Select value={exportScope} onValueChange={(value) => setExportScope(value as "logs" | "mcp_logs")}>
												<SelectTrigger data-testid="log-export-scope">
													<SelectValue placeholder="Select scope" />
												</SelectTrigger>
												<SelectContent>
													<SelectItem value="logs">Request logs</SelectItem>
													<SelectItem value="mcp_logs">MCP logs</SelectItem>
												</SelectContent>
											</Select>
										</div>

										<div className="grid gap-4 sm:grid-cols-2">
											<div className="space-y-2">
												<p className="text-sm font-medium">Format</p>
												<Select value={exportFormat} onValueChange={(value) => setExportFormat(value as "jsonl" | "csv")}>
													<SelectTrigger data-testid="log-export-format">
														<SelectValue placeholder="Select format" />
													</SelectTrigger>
													<SelectContent>
														<SelectItem value="jsonl">JSONL</SelectItem>
														<SelectItem value="csv">CSV</SelectItem>
													</SelectContent>
												</Select>
											</div>

											<div className="space-y-2">
												<p className="text-sm font-medium">Compression</p>
												<Select
													value={exportCompression || "none"}
													onValueChange={(value) => setExportCompression(value === "none" ? "" : "gzip")}
												>
													<SelectTrigger data-testid="log-export-compression">
														<SelectValue placeholder="Select compression" />
													</SelectTrigger>
													<SelectContent>
														<SelectItem value="none">None</SelectItem>
														<SelectItem value="gzip">Gzip</SelectItem>
													</SelectContent>
												</Select>
											</div>
										</div>

										<div className="space-y-2">
											<p className="text-sm font-medium">Max Rows</p>
											<Input
												type="number"
												min="1"
												value={exportMaxRows}
												onChange={(event) => setExportMaxRows(event.target.value)}
												data-testid="log-export-max-rows"
											/>
										</div>

										<Button
											onClick={handleCreateExport}
											className="w-full"
											isLoading={creatingLogExport || creatingMCPLogExport}
											dataTestId="log-export-submit"
										>
											Create {exportScope === "logs" ? "logs" : "MCP logs"} export
										</Button>
									</>
								)}
							</CardContent>
						</Card>

						<Card className="shadow-none">
							<CardHeader className="pb-3">
								<CardTitle className="text-base">Export Jobs</CardTitle>
							</CardHeader>
							<CardContent>
								<Table containerClassName="max-h-[34rem]" data-testid="log-exports-table">
									<TableHeader>
										<TableRow>
											<TableHead>ID</TableHead>
											<TableHead>Scope</TableHead>
											<TableHead>Status</TableHead>
											<TableHead>Format</TableHead>
											<TableHead>Rows</TableHead>
											<TableHead>Created</TableHead>
											<TableHead>Completed</TableHead>
											<TableHead className="text-right">Download</TableHead>
										</TableRow>
									</TableHeader>
									<TableBody>
										{jobs.length === 0 ? (
											<TableRow>
												<TableCell colSpan={8} className="h-24 text-center">
													<span className="text-muted-foreground text-sm">No export jobs have been created yet.</span>
												</TableCell>
											</TableRow>
										) : (
											jobs.map((job) => (
												<TableRow key={job.id}>
													<TableCell className="font-mono text-xs">{job.id}</TableCell>
													<TableCell>{job.scope}</TableCell>
													<TableCell>
														<Badge variant={exportStatusVariant(job.status)}>{job.status}</Badge>
													</TableCell>
													<TableCell>{job.compression ? `${job.format} + ${job.compression}` : job.format}</TableCell>
													<TableCell>{job.rows_exported.toLocaleString()}</TableCell>
													<TableCell className="text-xs">{formatTimestamp(job.created_at)}</TableCell>
													<TableCell className="text-xs">{job.completed_at ? formatTimestamp(job.completed_at) : "-"}</TableCell>
													<TableCell className="text-right">
														{job.status === "completed" ? (
															<Button asChild variant="outline" size="sm" dataTestId={`download-export-${job.id}`}>
																<a href={`${getApiBaseUrl()}/log-exports/${job.id}/download`}>
																	<Download />
																	Download
																</a>
															</Button>
														) : (
															<span className="text-muted-foreground text-xs">{job.error || "-"}</span>
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
				</TabsContent>
			</Tabs>
		</div>
	);
}
