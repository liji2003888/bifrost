"use client";

import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useToast } from "@/hooks/use-toast";
import { useLazyGetMCPLogsQuery, useLazyGetMCPLogsStatsQuery } from "@/lib/store/apis/mcpLogsApi";
import type { MCPHostedTool } from "@/lib/types/mcp";
import type { MCPToolLogEntry, MCPToolLogStats } from "@/lib/types/logs";

interface MCPHostedToolObservabilityDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	tool: MCPHostedTool | null;
}

type TimeRange = "24h" | "7d" | "30d";

const TIME_RANGE_OPTIONS: { label: string; value: TimeRange }[] = [
	{ label: "24h", value: "24h" },
	{ label: "7d", value: "7d" },
	{ label: "30d", value: "30d" },
];

function isoRangeFor(value: TimeRange): { start_time: string; end_time: string } {
	const end = new Date();
	const start = new Date(end);
	switch (value) {
		case "7d":
			start.setDate(start.getDate() - 7);
			break;
		case "30d":
			start.setDate(start.getDate() - 30);
			break;
		default:
			start.setHours(start.getHours() - 24);
			break;
	}
	return {
		start_time: start.toISOString(),
		end_time: end.toISOString(),
	};
}

function statusBadge(status?: string) {
	switch (status) {
		case "success":
			return <Badge variant="default">Success</Badge>;
		case "error":
			return <Badge variant="destructive">Error</Badge>;
		case "processing":
			return <Badge variant="secondary">Processing</Badge>;
		default:
			return <Badge variant="outline">{status || "Unknown"}</Badge>;
	}
}

function formatTimestamp(value?: string) {
	if (!value) return "-";
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return date.toLocaleString();
}

function getErrorMessage(entry: MCPToolLogEntry) {
	return entry.error_details?.error?.message || "-";
}

function getVirtualKeyLabel(entry: MCPToolLogEntry) {
	if (entry.virtual_key?.name) return entry.virtual_key.name;
	if (entry.virtual_key_name) return entry.virtual_key_name;
	return "-";
}

