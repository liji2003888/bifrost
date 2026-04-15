"use client";

import FullPageLoader from "@/components/fullPageLoader";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import { useDebouncedValue } from "@/hooks/useDebounce";
import { getErrorMessage } from "@/lib/store";
import {
	useCompleteOAuthFlowMutation,
	useGetMCPAuthConfigsQuery,
	useRevokeOAuthConfigMutation,
} from "@/lib/store/apis/mcpApi";
import type { MCPAuthConfigRecord } from "@/lib/types/mcp";
import { formatRelativeTimestamp } from "@/lib/utils/enterprise";
import { AlertCircle, CheckCircle2, Copy, ExternalLink, Link2, RefreshCw, Search, ShieldUser, Unplug } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

const PAGE_SIZE = 25;

function statusBadgeVariant(status: string): "default" | "secondary" | "destructive" | "outline" {
	switch (status) {
		case "authorized":
			return "default";
		case "pending":
			return "outline";
		case "revoked":
			return "secondary";
		case "failed":
		case "expired":
			return "destructive";
		default:
			return "secondary";
	}
}

function connectionBadgeVariant(state?: string): "default" | "secondary" | "destructive" | "outline" {
	switch (state) {
		case "connected":
			return "default";
		case "disconnected":
			return "secondary";
		case "error":
			return "destructive";
		default:
			return "outline";
	}
}

function SummaryCard({ title, value, subtitle }: { title: string; value: string | number; subtitle: string }) {
	return (
		<Card className="shadow-none">
			<CardHeader className="gap-1 pb-2">
				<CardTitle className="text-sm font-medium">{title}</CardTitle>
			</CardHeader>
			<CardContent className="space-y-1">
				<p className="text-2xl font-semibold tracking-tight">{value}</p>
				<p className="text-muted-foreground text-xs">{subtitle}</p>
			</CardContent>
		</Card>
	);
}

function OAuthConfigActions({
	record,
	onComplete,
	onConfirmRevoke,
	onCopy,
	onCopyStatusUrl,
	onCopyCompleteUrl,
	actionBusy,
}: {
	record: MCPAuthConfigRecord;
	onComplete: (id: string) => Promise<void>;
	onConfirmRevoke: (record: MCPAuthConfigRecord) => void;
	onCopy: (value: string) => Promise<void>;
	onCopyStatusUrl: (value: string) => Promise<void>;
	onCopyCompleteUrl: (value: string) => Promise<void>;
	actionBusy: boolean;
}) {
	const canComplete = record.status === "authorized" && !!record.pending_mcp_client && !record.linked_mcp_client;
	const canRevoke = record.status !== "revoked";
	const canOpenAuthorize = record.status === "pending" && !!record.authorize_url;

	return (
		<div className="flex flex-wrap items-center justify-end gap-2">
			<Button
				variant="outline"
				size="sm"
				onClick={() => void onCopy(record.id)}
				dataTestId={`oauth-config-copy-${record.id}`}
			>
				<Copy className="h-4 w-4" />
				Copy ID
			</Button>
				{record.status_url && (
					<Button variant="outline" size="sm" onClick={() => void onCopyStatusUrl(record.status_url)} dataTestId={`oauth-config-copy-status-${record.id}`}>
						<Link2 className="h-4 w-4" />
						Copy Status URL
					</Button>
				)}
				{canOpenAuthorize && (
					<Button asChild variant="outline" size="sm">
						<a href={record.authorize_url} target="_blank" rel="noreferrer" data-testid={`oauth-config-open-auth-${record.id}`}>
							<ExternalLink className="h-4 w-4" />
							Open Auth
						</a>
					</Button>
				)}
				{canComplete && (
					<>
						{record.complete_url && (
							<Button
								variant="outline"
								size="sm"
								onClick={() => void onCopyCompleteUrl(record.complete_url)}
								dataTestId={`oauth-config-copy-complete-${record.id}`}
							>
								<Copy className="h-4 w-4" />
								Copy Complete URL
							</Button>
					)}
					<Button
						variant="outline"
						size="sm"
						onClick={() => void onComplete(record.id)}
						disabled={actionBusy}
						isLoading={actionBusy}
						dataTestId={`oauth-config-complete-${record.id}`}
					>
						<CheckCircle2 className="h-4 w-4" />
						Complete OAuth
					</Button>
				</>
			)}
			{canRevoke && (
				<Button
					variant="destructive"
					size="sm"
					onClick={() => onConfirmRevoke(record)}
					disabled={actionBusy}
					dataTestId={`oauth-config-revoke-${record.id}`}
				>
					<Unplug className="h-4 w-4" />
					{record.status === "pending" ? "Cancel" : "Revoke"}
				</Button>
			)}
		</div>
	);
}

