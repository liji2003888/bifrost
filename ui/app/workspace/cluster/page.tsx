"use client";

import FullPageLoader from "@/components/fullPageLoader";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { getErrorMessage, useGetClusterStatusQuery, useGetVaultStatusQuery } from "@/lib/store";
import { formatRelativeTimestamp, formatTimestamp, isServiceDisabledError } from "@/lib/utils/enterprise";
import { AlertCircle, Database, GitBranch, KeyRound, RefreshCw, Server, ShieldCheck } from "lucide-react";
import type { ReactNode } from "react";
import { useMemo } from "react";

function HealthBadge({ healthy }: { healthy: boolean }) {
	return healthy ? <Badge>Healthy</Badge> : <Badge variant="destructive">Degraded</Badge>;
}

export default function ClusterPage() {
	const {
		data: clusterStatus,
		error: clusterError,
		isLoading: clusterLoading,
		isFetching: clusterFetching,
		refetch: refetchCluster,
	} = useGetClusterStatusQuery(undefined, {
		pollingInterval: 5000,
		skipPollingIfUnfocused: true,
	});
	const {
		data: vaultStatus,
		error: vaultError,
		isLoading: vaultLoading,
		isFetching: vaultFetching,
		refetch: refetchVault,
	} = useGetVaultStatusQuery(undefined, {
		pollingInterval: 15000,
		skipPollingIfUnfocused: true,
	});

	const clusterServiceDisabled = isServiceDisabledError(clusterError);
	const vaultServiceDisabled = isServiceDisabledError(vaultError);
	const localConfigSync = clusterStatus?.config_sync;

	const peerRuntimeDriftCount = useMemo(() => {
		if (!clusterStatus?.peers?.length || !localConfigSync?.runtime_hash) {
			return 0;
		}
		return clusterStatus.peers.filter((peer) => peer.config_sync?.runtime_hash && peer.config_sync.runtime_hash !== localConfigSync.runtime_hash).length;
	}, [clusterStatus?.peers, localConfigSync?.runtime_hash]);

	const peersOutOfSyncWithStoreCount = useMemo(() => {
		if (!clusterStatus?.peers?.length) {
			return 0;
		}
		return clusterStatus.peers.filter((peer) => peer.config_sync?.store_connected && peer.config_sync?.in_sync === false).length;
	}, [clusterStatus?.peers]);

	const summaryCards = useMemo(() => {
		if (!clusterStatus) {
			return [];
		}
		return [
			{
				label: "Cluster Health",
				value: clusterStatus.healthy ? "Healthy" : "Degraded",
				icon: ShieldCheck,
			},
			{
				label: "Peers",
				value: clusterStatus.peers.length.toLocaleString(),
				icon: Server,
			},
			{
				label: "Replicated KV Keys",
				value: clusterStatus.kv_keys.toLocaleString(),
				icon: Database,
			},
			{
				label: "Discovered Peers",
				value: (clusterStatus.discovery?.peer_count ?? 0).toLocaleString(),
				icon: GitBranch,
			},
			{
				label: "Runtime Drift",
				value: peerRuntimeDriftCount.toLocaleString(),
				icon: RefreshCw,
			},
			{
				label: "Managed Secrets",
				value: vaultStatus ? vaultStatus.managed_secrets.toLocaleString() : vaultServiceDisabled ? "Disabled" : "-",
				icon: KeyRound,
			},
		];
	}, [clusterStatus, peerRuntimeDriftCount, vaultStatus, vaultServiceDisabled]);

	if (clusterLoading && !clusterStatus) {
		return <FullPageLoader />;
	}

	return (
		<div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h1 className="text-2xl font-semibold tracking-tight">Cluster Mode</h1>
					<p className="text-muted-foreground mt-1 text-sm">
						Monitor node health, peer replication, discovery state, and Vault-backed runtime secret sync.
					</p>
				</div>
				<Button
					variant="outline"
					onClick={() => {
						void refetchCluster();
						void refetchVault();
					}}
					isLoading={clusterFetching || vaultFetching}
					dataTestId="cluster-refresh"
				>
					{!(clusterFetching || vaultFetching) && <RefreshCw />}
					Refresh
				</Button>
			</div>

			{Boolean(clusterError) && !clusterServiceDisabled && (
				<Alert variant="destructive">
					<AlertCircle />
					<AlertTitle>Unable to load cluster status</AlertTitle>
					<AlertDescription>{getErrorMessage(clusterError)}</AlertDescription>
				</Alert>
			)}

			{clusterServiceDisabled && (
				<Alert variant="info">
					<AlertCircle />
					<AlertTitle>Cluster mode is not enabled</AlertTitle>
					<AlertDescription>Enable `cluster_config.enabled` to activate peer health, replication, and discovery status.</AlertDescription>
				</Alert>
			)}

			{clusterStatus && (
				<>
					{localConfigSync?.store_connected && localConfigSync.in_sync === false && (
						<Alert variant="info">
							<AlertCircle />
							<AlertTitle>Current node runtime is behind ConfigStore</AlertTitle>
							<AlertDescription>
								This node is still serving a different runtime config than the persisted ConfigStore state. Cluster mode currently replicates KV state,
								not generic config hot reloads, so peer nodes can drift until they reload the changed config.
							</AlertDescription>
						</Alert>
					)}

					{peerRuntimeDriftCount > 0 && (
						<Alert variant="info">
							<AlertCircle />
							<AlertTitle>Runtime drift detected across the cluster</AlertTitle>
							<AlertDescription>
								{peerRuntimeDriftCount} peer node{peerRuntimeDriftCount === 1 ? "" : "s"} currently expose a different runtime config fingerprint than this
								node.
							</AlertDescription>
						</Alert>
					)}

					{peersOutOfSyncWithStoreCount > 0 && (
						<Alert variant="info">
							<AlertCircle />
							<AlertTitle>Some peer nodes are not reloaded from ConfigStore</AlertTitle>
							<AlertDescription>
								{peersOutOfSyncWithStoreCount} peer node{peersOutOfSyncWithStoreCount === 1 ? "" : "s"} report runtime drift from their own
								ConfigStore snapshot.
							</AlertDescription>
						</Alert>
					)}

					<div className="grid gap-4 md:grid-cols-2 xl:grid-cols-6">
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

					<div className="grid gap-4 lg:grid-cols-[1.6fr_1fr]">
						<Card className="shadow-none">
							<CardHeader className="pb-3">
								<CardTitle className="text-base">Node Overview</CardTitle>
							</CardHeader>
							<CardContent className="grid gap-4 sm:grid-cols-2">
								<InfoPair label="Node ID" value={clusterStatus.node_id} mono />
								<InfoPair label="Started" value={formatTimestamp(clusterStatus.started_at)} />
								<InfoPair label="Status" value={<HealthBadge healthy={clusterStatus.healthy} />} />
								<InfoPair
									label="Config Sync"
									value={
										!localConfigSync?.store_connected
											? "No ConfigStore"
											: localConfigSync.in_sync === false
												? "Runtime drift"
												: "In sync"
									}
								/>
								<InfoPair
									label="Discovery"
									value={
										clusterStatus.discovery?.enabled
											? `${clusterStatus.discovery.type || "enabled"} (${clusterStatus.discovery.peer_count} peers)`
											: "Disabled"
									}
								/>
								<InfoPair label="Config Store" value={localConfigSync?.store_connected ? localConfigSync.store_kind || "connected" : "Disabled"} />
								<InfoPair
									label="Tracked Resources"
									value={`${localConfigSync?.provider_count ?? 0} providers · ${localConfigSync?.virtual_key_count ?? 0} virtual keys`}
								/>
								<InfoPair
									label="Runtime Fingerprint"
									value={localConfigSync?.runtime_hash ? `${localConfigSync.runtime_hash.slice(0, 12)}...` : "-"}
									mono
								/>
								<InfoPair
									label="Store Fingerprint"
									value={localConfigSync?.store_hash ? `${localConfigSync.store_hash.slice(0, 12)}...` : localConfigSync?.store_connected ? "-" : "N/A"}
									mono
								/>
							</CardContent>
						</Card>

						<Card className="shadow-none">
							<CardHeader className="pb-3">
								<CardTitle className="text-base">Vault Runtime Sync</CardTitle>
							</CardHeader>
							<CardContent className="space-y-3">
								{vaultLoading && !vaultStatus ? (
									<div className="space-y-2">
										<Skeleton className="h-4 w-28" />
										<Skeleton className="h-4 w-40" />
										<Skeleton className="h-4 w-32" />
									</div>
								) : vaultServiceDisabled ? (
									<div className="space-y-1">
										<p className="text-sm font-medium">Disabled</p>
										<p className="text-muted-foreground text-sm">Vault sync has not been enabled for this deployment yet.</p>
									</div>
								) : vaultStatus ? (
									<div className="space-y-3">
										<InfoPair label="Backend" value={vaultStatus.type || "-"} />
										<InfoPair label="Last Sync" value={formatTimestamp(vaultStatus.last_sync)} />
										<InfoPair label="Managed Secrets" value={vaultStatus.managed_secrets.toLocaleString()} />
										{vaultStatus.last_error && (
											<Alert variant="destructive" className="py-2">
												<AlertCircle />
												<AlertDescription>{vaultStatus.last_error}</AlertDescription>
											</Alert>
										)}
									</div>
								) : (
									<p className="text-muted-foreground text-sm">Vault status is unavailable.</p>
								)}
							</CardContent>
						</Card>
					</div>

					<Card className="shadow-none">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">Peer Status</CardTitle>
						</CardHeader>
						<CardContent>
							<Table containerClassName="max-h-[32rem]" data-testid="cluster-peers-table">
								<TableHeader>
									<TableRow>
										<TableHead>Address</TableHead>
										<TableHead>Status</TableHead>
										<TableHead>Config Sync</TableHead>
										<TableHead>Last Seen</TableHead>
										<TableHead>Successes</TableHead>
										<TableHead>Failures</TableHead>
										<TableHead>Last Error</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{clusterStatus.peers.length === 0 ? (
										<TableRow>
											<TableCell colSpan={7} className="h-24 text-center">
												<span className="text-muted-foreground text-sm">No peers are registered for this node yet.</span>
											</TableCell>
										</TableRow>
									) : (
										clusterStatus.peers.map((peer) => (
											<TableRow key={peer.address}>
												<TableCell>
													<div className="flex flex-col gap-0.5">
														<span className="font-mono text-xs">{peer.address}</span>
														<span className="text-muted-foreground text-xs">{peer.node_id || "node id unavailable"}</span>
													</div>
												</TableCell>
												<TableCell>
													<div className="flex flex-col gap-1">
														<div>
															<HealthBadge healthy={peer.healthy} />
														</div>
														<span className="text-muted-foreground text-xs">
															Remote cluster:{" "}
															{peer.reported_healthy === undefined ? "unknown" : peer.reported_healthy ? "healthy" : "degraded"}
														</span>
													</div>
												</TableCell>
												<TableCell>
													<div className="flex flex-col gap-0.5 text-sm">
														<span>
															{!peer.config_sync?.store_connected
																? "No store"
																: peer.config_sync.in_sync === false
																	? "Runtime drift"
																	: "In sync"}
														</span>
														<span className="text-muted-foreground text-xs">
															{peer.config_sync?.runtime_hash && localConfigSync?.runtime_hash
																? peer.config_sync.runtime_hash === localConfigSync.runtime_hash
																	? "Matches local runtime"
																	: "Differs from local runtime"
																: "Runtime match unavailable"}
														</span>
														<span className="text-muted-foreground text-xs">
															{peer.config_sync?.store_kind || "store type unavailable"}
															{peer.config_sync?.drift_domains?.length ? ` · ${peer.config_sync.drift_domains.join(", ")}` : ""}
														</span>
													</div>
												</TableCell>
												<TableCell>
													<div className="flex flex-col gap-0.5">
														<span>{formatTimestamp(peer.last_seen)}</span>
														<span className="text-muted-foreground text-xs">{formatRelativeTimestamp(peer.last_seen)}</span>
													</div>
												</TableCell>
												<TableCell>{peer.consecutive_successes.toLocaleString()}</TableCell>
												<TableCell>{peer.consecutive_failures.toLocaleString()}</TableCell>
												<TableCell className="max-w-[28rem] truncate text-sm">{peer.last_error || "-"}</TableCell>
											</TableRow>
										))
									)}
								</TableBody>
							</Table>
							{localConfigSync?.last_error && (
								<Alert variant="info" className="mt-4">
									<AlertCircle />
									<AlertTitle>Config sync status is partially degraded</AlertTitle>
									<AlertDescription>{localConfigSync.last_error}</AlertDescription>
								</Alert>
							)}
						</CardContent>
					</Card>
				</>
			)}
		</div>
	);
}

function InfoPair({ label, value, mono = false }: { label: string; value: ReactNode; mono?: boolean }) {
	return (
		<div className="space-y-1">
			<p className="text-muted-foreground text-xs">{label}</p>
			<div className={mono ? "font-mono text-sm" : "text-sm"}>{value}</div>
		</div>
	);
}
