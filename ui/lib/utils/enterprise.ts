import { getErrorMessage } from "@/lib/store";
import { formatDistanceToNow } from "date-fns";

export function isServiceDisabledError(error: unknown) {
	return getErrorMessage(error).toLowerCase().includes("not enabled");
}

export function formatTimestamp(value?: string) {
	if (!value) {
		return "-";
	}
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return "-";
	}
	return date.toLocaleString();
}

export function formatRelativeTimestamp(value?: string) {
	if (!value) {
		return "Never";
	}
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return "-";
	}
	return formatDistanceToNow(date, { addSuffix: true });
}

export function formatPercentage(value: number, fractionDigits = 1) {
	return `${value.toFixed(fractionDigits)}%`;
}

export function formatLatencyMs(value?: number, fractionDigits = 1) {
	if (value === undefined || value === null || Number.isNaN(value)) {
		return "-";
	}
	return `${value.toFixed(fractionDigits)} ms`;
}

export function toDateTimeLocalValue(value?: string) {
	if (!value) {
		return "";
	}
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return "";
	}
	const localDate = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
	return localDate.toISOString().slice(0, 16);
}

export function fromDateTimeLocalValue(value: string) {
	if (!value) {
		return undefined;
	}
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return undefined;
	}
	return date.toISOString();
}