export default function MCPAuthConfigView() {
	const [search, setSearch] = useState("");
	const [statusFilter, setStatusFilter] = useState("all");
	const [offset, setOffset] = useState(0);
	const [confirmRevoke, setConfirmRevoke] = useState<MCPAuthConfigRecord | null>(null);
	const debouncedSearch = useDebouncedValue(search, 300);
	const { copy } = useCopyToClipboard({ successMessage: "OAuth config ID copied" });

	useEffect(() => {
		setOffset(0);
	}, [debouncedSearch, statusFilter]);

	const {
		data,
		error,
		isLoading,
		isFetching,
		refetch,
	} = useGetMCPAuthConfigsQuery(
		{
			limit: PAGE_SIZE,
			offset,
			search: debouncedSearch || undefined,
			status: statusFilter === "all" ? undefined : statusFilter,
		},
		{
			pollingInterval: 5000,
			skipPollingIfUnfocused: true,
		},
	);
	const [revokeOAuthConfig, { isLoading: revoking }] = useRevokeOAuthConfigMutation();
	const [completeOAuthFlow, { isLoading: completing }] = useCompleteOAuthFlowMutation();

	const configs = data?.configs ?? [];
	const totalCount = data?.total_count ?? 0;
	const now = Date.now();

	const summary = useMemo(() => {
		let authorized = 0;
		let pending = 0;
		let expiringSoon = 0;
		for (const record of configs) {
			if (record.status === "authorized") {
				authorized += 1;
			}
			if (record.status === "pending") {
				pending += 1;
			}
			if (record.token_expires_at) {
				const expiry = new Date(record.token_expires_at).getTime();
				if (!Number.isNaN(expiry) && expiry > now && expiry-now <= 15*60*1000) {
					expiringSoon += 1;
				}
			}
		}
		return { authorized, pending, expiringSoon };
	}, [configs, now]);

	const handleComplete = async (oauthConfigId: string) => {
		try {
			await completeOAuthFlow(oauthConfigId).unwrap();
			toast.success("MCP OAuth authorization completed");
			void refetch();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	const handleRevoke = async () => {
		if (!confirmRevoke) {
			return;
		}
		try {
			await revokeOAuthConfig(confirmRevoke.id).unwrap();
			toast.success(confirmRevoke.status === "pending" ? "OAuth config cancelled" : "OAuth config revoked");
			setConfirmRevoke(null);
			void refetch();
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	const handleCopyStatusURL = async (statusUrl: string) => {
		await copy(statusUrl);
	};

	const handleCopyCompleteURL = async (completeUrl: string) => {
		await copy(completeUrl);
	};

	const pageStart = totalCount === 0 ? 0 : offset + 1;
	const pageEnd = Math.min(offset + configs.length, totalCount);
	const hasPrev = offset > 0;
	const hasNext = offset + PAGE_SIZE < totalCount;

	if (isLoading) {
		return <FullPageLoader />;
	}

	return (
		<div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h1 className="text-2xl font-semibold tracking-tight">MCP Auth Config</h1>
					<p className="text-muted-foreground mt-1 text-sm">
						集中查看 OAuth MCP server 的授权状态、待完成配置和已绑定的 MCP 连接。状态来自共享 ConfigStore，集群内任一节点完成授权后会自动同步。
					</p>
				</div>
				<div className="flex items-center gap-2">
					<Button variant="outline" onClick={() => void refetch()} isLoading={isFetching} dataTestId="mcp-auth-config-refresh">
						{!isFetching && <RefreshCw className="h-4 w-4" />}
						Refresh
					</Button>
					<Button asChild>
						<Link href="/workspace/mcp-registry">
							<ExternalLink className="h-4 w-4" />
							Open MCP Registry
						</Link>
					</Button>
				</div>
			</div>

			<Alert variant="info">
				<ShieldUser />
				<AlertTitle>当前页负责管理 OAuth MCP 授权配置</AlertTitle>
				<AlertDescription>
					新的 OAuth MCP Server 仍然从 <strong>MCP Registry</strong> 创建；这里负责查看授权状态、取消挂起授权、撤销已授权配置，以及在授权已完成但 MCP Server 尚未落库时执行 <strong>Complete OAuth</strong>。
				</AlertDescription>
			</Alert>

			{Boolean(error) && (
				<Alert variant="destructive">
					<AlertCircle />
					<AlertTitle>Unable to load MCP auth configs</AlertTitle>
					<AlertDescription>{getErrorMessage(error)}</AlertDescription>
				</Alert>
			)}

			<div className="grid gap-4 md:grid-cols-4">
				<SummaryCard title="OAuth Configs" value={totalCount} subtitle="Current page scope" />
				<SummaryCard title="Authorized" value={summary.authorized} subtitle="Ready for MCP server access" />
				<SummaryCard title="Pending" value={summary.pending} subtitle="Waiting for OAuth completion" />
				<SummaryCard title="Token Expiring Soon" value={summary.expiringSoon} subtitle="Within the next 15 minutes" />
			</div>

			<Card className="shadow-none">
				<CardHeader className="gap-4">
					<CardTitle>OAuth MCP Configurations</CardTitle>
					<div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
						<div className="relative w-full md:max-w-sm">
							<Search className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
							<Input
								value={search}
								onChange={(e) => setSearch(e.target.value)}
								placeholder="Search by client ID, server URL, or endpoint"
								className="pl-9"
								data-testid="mcp-auth-config-search"
							/>
						</div>
						<div className="flex items-center gap-2">
							<Select value={statusFilter} onValueChange={setStatusFilter}>
								<SelectTrigger className="w-[180px]" data-testid="mcp-auth-config-status-filter">
									<SelectValue placeholder="All statuses" />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="all">All statuses</SelectItem>
									<SelectItem value="pending">Pending</SelectItem>
									<SelectItem value="authorized">Authorized</SelectItem>
									<SelectItem value="failed">Failed</SelectItem>
									<SelectItem value="expired">Expired</SelectItem>
									<SelectItem value="revoked">Revoked</SelectItem>
								</SelectContent>
							</Select>
						</div>
					</div>
				</CardHeader>
				<CardContent className="space-y-4">
					{configs.length === 0 ? (
						<div className="text-muted-foreground flex min-h-[220px] flex-col items-center justify-center gap-3 text-center">
							<Link2 className="h-10 w-10" />
							<div className="space-y-1">
								<p className="text-foreground font-medium">No MCP auth configs found</p>
								<p className="text-sm">在 MCP Registry 中创建使用 OAuth 的 MCP Server 后，这里会显示授权状态和后续管理动作。</p>
							</div>
						</div>
					) : (
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>OAuth Config</TableHead>
									<TableHead>MCP Server</TableHead>
									<TableHead>MCP Client</TableHead>
									<TableHead>Status</TableHead>
									<TableHead>Token</TableHead>
									<TableHead>Last Updated</TableHead>
									<TableHead className="text-right">Actions</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{configs.map((record) => (
									<TableRow key={record.id} data-testid={`mcp-auth-config-row-${record.id}`}>
										<TableCell className="max-w-[260px] whitespace-normal">
											<div className="space-y-1">
												<div className="flex flex-wrap items-center gap-2">
													<p className="font-medium">{record.client_id || "Dynamic registration"}</p>
													<Badge variant={record.use_discovery ? "outline" : "secondary"}>
														{record.use_discovery ? "Discovery" : "Manual"}
													</Badge>
												</div>
												<p className="text-muted-foreground break-all font-mono text-xs">{record.id}</p>
												{record.scopes && record.scopes.length > 0 && (
													<p className="text-muted-foreground text-xs">Scopes: {record.scopes.join(", ")}</p>
												)}
											</div>
										</TableCell>
										<TableCell className="max-w-[320px] whitespace-normal">
											<div className="space-y-1">
												<p className="break-all text-sm">{record.server_url || record.redirect_uri}</p>
												<p className="text-muted-foreground break-all text-xs">{record.authorize_url}</p>
											</div>
										</TableCell>
										<TableCell className="max-w-[280px] whitespace-normal">
											<div className="space-y-2">
												{record.linked_mcp_client ? (
													<div className="space-y-1">
														<div className="flex flex-wrap items-center gap-2">
															<p className="font-medium">{record.linked_mcp_client.name}</p>
															<Badge variant={connectionBadgeVariant(record.linked_mcp_client.state)}>
																{record.linked_mcp_client.state || "configured"}
															</Badge>
														</div>
														<p className="text-muted-foreground font-mono text-xs">{record.linked_mcp_client.client_id}</p>
													</div>
												) : record.pending_mcp_client ? (
													<div className="space-y-1">
														<div className="flex flex-wrap items-center gap-2">
															<p className="font-medium">{record.pending_mcp_client.name}</p>
															<Badge variant="outline">Pending attach</Badge>
														</div>
														<p className="text-muted-foreground font-mono text-xs">{record.pending_mcp_client.client_id}</p>
													</div>
												) : (
													<p className="text-muted-foreground text-sm">Not linked to a saved MCP client</p>
												)}
												{record.next_steps && record.next_steps.length > 0 && (
													<ol className="text-muted-foreground list-decimal space-y-1 pl-4 text-xs">
														{record.next_steps.map((step) => (
															<li key={step}>{step}</li>
														))}
													</ol>
												)}
											</div>
										</TableCell>
										<TableCell>
											<div className="space-y-2">
												<Badge variant={statusBadgeVariant(record.status)}>{record.status}</Badge>
												<p className="text-muted-foreground text-xs">Expires {formatRelativeTimestamp(record.expires_at)}</p>
											</div>
										</TableCell>
										<TableCell className="max-w-[220px] whitespace-normal">
											{record.token_expires_at ? (
												<div className="space-y-1">
													<p className="text-sm">Expires {formatRelativeTimestamp(record.token_expires_at)}</p>
													{record.token_scopes && record.token_scopes.length > 0 && (
														<p className="text-muted-foreground text-xs">{record.token_scopes.join(", ")}</p>
													)}
												</div>
											) : (
												<p className="text-muted-foreground text-sm">No linked token</p>
											)}
										</TableCell>
										<TableCell>
											<div className="space-y-1">
												<p className="text-sm">{new Date(record.updated_at).toLocaleString()}</p>
												<p className="text-muted-foreground text-xs">{formatRelativeTimestamp(record.updated_at)}</p>
											</div>
										</TableCell>
										<TableCell className="text-right">
												<OAuthConfigActions
													record={record}
													onComplete={handleComplete}
													onConfirmRevoke={setConfirmRevoke}
													onCopy={copy}
													onCopyStatusUrl={handleCopyStatusURL}
													onCopyCompleteUrl={handleCopyCompleteURL}
													actionBusy={revoking || completing}
												/>
										</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					)}

					<div className="flex flex-wrap items-center justify-between gap-3">
						<p className="text-muted-foreground text-sm">
							{totalCount === 0 ? "No results" : `Showing ${pageStart}-${pageEnd} of ${totalCount}`}
						</p>
						<div className="flex items-center gap-2">
							<Button variant="outline" onClick={() => setOffset((prev) => Math.max(0, prev - PAGE_SIZE))} disabled={!hasPrev}>
								Previous
							</Button>
							<Button variant="outline" onClick={() => setOffset((prev) => prev + PAGE_SIZE)} disabled={!hasNext}>
								Next
							</Button>
						</div>
					</div>
				</CardContent>
			</Card>

			<AlertDialog open={!!confirmRevoke} onOpenChange={(open) => !open && setConfirmRevoke(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{confirmRevoke?.status === "pending" ? "Cancel pending OAuth config?" : "Revoke OAuth config?"}</AlertDialogTitle>
						<AlertDialogDescription>
							{confirmRevoke?.status === "pending"
								? "This will cancel the pending OAuth authorization and clear the waiting MCP client attachment."
								: "This will revoke the linked OAuth token, mark the auth config as revoked, and sync the change across the cluster."}
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Keep config</AlertDialogCancel>
						<AlertDialogAction onClick={() => void handleRevoke()} disabled={revoking}>
							{revoking ? "Processing..." : confirmRevoke?.status === "pending" ? "Cancel OAuth" : "Revoke OAuth"}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}
