"use client";

import { useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { useToast } from "@/hooks/use-toast";
import { usePreviewMCPHostedToolMutation } from "@/lib/store/apis/mcpApi";
import type { MCPHostedTool, PreviewMCPHostedToolResult } from "@/lib/types/mcp";

interface MCPHostedToolPreviewDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	tool: MCPHostedTool | null;
}

function defaultPreviewValue(schema: any): any {
	switch (schema?.type) {
		case "number":
		case "integer":
			return 0;
		case "boolean":
			return false;
		case "array":
			return [];
		case "object": {
			const result: Record<string, any> = {};
			for (const key of schema?.required || []) {
				result[key] = defaultPreviewValue(schema?.properties?.[key]);
			}
			return result;
		}
		default:
			return "";
	}
}

function buildPreviewArgs(tool: MCPHostedTool | null): Record<string, any> {
	const parameters = tool?.tool_schema?.function?.parameters;
	const properties = parameters?.properties || {};
	const required = parameters?.required || [];
	if (required.length === 0) {
		return {};
	}
	return Object.fromEntries(required.map((key) => [key, defaultPreviewValue(properties[key])]));
}

function previewStatusBadge(result: PreviewMCPHostedToolResult | null) {
	if (!result) {
		return null;
	}
	if (result.status_code >= 200 && result.status_code < 300) {
		return <Badge variant="default">Success</Badge>;
	}
	return <Badge variant="destructive">HTTP {result.status_code}</Badge>;
}

export function MCPHostedToolPreviewDialog({ open, onOpenChange, tool }: MCPHostedToolPreviewDialogProps) {
	const { toast } = useToast();
	const [argsText, setArgsText] = useState("{}");
	const [result, setResult] = useState<PreviewMCPHostedToolResult | null>(null);
	const [previewHostedTool, { isLoading }] = usePreviewMCPHostedToolMutation();

	const defaultArgs = useMemo(() => JSON.stringify(buildPreviewArgs(tool), null, 2), [tool]);

	useEffect(() => {
		if (!open) {
			return;
		}
		setArgsText(defaultArgs);
		setResult(null);
	}, [open, defaultArgs]);

	const runPreview = async () => {
		if (!tool) {
			return;
		}
		let args: Record<string, any> = {};
		if (argsText.trim()) {
			try {
				args = JSON.parse(argsText);
			} catch {
				toast({ title: "Error", description: "Preview args must be valid JSON.", variant: "destructive" });
				return;
			}
		}
		try {
			const response = await previewHostedTool({ id: tool.tool_id, data: { args } }).unwrap();
			setResult(response.preview);
		} catch (error: any) {
			toast({
				title: "Preview failed",
				description: error?.data?.error?.message || "Failed to preview hosted tool.",
				variant: "destructive",
			});
		}
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-h-[90vh] max-w-5xl overflow-y-auto">
				<DialogHeader>
					<DialogTitle>Preview Hosted Tool</DialogTitle>
					<DialogDescription>
						Run a safe, on-demand preview against the currently selected node. This uses the hosted tool&apos;s timeout and body-size limits and does not affect normal LLM inference traffic.
					</DialogDescription>
				</DialogHeader>

				<div className="grid gap-4 lg:grid-cols-2">
					<div className="space-y-4">
						<div className="space-y-2">
							<Label>Tool</Label>
							<div className="rounded-md border p-3 text-sm">
								<div className="font-medium">{tool?.name || "Unknown tool"}</div>
								<div className="text-muted-foreground mt-1 font-mono text-xs">{tool?.method} {tool?.url}</div>
							</div>
						</div>

						<div className="space-y-2">
							<Label>Preview Args (JSON)</Label>
							<Textarea
								value={argsText}
								onChange={(e) => setArgsText(e.target.value)}
								rows={12}
								className="font-mono text-xs"
								placeholder='{"user_id":"42"}'
							/>
						</div>

						<div className="flex justify-end">
							<Button onClick={() => void runPreview()} disabled={isLoading || !tool}>
								{isLoading ? "Running Preview..." : "Run Preview"}
							</Button>
						</div>
					</div>

					<div className="space-y-4">
						<div className="grid grid-cols-2 gap-3">
							<div className="rounded-md border p-3">
								<div className="text-muted-foreground text-xs">Status</div>
								<div className="mt-2">{previewStatusBadge(result) || <Badge variant="outline">Not run</Badge>}</div>
							</div>
							<div className="rounded-md border p-3">
								<div className="text-muted-foreground text-xs">Latency</div>
								<div className="mt-2 text-sm font-medium">{result ? `${result.latency_ms} ms` : "-"}</div>
							</div>
							<div className="rounded-md border p-3">
								<div className="text-muted-foreground text-xs">Response Size</div>
								<div className="mt-2 text-sm font-medium">{result ? `${result.response_bytes} bytes` : "-"}</div>
							</div>
							<div className="rounded-md border p-3">
								<div className="text-muted-foreground text-xs">Content Type</div>
								<div className="mt-2 truncate text-sm font-medium">{result?.content_type || "-"}</div>
							</div>
						</div>

						<div className="space-y-2">
							<Label>Resolved URL</Label>
							<div className="rounded-md border p-3 font-mono text-xs break-all">
								{result?.resolved_url || tool?.url || "-"}
							</div>
						</div>

						<div className="space-y-2">
							<Label>Response Output</Label>
							<div className="rounded-md border p-3">
								<pre className="max-h-[260px] overflow-auto whitespace-pre-wrap break-words font-mono text-xs">
									{result?.output || "Run preview to inspect the structured output."}
								</pre>
								{result?.truncated ? (
									<p className="text-muted-foreground mt-2 text-xs">
										Preview output was truncated to keep the UI and gateway memory profile stable.
									</p>
								) : null}
							</div>
						</div>

						<div className="space-y-2">
							<Label>Response Schema</Label>
							<div className="rounded-md border p-3">
								<pre className="max-h-[220px] overflow-auto whitespace-pre-wrap break-words font-mono text-xs">
									{JSON.stringify(result?.response_schema || tool?.response_schema || {}, null, 2) || "{}"}
								</pre>
							</div>
						</div>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}
