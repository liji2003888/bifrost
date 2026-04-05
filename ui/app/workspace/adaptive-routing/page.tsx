"use client";

import FullPageLoader from "@/components/fullPageLoader";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useDebouncedValue } from "@/hooks/useDebounce";
import { ProviderLabels } from "@/lib/constants/logs";
import { getErrorMessage, useGetAdaptiveRoutingStatusQuery, useGetAlertsQuery, useGetProvidersQuery } from "@/lib/store";
import { formatLatencyMs, formatPercentage, formatRelativeTimestamp, isServiceDisabledError } from "@/lib/utils/enterprise";
import { AlertCircle, ArrowUpDown, Gauge, RefreshCw, Route, ShieldAlert } from "lucide-react";
import { useMemo, useState } from "react";

function scoreBadgeVariant(score: number): "default" | "secondary" | "destructive" {
	if (score >= 0.85) {
		return "default";
	}
	if (score >= 0.6) {
		return "secondary";
	}
	return "destructive";
}

export default function AdaptiveRoutingPage() {
	const [providerFilter, setProviderFilter] = useState("all");
	const [modelFilter, setModelFilter] = useState("");
	const debouncedModelFilter = useDebouncedValue(modelFilter, 250);

	const { data: providers = [] } = useGetProvidersQuery();
	const {
		data: routingStatus,
		error: routingError,
		isLoading: routingLoading,
		isFetching: routingFetching,
		refetch: refetchRouting,
	} = useGetAdaptiveRoutingStatusQuery(
		{
			cluster: true,
			provider: providerFilter === "all" ? undefined : providerFilter,
			model: debouncedModelFilter.trim() || undefined,
		},
		{
			pollingInterval: 5000,
			skipPollingIfUnfocused: true,
		},
	);
	const {
		data: alertsResponse,
		error: alertsError,
		isFetching: alertsFetching,
		refetch: refetchAlerts,
	} = useGetAlertsQuery(
		{
			cluster: true,
		},
		{
			pollingInterval: 15000,
			skipPollingIfUnfocused: true,
		},
	);

	const routingDisabled = isServiceDisabledError(routingError);
	const alertsDisabled = isServiceDisabledError(alertsError);

	const directionRows = useMemo(
		() =>
			(routingStatus?.directions ?? []).slice().sort((a, b) => {
				if (a.score === b.score) {
					return b.samples - a.samples;
				}
				return b.score - a.score;
			}),
		[routingStatus?.directions],
	);
	const routeRows = useMemo(
		() =>
			(routingStatus?.routes ?? []).slice().sort((a, b) => {
				if (a.consecutive_failures === b.consecutive_failures) {
					return b.error_ewma - a.error_ewma;
				}
				return b.consecutive_failures - a.consecutive_failures;
			}),
		[routingStatus?.routes],
	);
	const activeAlerts = alertsResponse?.alerts ?? [];
	const routingWarnings = routingStatus?.warnings ?? [];
	const alertWarnings = alertsResponse?.warnings ?? [];
	const degradedRoutes = routeRows.filter((route) => route.consecutive_failures > 0 || route.error_ewma >= 0.4).length;
	const lowScoreDirections = directionRows.filter((direction) => direction.score < 0.6).length;
	const reportingNodes = useMemo(() => {
		const values = new Set<string>();
		for (const route of routeRows) {
			if (route.node_id) {
				values.add(route.node_id);
			}
		}
		for (const direction of directionRows) {
			if (direction.node_id) {
				values.add(direction.node_id);
			}
		}
		return values.size;
	}, [directionRows, routeRows]);

	const providerOptions = useMemo(() => {
		const values = new Set<string>();
		for (const provider of providers) {
			values.add(provider.name);
		}
		for (const route of routingStatus?.routes ?? []) {
			values.add(route.provider);
		}
		return [...values].sort((a, b) => a.localeCompare(b));
	}, [providers, routingStatus?.routes]);

	if (routingLoading && !routingStatus) {
		return <FullPageLoader />;
	}

	return (
		<div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h1 className="text-2xl font-semibold tracking-tight">Adaptive Routing</h1>
					<p className="text-muted-foreground mt-1 text-sm">
						Inspect key-level route scoring, provider direction health, and the active enterprise alert feed.
					</p>
				</div>
				<div className="flex flex-wrap gap-2">
					<Select value={providerFilter} onValueChange={setProviderFilter}>
						<SelectTrigger className="w-[180px]" data-testid="adaptive-routing-provider-filter">
							<SelectValue placeholder="All providers" />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">All providers</SelectItem>
							{providerOptions.map((provider) => (
								<SelectItem key={provider} value={provider}>
									{ProviderLabels[provider as keyof typeof ProviderLabels] || provider}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					<Input
						value={modelFilter}
						onChange={(event) => setModelFilter(event.target.value)}
						placeholder="Filter model"
						className="w-[220px]"
						data-testid="adaptive-routing-model-filter"
					/>
					<Button
						variant="outline"
						onClick={() => {
							void refetchRouting();
							void refetchAlerts();
						}}
						isLoading={routingFetching || alertsFetching}
						dataTestId="adaptive-routing-refresh"
					>
						{!(routingFetching || alertsFetching) && <RefreshCw />}
						Refresh
					</Button>
				</div>
			</div>

			{Boolean(routingError) && !routingDisabled && (
				<Alert variant="destructive">
					<AlertCircle />
					<AlertTitle>Unable to load adaptive routing status</AlertTitle>
					<AlertDescription>{getErrorMessage(routingError)}</AlertDescription>
				</Alert>
			)}

			{routingDisabled && (
				<Alert variant="info">
					<AlertCircle />
					<AlertTitle>Adaptive routing is not enabled</AlertTitle>
					<AlertDescription>
						Enable `load_balancer_config.enabled` to expose real-time route scores, fallback ordering, and traffic health signals.
					</AlertDescription>
				</Alert>
			)}

			{routingStatus && (
				<>
					<div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
						<SummaryCard label="Tracked Routes" value={routeRows.length.toLocaleString()} icon={Route} />
						<SummaryCard label="Direction Scores" value={directionRows.length.toLocaleString()} icon={ArrowUpDown} />
						<SummaryCard label="Degraded Routes" value={degradedRoutes.toLocaleString()} icon={ShieldAlert} />
						<SummaryCard
							label={routingStatus?.cluster ? "Nodes Reporting" : "Low-Score Directions"}
							value={routingStatus?.cluster ? reportingNodes.toLocaleString() : lowScoreDirections.toLocaleString()}
							icon={routingStatus?.cluster ? Gauge : Gauge}
						/>
					</div>

					{routingWarnings.length > 0 && (
						<Alert variant="info">
							<AlertCircle />
							<AlertTitle>Partial cluster aggregation</AlertTitle>
							<AlertDescription>
								{routingWarnings.length} peer{routingWarnings.length === 1 ? "" : "s"} could not be queried for adaptive routing status.
							</AlertDescription>
						</Alert>
					)}

					<Card className="shadow-none">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">Active Alerts</CardTitle>
						</CardHeader>
						<CardContent>
							{Boolean(alertsError) && !alertsDisabled && (
								<Alert variant="destructive" className="mb-4">
									<AlertCircle />
									<AlertDescription>{getErrorMessage(alertsError)}</AlertDescription>
								</Alert>
							)}
							{alertsDisabled ? (
								<p className="text-muted-foreground text-sm">Alerts are not enabled for this deployment.</p>
							) : activeAlerts.length === 0 ? (
								<p className="text-muted-foreground text-sm">No active alerts right now.</p>
							) : (
								<div className="grid gap-3 lg:grid-cols-2">
									{activeAlerts.map((alert) => (
										<div key={`${alert.node_id || alert.address || "local"}:${alert.id}`} className="rounded-sm border p-4">
											<div className="flex items-start justify-between gap-3">
												<div>
													<p className="font-medium">{alert.title}</p>
													<p className="text-muted-foreground mt-1 text-sm">{alert.message}</p>
												</div>
												<Badge
													variant={alert.severity === "critical" ? "destructive" : alert.severity === "warning" ? "secondary" : "outline"}
												>
													{alert.severity}
												</Badge>
											</div>
											<div className="text-muted-foreground mt-3 flex flex-wrap gap-4 text-xs">
												<span>{alert.node_id || alertsResponse?.node_id || "local"}</span>
												<span>{alert.type}</span>
												<span>{formatRelativeTimestamp(alert.triggered_at)}</span>
											</div>
										</div>
									))}
								</div>
							)}
							{alertWarnings.length > 0 && (
								<p className="text-muted-foreground mt-4 text-xs">
									{alertWarnings.length} peer{alertWarnings.length === 1 ? "" : "s"} did not return alert data during aggregation.
								</p>
							)}
						</CardContent>
					</Card>

					<div className="grid gap-4 xl:grid-cols-2">
						<Card className="shadow-none">
							<CardHeader className="pb-3">
								<CardTitle className="text-base">Direction Health</CardTitle>
							</CardHeader>
							<CardContent>
								<Table containerClassName="max-h-[32rem]" data-testid="adaptive-directions-table">
									<TableHeader>
										<TableRow>
											<TableHead>Node</TableHead>
											<TableHead>Provider</TableHead>
											<TableHead>Model</TableHead>
											<TableHead>Score</TableHead>
											<TableHead>Samples</TableHead>
											<TableHead>Latency</TableHead>
											<TableHead>Error EWMA</TableHead>
											<TableHead>Updated</TableHead>
										</TableRow>
									</TableHeader>
									<TableBody>
										{directionRows.length === 0 ? (
											<TableRow>
												<TableCell colSpan={8} className="h-24 text-center">
													<span className="text-muted-foreground text-sm">No direction metrics have been collected yet.</span>
												</TableCell>
											</TableRow>
										) : (
											directionRows.map((direction) => (
												<TableRow key={`${direction.node_id || direction.address || "local"}/${direction.provider}/${direction.model}`}>
													<TableCell className="font-mono text-xs">{direction.node_id || direction.address || "local"}</TableCell>
													<TableCell>{ProviderLabels[direction.provider as keyof typeof ProviderLabels] || direction.provider}</TableCell>
													<TableCell className="font-mono text-xs">{direction.model || "*"}</TableCell>
													<TableCell>
														<Badge variant={scoreBadgeVariant(direction.score)}>{direction.score.toFixed(2)}</Badge>
													</TableCell>
													<TableCell>{direction.samples.toLocaleString()}</TableCell>
													<TableCell>{formatLatencyMs(direction.latency_ewma)}</TableCell>
													<TableCell>{formatPercentage(direction.error_ewma * 100)}</TableCell>
													<TableCell className="text-xs">{formatRelativeTimestamp(direction.last_updated)}</TableCell>
												</TableRow>
											))
										)}
									</TableBody>
								</Table>
							</CardContent>
						</Card>

						<Card className="shadow-none">
							<CardHeader className="pb-3">
								<CardTitle className="text-base">Key Route Health</CardTitle>
							</CardHeader>
							<CardContent>
								<Table containerClassName="max-h-[32rem]" data-testid="adaptive-routes-table">
									<TableHeader>
										<TableRow>
											<TableHead>Node</TableHead>
											<TableHead>Provider</TableHead>
											<TableHead>Model</TableHead>
											<TableHead>Key</TableHead>
											<TableHead>Samples</TableHead>
											<TableHead>Success Rate</TableHead>
											<TableHead>Latency</TableHead>
											<TableHead>Failures</TableHead>
											<TableHead>Updated</TableHead>
										</TableRow>
									</TableHeader>
									<TableBody>
										{routeRows.length === 0 ? (
											<TableRow>
												<TableCell colSpan={9} className="h-24 text-center">
													<span className="text-muted-foreground text-sm">No route metrics have been collected yet.</span>
												</TableCell>
											</TableRow>
										) : (
											routeRows.map((route) => {
												const successRate = route.samples > 0 ? (route.successes / route.samples) * 100 : 0;
												return (
													<TableRow key={`${route.node_id || route.address || "local"}/${route.provider}/${route.model}/${route.key_id}`}>
														<TableCell className="font-mono text-xs">{route.node_id || route.address || "local"}</TableCell>
														<TableCell>{ProviderLabels[route.provider as keyof typeof ProviderLabels] || route.provider}</TableCell>
														<TableCell className="font-mono text-xs">{route.model || "*"}</TableCell>
														<TableCell className="font-mono text-xs">{route.key_id}</TableCell>
														<TableCell>{route.samples.toLocaleString()}</TableCell>
														<TableCell>{formatPercentage(successRate)}</TableCell>
														<TableCell>{formatLatencyMs(route.latency_ewma)}</TableCell>
														<TableCell>
															<div className="flex items-center gap-2">
																<Badge variant={route.consecutive_failures > 0 ? "destructive" : "outline"}>
																	{route.consecutive_failures}
																</Badge>
																<span className="text-muted-foreground text-xs">{route.failures} total</span>
															</div>
														</TableCell>
														<TableCell className="text-xs">{formatRelativeTimestamp(route.last_updated)}</TableCell>
													</TableRow>
												);
											})
										)}
									</TableBody>
								</Table>
							</CardContent>
						</Card>
					</div>
				</>
			)}
		</div>
	);
}

function SummaryCard({ label, value, icon: Icon }: { label: string; value: string; icon: typeof Route }) {
	return (
		<Card className="shadow-none">
			<CardContent className="flex items-start justify-between px-4 py-4">
				<div>
					<p className="text-muted-foreground text-xs">{label}</p>
					<p className="mt-1 text-xl font-semibold">{value}</p>
				</div>
				<Icon className="text-muted-foreground mt-0.5 h-4 w-4" />
			</CardContent>
		</Card>
	);
}
