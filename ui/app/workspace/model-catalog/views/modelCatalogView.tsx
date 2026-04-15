"use client";

import { DateTimePickerWithRange } from "@/components/ui/datePickerWithRange";
import { useGetModelsQuery, useGetProvidersQuery, useLazyGetLogsStatsQuery, useLazyGetLogsModelHistogramQuery } from "@/lib/store";
import { ProviderNames } from "@/lib/constants/logs";
import { KnownProvider } from "@/lib/types/config";
import { LogStats } from "@/lib/types/logs";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { useEffect, useMemo, useState } from "react";
import { DateRange } from "react-day-picker";
import ModelCatalogTable, { ModelCatalogRow } from "./modelCatalogTable";
import { ModelCatalogEmptyState } from "./modelCatalogEmptyState";
import FullPageLoader from "@/components/fullPageLoader";
import { NoPermissionView } from "@/components/noPermissionView";

const MODEL_CATALOG_TIME_PERIODS = [
	{ label: "Last 24 hours", value: "24h" },
	{ label: "Last 7 days", value: "7d" },
	{ label: "Last 30 days", value: "30d" },
];

function getRangeForPeriod(period: string): { from: Date; to: Date } {
	const to = new Date();
	const from = new Date(to.getTime());
	switch (period) {
		case "7d":
			from.setDate(from.getDate() - 7);
			break;
		case "30d":
			from.setDate(from.getDate() - 30);
			break;
		case "24h":
		default:
			from.setHours(from.getHours() - 24);
			break;
	}
	return { from, to };
}

