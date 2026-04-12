"use client";

import FullPageLoader from "@/components/fullPageLoader";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ProviderLabels } from "@/lib/constants/logs";
import {
	getErrorMessage,
	useGetAdaptiveRoutingStatusQuery,
	useGetAlertsQuery,
	useGetProvidersQuery,
} from "@/lib/store";
import { formatPercentage, formatRelativeTimestamp, isServiceDisabledError } from "@/lib/utils/enterprise";
import { AlertCircle, CheckCircle2, RefreshCw, Wifi } from "lucide-react";
import { useMemo, useState } from "react";

function stateBadgeVariant(state: "healthy" | "degraded" | "failed" | "recovering"): "default" | "secondary" | "destructive" | "outline" {
	switch (state) {
		case "healthy":
			return "default";
		case "recovering":
			return "outline";
		case "degraded":
			return "secondary";
		default:
			return "destructive";
	}
}

export default function AdaptiveRoutingPage() {
	const [providerFilter, setProviderFilter] = useState("all");
	const [modelFilter, setModelFilter] = useState("all");

	const { data: providers = [] } = useGetProvidersQuery();
	const {
		data: routingStatus,
		error: routingError,
		isLoading: routingLoading,
		isFetching: routingFetching,
		refetch: refetchRouting,
	} = useGetAdaptiveRoutingStatusQuery(
		{
			provider: providerFilter === "all" ? undefined : providerFilter,
			model: modelFilter === "all" ? undefined : modelFilter,
		},
		{
			pollingInterval: 5000,
			skipPollingIfUnfocused: true,
		},
	);
	const {
		data: alertsResponse,
		isFetching: alertsFetching,
		refetch: refetchAlerts,
	} = useGetAlertsQuery(
		{ cluster: true },
		{ pollingInterval: 15000, skipPollingIfUnfocused: true },
	);

	const routingDisabled = isServiceDisabledError(routingError);

	const directionRows = useMemo(
		() =>
			(routingStatus?.directions ?? []).slice().sort((a, b) => b.weight - a.weight),
		[routingStatus?.directions],
	);
	const routeRows = useMemo(
		() =>
			(routingStatus?.routes ?? []).slice().sort((a, b) => b.weight - a.weight),
		[routingStatus?.routes],
	);

	// Live metrics computed from direction + route data
	const totalRequests = useMemo(() => {
		let total = 0;
		for (const d of directionRows) {
			total += d.samples;
		}
		return total;
	}, [directionRows]);

	const successRate = useMemo(() => {
		let totalSamples = 0;
		let totalSuccesses = 0;
		for (const d of directionRows) {
			totalSamples += d.samples;
			totalSuccesses += d.successes;
		}
		return totalSamples > 0 ? (totalSuccesses / totalSamples) * 100 : 100;
	}, [directionRows]);

	// Traffic distribution: aggregate by key (provider+model+key_id)
	const trafficDistribution = useMemo(() => {
		const totalSamples = routeRows.reduce((sum, r) => sum + r.samples, 0);
		return routeRows
			.filter((r) => r.samples > 0)
			.map((r) => ({
				key_id: r.key_id,
				provider: r.provider,
				model: r.model,
				samples: r.samples,
				share: totalSamples > 0 ? (r.samples / totalSamples) * 100 : 0,
			}))
			.sort((a, b) => b.share - a.share);
	}, [routeRows]);

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

	const modelOptions = useMemo(() => {
		const values = new Set<string>();
		for (const route of routingStatus?.routes ?? []) {
			if (route.model) {
				values.add(route.model);
			}
		}
		for (const direction of routingStatus?.directions ?? []) {
			if (direction.model) {
				values.add(direction.model);
			}
		}
		return [...values].sort((a, b) => a.localeCompare(b));
	}, [routingStatus?.directions, routingStatus?.routes]);

	const activeAlerts = alertsResponse?.alerts ?? [];

	if (routingLoading && !routingStatus) {
		return <FullPageLoader />;
	}

	const connectedTime = new Date().toLocaleTimeString();

	return (
		<div className="mx-auto flex w-full max-w-7xl flex-col gap-6">
			{/* Header */}
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h1 className="text-2xl font-semibold tracking-tight">Adaptive Load Balancing</h1>
					<p className="text-muted-foreground mt-1 text-sm">
						Cluster-aware live metrics for direction-level provider selection and route-level key balancing.
					</p>
				</div>
				<div className="flex items-center gap-3">
					<div className="flex items-center gap-2 text-sm">
						{routingDisabled ? (
							<span className="text-muted-foreground">Disconnected</span>
						) : (
							<>
								<Wifi className="h-4 w-4 text-green-500" />
								<span className="text-muted-foreground">Connected</span>
								<span className="text-muted-foreground">{connectedTime}</span>
							</>
						)}
					</div>
					<Button
						variant="outline"
						onClick={() => {
							void refetchRouting();
							void refetchAlerts();
						}}
						isLoading={routingFetching || alertsFetching}
						dataTestId="adaptive-routing-refresh"
					>
						{!(routingFetching || alertsFetching) && <RefreshCw className="h-4 w-4" />}
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
						请在 <strong>Provider Routing Rules</strong> 中创建 <strong>Adaptive Load Balancing</strong> 规则。这个页面现在只负责状态观测，不再直接承载策略配置。
					</AlertDescription>
				</Alert>
			)}

			{routingStatus && (
				<>
					{/* Live Metrics Summary */}
					<div className="grid gap-4 md:grid-cols-2">
						<Card className="shadow-none">
							<CardContent className="flex items-center justify-center px-6 py-8">
								<div className="text-center">
									<p className="font-mono text-4xl font-semibold">{totalRequests.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}</p>
									<p className="text-muted-foreground mt-1 text-sm">Total Requests</p>
								</div>
							</CardContent>
						</Card>
						<Card className="shadow-none">
							<CardContent className="flex items-center justify-center px-6 py-8">
								<div className="text-center">
									<p className="font-mono text-4xl font-semibold">{formatPercentage(successRate)}</p>
									<p className="text-muted-foreground mt-1 text-sm">Success Rate</p>
								</div>
							</CardContent>
						</Card>
					</div>

					{/* Active Alerts */}
					{activeAlerts.length > 0 && (
						<Card className="shadow-none border-orange-200 dark:border-orange-800">
							<CardHeader className="pb-3">
								<CardTitle className="text-base">Active Alerts</CardTitle>
							</CardHeader>
							<CardContent>
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
												<span>{alert.node_id || "local"}</span>
												<span>{alert.type}</span>
												<span>{formatRelativeTimestamp(alert.triggered_at)}</span>
											</div>
										</div>
									))}
								</div>
							</CardContent>
						</Card>
					)}

					{/* Filters */}
					<div className="flex flex-wrap gap-2">
						<Select value={providerFilter} onValueChange={setProviderFilter}>
							<SelectTrigger className="w-[160px]" data-testid="adaptive-routing-provider-filter">
								<SelectValue placeholder="All Providers" />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="all">All Providers</SelectItem>
								{providerOptions.map((provider) => (
									<SelectItem key={provider} value={provider}>
										{ProviderLabels[provider as keyof typeof ProviderLabels] || provider}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
						<Select value={modelFilter} onValueChange={setModelFilter}>
							<SelectTrigger className="w-[180px]" data-testid="adaptive-routing-model-filter">
								<SelectValue placeholder="All Models" />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="all">All Models</SelectItem>
								{modelOptions.map((model) => (
									<SelectItem key={model} value={model}>
										{model}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>

					{/* Total Traffic Distribution */}
					<Card className="shadow-none">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">Total Traffic Distribution in the last 10s</CardTitle>
						</CardHeader>
						<CardContent>
							<Table data-testid="traffic-distribution-table">
								<TableHeader>
									<TableRow>
										<TableHead>Key</TableHead>
										<TableHead>Provider</TableHead>
										<TableHead>Model</TableHead>
										<TableHead className="w-[40%]">Total Traffic</TableHead>
										<TableHead className="text-right">Share</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{trafficDistribution.length === 0 ? (
										<TableRow>
											<TableCell colSpan={5} className="h-24 text-center">
												<span className="text-muted-foreground text-sm">No traffic data collected yet.</span>
											</TableCell>
										</TableRow>
									) : (
										trafficDistribution.map((row) => (
											<TableRow key={`${row.provider}/${row.model}/${row.key_id}`}>
												<TableCell className="font-mono text-xs">{row.key_id}</TableCell>
												<TableCell>{ProviderLabels[row.provider as keyof typeof ProviderLabels] || row.provider}</TableCell>
												<TableCell className="font-mono text-xs">{row.model}</TableCell>
												<TableCell>
													<div className="flex items-center gap-2">
														<div className="h-2.5 flex-1 rounded-full bg-muted">
															<div
																className="h-2.5 rounded-full bg-primary"
																style={{ width: `${Math.min(row.share, 100)}%` }}
															/>
														</div>
													</div>
												</TableCell>
												<TableCell className="text-right font-mono text-sm">{formatPercentage(row.share)}</TableCell>
											</TableRow>
										))
									)}
								</TableBody>
							</Table>
						</CardContent>
					</Card>

					{/* Direction Weights & Performance */}
					<Card className="shadow-none">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">Direction Weights &amp; Performance</CardTitle>
						</CardHeader>
						<CardContent>
							<Table containerClassName="max-h-[32rem]" data-testid="adaptive-directions-table">
								<TableHeader>
									<TableRow>
										<TableHead>Provider</TableHead>
										<TableHead>Model</TableHead>
										<TableHead>Weight &darr;</TableHead>
										<TableHead>Success Rate</TableHead>
										<TableHead>Errors &uarr;</TableHead>
										<TableHead>U (Utilization Penalty)</TableHead>
										<TableHead>E (Error Penalty)</TableHead>
										<TableHead>L (Latency Penalty) &uarr;</TableHead>
										<TableHead>Health Status</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{directionRows.length === 0 ? (
										<TableRow>
											<TableCell colSpan={9} className="h-24 text-center">
												<span className="text-muted-foreground text-sm">No direction metrics have been collected yet.</span>
											</TableCell>
										</TableRow>
									) : (
										directionRows.map((direction) => {
											const dirSuccessRate = direction.samples > 0
												? (direction.successes / direction.samples) * 100
												: 100;
											const utilPenalty = direction.actual_traffic_share > 0
												? Math.max(0, direction.actual_traffic_share - direction.expected_traffic_share).toFixed(2)
												: "0.00";
											const errPenalty = direction.error_ewma.toFixed(2);
											const latPenalty = direction.latency_ewma > 0 ? (direction.latency_ewma / 1000).toFixed(2) : "0.00";
											return (
												<TableRow key={`${direction.node_id || "local"}/${direction.provider}/${direction.model}`}>
													<TableCell>{ProviderLabels[direction.provider as keyof typeof ProviderLabels] || direction.provider}</TableCell>
													<TableCell className="font-mono text-xs">{direction.model || "*"}</TableCell>
													<TableCell>
														<div className="text-sm">
															{direction.weight.toLocaleString()}
															<span className="text-muted-foreground ml-1 text-xs">
																{formatPercentage(direction.expected_traffic_share * 100)}
															</span>
														</div>
													</TableCell>
													<TableCell>
														<span className={dirSuccessRate >= 99 ? "text-green-600 font-medium" : dirSuccessRate >= 90 ? "text-yellow-600" : "text-red-600 font-medium"}>
															{formatPercentage(dirSuccessRate)}
														</span>
														<span className="text-muted-foreground ml-1 text-xs">{direction.samples} reqs</span>
													</TableCell>
													<TableCell>{direction.failures}</TableCell>
													<TableCell>
														<span className={Number(utilPenalty) > 0 ? "text-yellow-600" : "text-green-600"}>{utilPenalty}</span>
													</TableCell>
													<TableCell>
														<span className={Number(errPenalty) > 0.01 ? "text-red-600" : "text-green-600"}>{errPenalty}</span>
													</TableCell>
													<TableCell>
														<span className={Number(latPenalty) > 1 ? "text-yellow-600" : ""}>{latPenalty}</span>
													</TableCell>
													<TableCell>
														<Badge variant={stateBadgeVariant(direction.state)}>{direction.state}</Badge>
													</TableCell>
												</TableRow>
											);
										})
									)}
								</TableBody>
							</Table>
						</CardContent>
					</Card>

					{/* Route Weights & Performance */}
					<Card className="shadow-none">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">Route Weights &amp; Performance</CardTitle>
						</CardHeader>
						<CardContent>
							<Table containerClassName="max-h-[32rem]" data-testid="adaptive-routes-table">
								<TableHeader>
									<TableRow>
										<TableHead>Key</TableHead>
										<TableHead>Provider</TableHead>
										<TableHead>Model</TableHead>
										<TableHead>Weight &darr;</TableHead>
										<TableHead>Success Rate</TableHead>
										<TableHead>Errors &uarr;</TableHead>
										<TableHead>U (Utilization Penalty)</TableHead>
										<TableHead>E (Error Penalty)</TableHead>
										<TableHead>L (Latency Penalty)</TableHead>
										<TableHead>M (Momentum)</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{routeRows.length === 0 ? (
										<TableRow>
											<TableCell colSpan={10} className="h-24 text-center">
												<span className="text-muted-foreground text-sm">No route metrics have been collected yet.</span>
											</TableCell>
										</TableRow>
									) : (
										routeRows.map((route) => {
											const routeSuccessRate = route.samples > 0 ? (route.successes / route.samples) * 100 : 0;
											const utilPenalty = route.actual_traffic_share > 0
												? Math.max(0, route.actual_traffic_share - route.expected_traffic_share).toFixed(2)
												: "0.00";
											const errPenalty = route.error_ewma.toFixed(2);
											const latPenalty = route.latency_ewma > 0 ? (route.latency_ewma / 1000).toFixed(2) : "0.00";
											const momentum = route.score.toFixed(2);
											return (
												<TableRow key={`${route.node_id || "local"}/${route.provider}/${route.model}/${route.key_id}`}>
													<TableCell className="font-mono text-xs">{route.key_id}</TableCell>
													<TableCell>{ProviderLabels[route.provider as keyof typeof ProviderLabels] || route.provider}</TableCell>
													<TableCell className="font-mono text-xs">{route.model || "*"}</TableCell>
													<TableCell>
														<div className="text-sm">
															{route.weight.toLocaleString()}
															<span className="text-muted-foreground ml-1 text-xs">
																{formatPercentage(route.expected_traffic_share * 100)}
															</span>
														</div>
													</TableCell>
													<TableCell>
														<span className={routeSuccessRate >= 99 ? "text-green-600 font-medium" : routeSuccessRate >= 90 ? "text-yellow-600" : "text-red-600 font-medium"}>
															{formatPercentage(routeSuccessRate)}
														</span>
														<span className="text-muted-foreground ml-1 text-xs">{route.samples} reqs</span>
													</TableCell>
													<TableCell>{route.failures}</TableCell>
													<TableCell>
														<span className={Number(utilPenalty) > 0 ? "text-yellow-600" : "text-green-600"}>{utilPenalty}</span>
													</TableCell>
													<TableCell>
														<span className={Number(errPenalty) > 0.01 ? "text-red-600" : "text-green-600"}>{errPenalty}</span>
													</TableCell>
													<TableCell>{latPenalty}</TableCell>
													<TableCell>{momentum}</TableCell>
												</TableRow>
											);
										})
									)}
								</TableBody>
							</Table>
						</CardContent>
					</Card>
				</>
			)}
		</div>
	);
}