export function MCPHostedToolObservabilityDialog({ open, onOpenChange, tool }: MCPHostedToolObservabilityDialogProps) {
	const { toast } = useToast();
	const [timeRange, setTimeRange] = useState<TimeRange>("24h");
	const [logs, setLogs] = useState<MCPToolLogEntry[]>([]);
	const [stats, setStats] = useState<MCPToolLogStats | null>(null);
	const [isRefreshing, setIsRefreshing] = useState(false);
	const [triggerLogs] = useLazyGetMCPLogsQuery();
	const [triggerStats] = useLazyGetMCPLogsStatsQuery();

	const responseExamples = useMemo(() => {
		if (!tool?.response_examples || tool.response_examples.length === 0) {
			return [];
		}
		return tool.response_examples.slice(0, 3);
	}, [tool]);

	const refresh = async () => {
		if (!tool?.name) {
			return;
		}
		setIsRefreshing(true);
		const range = isoRangeFor(timeRange);
		try {
			const [logsResponse, statsResponse] = await Promise.all([
				triggerLogs({
					filters: {
						tool_names: [tool.name],
						start_time: range.start_time,
						end_time: range.end_time,
					},
					pagination: {
						limit: 10,
						offset: 0,
						sort_by: "timestamp",
						order: "desc",
					},
				}).unwrap(),
				triggerStats({
					filters: {
						tool_names: [tool.name],
						start_time: range.start_time,
						end_time: range.end_time,
					},
				}).unwrap(),
			]);
			setLogs(logsResponse.logs || []);
			setStats(statsResponse);
		} catch (error: any) {
			toast({
				title: "Failed to load observability data",
				description: error?.data?.error?.message || "Unable to fetch recent hosted tool logs.",
				variant: "destructive",
			});
		} finally {
			setIsRefreshing(false);
		}
	};

	useEffect(() => {
		if (!open) {
			return;
		}
		void refresh();
	}, [open, timeRange, tool?.name]);

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-h-[90vh] max-w-6xl overflow-y-auto">
				<DialogHeader>
					<DialogTitle>Hosted Tool Observability</DialogTitle>
					<DialogDescription>
						Cluster-safe observability for this hosted tool using the existing MCP log store. No extra worker or polling loop is added to the inference hot path.
					</DialogDescription>
				</DialogHeader>

				<div className="space-y-4">
					<div className="flex flex-wrap items-center justify-between gap-3">
						<div>
							<div className="text-sm font-medium">{tool?.name || "Unknown tool"}</div>
							<div className="text-muted-foreground font-mono text-xs">
								{tool?.method} {tool?.url}
							</div>
						</div>
						<div className="flex items-center gap-2">
							{TIME_RANGE_OPTIONS.map((option) => (
								<Button
									key={option.value}
									type="button"
									size="sm"
									variant={timeRange === option.value ? "default" : "outline"}
									onClick={() => setTimeRange(option.value)}
								>
									{option.label}
								</Button>
							))}
							<Button type="button" size="sm" variant="outline" onClick={() => void refresh()} disabled={isRefreshing}>
								{isRefreshing ? "Refreshing..." : "Refresh"}
							</Button>
						</div>
					</div>

					<div className="grid gap-3 md:grid-cols-4">
						<div className="rounded-md border p-3">
							<div className="text-muted-foreground text-xs">Total Calls</div>
							<div className="mt-2 text-xl font-semibold">{stats?.total_executions ?? 0}</div>
						</div>
						<div className="rounded-md border p-3">
							<div className="text-muted-foreground text-xs">Success Rate</div>
							<div className="mt-2 text-xl font-semibold">{stats ? `${stats.success_rate.toFixed(1)}%` : "-"}</div>
						</div>
						<div className="rounded-md border p-3">
							<div className="text-muted-foreground text-xs">Average Latency</div>
							<div className="mt-2 text-xl font-semibold">{stats ? `${stats.average_latency.toFixed(1)} ms` : "-"}</div>
						</div>
						<div className="rounded-md border p-3">
							<div className="text-muted-foreground text-xs">Total Cost</div>
							<div className="mt-2 text-xl font-semibold">{stats ? `$${stats.total_cost.toFixed(6)}` : "-"}</div>
						</div>
					</div>

					<div className="grid gap-4 lg:grid-cols-2">
						<div className="space-y-2">
							<Label>Response Examples</Label>
							<div className="rounded-md border p-3">
								{responseExamples.length === 0 ? (
									<p className="text-muted-foreground text-sm">No response examples configured yet.</p>
								) : (
									<div className="space-y-3">
										{responseExamples.map((example, index) => (
											<div key={index} className="rounded-md border p-3">
												<div className="text-muted-foreground mb-2 text-xs font-medium">Example {index + 1}</div>
												<pre className="max-h-[180px] overflow-auto whitespace-pre-wrap break-words font-mono text-xs">
													{JSON.stringify(example, null, 2)}
												</pre>
											</div>
										))}
									</div>
								)}
							</div>
						</div>

						<div className="space-y-2">
							<Label>Response Schema</Label>
							<div className="rounded-md border p-3">
								<pre className="max-h-[260px] overflow-auto whitespace-pre-wrap break-words font-mono text-xs">
									{JSON.stringify(tool?.response_schema || {}, null, 2) || "{}"}
								</pre>
							</div>
						</div>
					</div>

					<div className="space-y-2">
						<Label>Recent Calls</Label>
						<div className="overflow-hidden rounded-md border">
							<Table>
								<TableHeader>
									<TableRow className="bg-muted/50">
										<TableHead>Time</TableHead>
										<TableHead>Status</TableHead>
										<TableHead>Latency</TableHead>
										<TableHead>Virtual Key</TableHead>
										<TableHead>LLM Request</TableHead>
										<TableHead>Error</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{logs.length === 0 ? (
										<TableRow>
											<TableCell colSpan={6} className="text-muted-foreground h-20 text-center">
												No recent calls for the selected time range.
											</TableCell>
										</TableRow>
									) : (
										logs.map((entry) => (
											<TableRow key={entry.id}>
												<TableCell className="text-xs">{formatTimestamp(entry.timestamp)}</TableCell>
												<TableCell>{statusBadge(entry.status)}</TableCell>
												<TableCell className="text-xs">{entry.latency ? `${entry.latency} ms` : "-"}</TableCell>
												<TableCell className="text-xs">{getVirtualKeyLabel(entry)}</TableCell>
												<TableCell className="font-mono text-xs">{entry.llm_request_id || "-"}</TableCell>
												<TableCell className="max-w-[260px] truncate text-xs">{getErrorMessage(entry)}</TableCell>
											</TableRow>
										))
									)}
								</TableBody>
							</Table>
						</div>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}