export default function ModelCatalogView() {
	const hasAccess = useRbac(RbacResource.ModelProvider, RbacOperation.View);

	const [providerFilter, setProviderFilter] = useState("");
	const [statsMap, setStatsMap] = useState<Map<string, LogStats>>(new Map());
	const [modelsUsedMap, setModelsUsedMap] = useState<Map<string, string[]>>(new Map());
	const [isLoadingModels, setIsLoadingModels] = useState(true);
	const [selectedPeriod, setSelectedPeriod] = useState<string | undefined>("24h");
	const [dateRange, setDateRange] = useState<DateRange>(() => getRangeForPeriod("24h"));

	const {
		data: providers,
		isLoading: isLoadingProviders,
		error: providersError,
		refetch: refetchProviders,
	} = useGetProvidersQuery(undefined, { skip: !hasAccess });
	const { data: modelsData } = useGetModelsQuery({ unfiltered: true }, { skip: !hasAccess });

	// Global 24h stats for summary cards (lazy so we get fresh timestamps)
	const [triggerGlobalStats, { data: globalStats }] = useLazyGetLogsStatsQuery();

	// Per-provider traffic stats (lazy, fired when providers load)
	const [triggerStats] = useLazyGetLogsStatsQuery();
	const [triggerModelHistogram] = useLazyGetLogsModelHistogramQuery();

	const startTime = dateRange.from?.toISOString();
	const endTime = dateRange.to?.toISOString();
	const timeRangeLabel = useMemo(
		() => MODEL_CATALOG_TIME_PERIODS.find((period) => period.value === selectedPeriod)?.label ?? "Selected Range",
		[selectedPeriod],
	);

	useEffect(() => {
		if (!hasAccess || !startTime || !endTime) return;
		triggerGlobalStats({ filters: { start_time: startTime, end_time: endTime } });
	}, [endTime, hasAccess, startTime, triggerGlobalStats]);

	useEffect(() => {
		if (!providers || providers.length === 0 || !startTime || !endTime) return;
		let cancelled = false;

		Promise.all(
			providers.map((p) =>
				triggerStats({ filters: { providers: [p.name], start_time: startTime, end_time: endTime } })
					.unwrap()
					.then((stats) => [p.name, stats] as const)
					.catch(() => [p.name, { total_requests: 0, success_rate: 0, average_latency: 0, total_tokens: 0, total_cost: 0 }] as const),
			),
		).then((results) => {
			if (!cancelled) setStatsMap(new Map(results));
		});
		return () => {
			cancelled = true;
		};
	}, [endTime, providers, startTime, triggerStats]);

	// Per-provider models used in the selected range
	useEffect(() => {
		if (!providers || providers.length === 0 || !startTime || !endTime) return;
		let cancelled = false;
		setIsLoadingModels(true);

		Promise.all(
			providers.map((p) =>
				triggerModelHistogram({ filters: { providers: [p.name], start_time: startTime, end_time: endTime } })
					.unwrap()
					.then((data): [string, string[]] => [p.name, data.models ?? []])
					.catch((): [string, string[]] => [p.name, []]),
			),
		).then((results) => {
			if (!cancelled) {
				setModelsUsedMap(new Map(results));
				setIsLoadingModels(false);
			}
		});
		return () => {
			cancelled = true;
		};
	}, [endTime, providers, startTime, triggerModelHistogram]);

	// Build table rows
	const rows: ModelCatalogRow[] = useMemo(() => {
		if (!providers) return [];

		return providers.map((p) => {
			const isCustom = !ProviderNames.includes(p.name as KnownProvider);
			const modelsUsed = modelsUsedMap.get(p.name) ?? [];

			const providerStats = statsMap.get(p.name);
			const totalTraffic24h = providerStats?.total_requests ?? 0;
			const totalCost24h = providerStats?.total_cost ?? 0;

			return {
				providerName: p.name,
				isCustom,
				baseProviderType: p.custom_provider_config?.base_provider_type,
				modelsUsed,
				totalTraffic24h,
				totalCost24h,
			};
		});
	}, [providers, statsMap, modelsUsedMap]);

	// Filter rows by provider
	const filteredRows = useMemo(() => {
		if (!providerFilter) return rows;
		return rows.filter((r) => r.providerName === providerFilter);
	}, [rows, providerFilter]);

	if (isLoadingProviders) {
		return <FullPageLoader />;
	}

	if (!hasAccess) {
		return <NoPermissionView entity="model catalog" />;
	}

	if (providersError) {
		return (
			<div className="flex h-full flex-col items-center justify-center gap-4 text-center">
				<p className="text-muted-foreground text-sm">Failed to load providers</p>
				<button type="button" data-testid="model-catalog-retry-btn" onClick={refetchProviders} className="text-sm underline">
					Retry
				</button>
			</div>
		);
	}

	if (!providers || providers.length === 0) {
		return <ModelCatalogEmptyState />;
	}

	return (
		<div className="mx-auto w-full max-w-7xl">
			<ModelCatalogTable
				rows={filteredRows}
				providers={(providers ?? []).map((p) => p.name)}
				providerFilter={providerFilter}
				onProviderFilterChange={setProviderFilter}
				totalProviders={(providers ?? []).length}
				totalModels={modelsData?.total ?? 0}
				totalRequestsInRange={globalStats?.total_requests ?? 0}
				totalCostInRange={globalStats?.total_cost ?? 0}
				isLoadingModels={isLoadingModels}
				timeRangeLabel={timeRangeLabel}
				timeRangePicker={
					<DateTimePickerWithRange
						triggerTestId="model-catalog-date-range"
						dateTime={dateRange}
						preDefinedPeriods={MODEL_CATALOG_TIME_PERIODS}
						predefinedPeriod={selectedPeriod}
						onDateTimeUpdate={(range) => {
							setDateRange({ from: range.from, to: range.to });
							setSelectedPeriod(undefined);
						}}
						onPredefinedPeriodChange={(periodValue) => {
							if (!periodValue) return;
							const range = getRangeForPeriod(periodValue);
							setDateRange(range);
							setSelectedPeriod(periodValue);
						}}
					/>
				}
			/>
		</div>
	);
}
