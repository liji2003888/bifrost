"use client";

import { useEffect, useMemo, useState } from "react";
import { Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { useToast } from "@/hooks/use-toast";
import { useCreateMCPHostedToolMutation, useUpdateMCPHostedToolMutation } from "@/lib/store/apis/mcpApi";
import type { MCPHostedTool } from "@/lib/types/mcp";

interface MCPHostedToolFormProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	tool?: MCPHostedTool | null;
	onSaved?: () => void | Promise<unknown>;
}

export function MCPHostedToolForm({ open, onOpenChange, tool, onSaved }: MCPHostedToolFormProps) {
	const { toast } = useToast();
	const isEditing = !!tool;
	const [createHostedTool, { isLoading: isCreating }] = useCreateMCPHostedToolMutation();
	const [updateHostedTool, { isLoading: isUpdating }] = useUpdateMCPHostedToolMutation();

	const initialHeaders = useMemo(() => {
		const entries = Object.entries(tool?.headers || {});
		return entries.length > 0 ? entries.map(([key, value]) => ({ key, value })) : [{ key: "Authorization", value: "{{req.header.authorization}}" }];
	}, [tool]);
	const initialQueryParams = useMemo(() => {
		const entries = Object.entries(tool?.query_params || {});
		return entries.length > 0 ? entries.map(([key, value]) => ({ key, value })) : [];
	}, [tool]);
	const initialAuthHeaderMappings = useMemo(() => {
		const entries = Object.entries(tool?.auth_profile?.header_mappings || {});
		return entries.length > 0 ? entries.map(([target, source]) => ({ target, source })) : [{ target: "", source: "" }];
	}, [tool]);

	const [name, setName] = useState("");
	const [method, setMethod] = useState("GET");
	const [url, setURL] = useState("");
	const [description, setDescription] = useState("");
	const [bodyTemplate, setBodyTemplate] = useState("");
	const [responseSchemaText, setResponseSchemaText] = useState("");
	const [responseExamplesText, setResponseExamplesText] = useState("");
	const [responseJSONPath, setResponseJSONPath] = useState("");
	const [responseTemplate, setResponseTemplate] = useState("");
	const [headers, setHeaders] = useState<{ key: string; value: string }[]>(initialHeaders);
	const [queryParams, setQueryParams] = useState<{ key: string; value: string }[]>(initialQueryParams);
	const [authMode, setAuthMode] = useState<"none" | "bearer_passthrough" | "header_passthrough">("none");
	const [authHeaderMappings, setAuthHeaderMappings] = useState<{ target: string; source: string }[]>(initialAuthHeaderMappings);
	const [timeoutSeconds, setTimeoutSeconds] = useState("");
	const [maxResponseBodyBytes, setMaxResponseBodyBytes] = useState("");

	useEffect(() => {
		if (!open) {
			return;
		}
		setName(tool?.name || "");
		setMethod(tool?.method || "GET");
		setURL(tool?.url || "");
		setDescription(tool?.description || "");
		setBodyTemplate(tool?.body_template || "");
		setResponseSchemaText(tool?.response_schema ? JSON.stringify(tool.response_schema, null, 2) : "");
		setResponseExamplesText(tool?.response_examples ? JSON.stringify(tool.response_examples, null, 2) : "");
		setResponseJSONPath(tool?.response_json_path || "");
		setResponseTemplate(tool?.response_template || "");
		setHeaders(initialHeaders);
		setQueryParams(initialQueryParams);
		setAuthMode(tool?.auth_profile?.mode || "none");
		setAuthHeaderMappings(initialAuthHeaderMappings);
		setTimeoutSeconds(tool?.execution_profile?.timeout_seconds ? String(tool.execution_profile.timeout_seconds) : "");
		setMaxResponseBodyBytes(tool?.execution_profile?.max_response_body_bytes ? String(tool.execution_profile.max_response_body_bytes) : "");
	}, [open, tool, initialHeaders, initialQueryParams, initialAuthHeaderMappings]);

	const isSaving = isCreating || isUpdating;

	const save = async () => {
		if (!name.trim()) {
			toast({ title: "Error", description: "Name is required.", variant: "destructive" });
			return;
		}
		if (!url.trim()) {
			toast({ title: "Error", description: "URL is required.", variant: "destructive" });
			return;
		}

		const normalizedHeaders = Object.fromEntries(
			headers
				.map((header) => [header.key.trim(), header.value] as const)
				.filter(([key]) => key.length > 0),
		);
		const normalizedQueryParams = Object.fromEntries(
			queryParams
				.map((query) => [query.key.trim(), query.value] as const)
				.filter(([key]) => key.length > 0),
		);
		const normalizedAuthHeaderMappings = Object.fromEntries(
			authHeaderMappings
				.map((mapping) => [mapping.target.trim(), mapping.source.trim().toLowerCase()] as const)
				.filter(([target, source]) => target.length > 0 && source.length > 0),
		);
		const parsedTimeoutSeconds = timeoutSeconds.trim() ? Number(timeoutSeconds.trim()) : undefined;
		const parsedMaxResponseBodyBytes = maxResponseBodyBytes.trim() ? Number(maxResponseBodyBytes.trim()) : undefined;
		let parsedResponseSchema: Record<string, any> | undefined;
		let parsedResponseExamples: any[] | undefined;
		if (parsedTimeoutSeconds !== undefined && (!Number.isFinite(parsedTimeoutSeconds) || parsedTimeoutSeconds <= 0)) {
			toast({ title: "Error", description: "Timeout seconds must be a positive number.", variant: "destructive" });
			return;
		}
		if (parsedMaxResponseBodyBytes !== undefined && (!Number.isFinite(parsedMaxResponseBodyBytes) || parsedMaxResponseBodyBytes <= 0)) {
			toast({ title: "Error", description: "Max response body bytes must be a positive number.", variant: "destructive" });
			return;
		}
		if (responseSchemaText.trim()) {
			try {
				parsedResponseSchema = JSON.parse(responseSchemaText);
			} catch {
				toast({ title: "Error", description: "Response schema must be valid JSON.", variant: "destructive" });
				return;
			}
		}
		if (responseExamplesText.trim()) {
			try {
				const parsed = JSON.parse(responseExamplesText);
				if (!Array.isArray(parsed)) {
					toast({ title: "Error", description: "Response examples must be a JSON array.", variant: "destructive" });
					return;
				}
				parsedResponseExamples = parsed;
			} catch {
				toast({ title: "Error", description: "Response examples must be valid JSON.", variant: "destructive" });
				return;
			}
		}

		const payload = {
			name: name.trim(),
			description: description.trim() || undefined,
			method,
			url: url.trim(),
			headers: Object.keys(normalizedHeaders).length > 0 ? normalizedHeaders : undefined,
			query_params: Object.keys(normalizedQueryParams).length > 0 ? normalizedQueryParams : undefined,
			auth_profile: authMode === "none"
				? undefined
				: authMode === "bearer_passthrough"
					? { mode: authMode }
					: {
						mode: authMode,
						header_mappings: Object.keys(normalizedAuthHeaderMappings).length > 0 ? normalizedAuthHeaderMappings : undefined,
					},
			execution_profile:
				parsedTimeoutSeconds !== undefined || parsedMaxResponseBodyBytes !== undefined
					? {
						timeout_seconds: parsedTimeoutSeconds,
						max_response_body_bytes: parsedMaxResponseBodyBytes,
					}
					: undefined,
			body_template: bodyTemplate.trim() || undefined,
			response_schema: parsedResponseSchema,
			response_examples: parsedResponseExamples,
			response_json_path: responseJSONPath.trim() || undefined,
			response_template: responseTemplate.trim() || undefined,
		};

		try {
			if (tool) {
				await updateHostedTool({ id: tool.tool_id, data: payload }).unwrap();
				toast({ title: "Saved", description: `Hosted tool ${tool.name} updated successfully.` });
			} else {
				await createHostedTool(payload).unwrap();
				toast({ title: "Created", description: `Hosted tool ${payload.name} created successfully.` });
			}
			await onSaved?.();
			onOpenChange(false);
		} catch (error: any) {
			toast({
				title: "Error",
				description: error?.data?.error?.message || "Failed to save hosted tool.",
				variant: "destructive",
			});
		}
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
				<DialogHeader>
					<DialogTitle>{isEditing ? "Edit Hosted API Tool" : "Create Hosted API Tool"}</DialogTitle>
					<DialogDescription>
						Configure an internal HTTP API as an in-process MCP tool. This only updates ConfigStore and MCP runtime registration, so it stays safe for cluster deployments.
					</DialogDescription>
				</DialogHeader>

				<div className="space-y-4">
					<div className="grid grid-cols-2 gap-4">
						<div className="space-y-2">
							<Label>Name</Label>
							<Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Get User Profile" />
						</div>
						<div className="space-y-2">
							<Label>Method</Label>
							<Select value={method} onValueChange={setMethod}>
								<SelectTrigger><SelectValue /></SelectTrigger>
								<SelectContent>
									{["GET", "POST", "PUT", "DELETE", "PATCH"].map((item) => (
										<SelectItem key={item} value={item}>{item}</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
					</div>

					<div className="space-y-2">
						<Label>URL</Label>
						<Input value={url} onChange={(e) => setURL(e.target.value)} placeholder="https://api.company.com/users/{{args.user_id}}" className="font-mono text-sm" />
					</div>

					<div className="space-y-2">
						<Label>Description</Label>
						<Textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={2} placeholder="Describe what this tool does for the LLM." />
					</div>

					<div className="space-y-2">
						<div className="flex items-center justify-between">
							<Label>Headers</Label>
							<Button type="button" variant="outline" size="sm" onClick={() => setHeaders((prev) => [...prev, { key: "", value: "" }])}>
								<Plus className="mr-1 h-3 w-3" /> Add Header
							</Button>
						</div>
						<div className="space-y-2">
							{headers.map((header, index) => (
								<div key={index} className="flex gap-2">
									<Input
										value={header.key}
										onChange={(e) =>
											setHeaders((prev) => prev.map((item, current) => (current === index ? { ...item, key: e.target.value } : item)))
										}
										placeholder="Header name"
									/>
									<Input
										value={header.value}
										onChange={(e) =>
											setHeaders((prev) => prev.map((item, current) => (current === index ? { ...item, value: e.target.value } : item)))
										}
										placeholder="{{req.header.authorization}}"
										className="font-mono text-xs"
									/>
									<Button type="button" variant="ghost" size="icon" onClick={() => setHeaders((prev) => prev.filter((_, current) => current !== index))}>
										<Trash2 className="h-4 w-4" />
									</Button>
								</div>
							))}
						</div>
					</div>

					<div className="space-y-2">
						<div className="flex items-center justify-between">
							<Label>Query Params</Label>
							<Button type="button" variant="outline" size="sm" onClick={() => setQueryParams((prev) => [...prev, { key: "", value: "" }])}>
								<Plus className="mr-1 h-3 w-3" /> Add Query Param
							</Button>
						</div>
						<div className="space-y-2">
							{queryParams.length === 0 ? (
								<p className="text-muted-foreground text-xs">Optional. Use this when you want stable query parameter mapping instead of embedding everything into the URL string.</p>
							) : (
								queryParams.map((query, index) => (
									<div key={index} className="flex gap-2">
										<Input
											value={query.key}
											onChange={(e) =>
												setQueryParams((prev) => prev.map((item, current) => (current === index ? { ...item, key: e.target.value } : item)))
											}
											placeholder="tenant_id"
										/>
										<Input
											value={query.value}
											onChange={(e) =>
												setQueryParams((prev) => prev.map((item, current) => (current === index ? { ...item, value: e.target.value } : item)))
											}
											placeholder="{{args.tenant_id}}"
											className="font-mono text-xs"
										/>
										<Button type="button" variant="ghost" size="icon" onClick={() => setQueryParams((prev) => prev.filter((_, current) => current !== index))}>
											<Trash2 className="h-4 w-4" />
										</Button>
									</div>
								))
							)}
						</div>
					</div>

					<div className="grid grid-cols-2 gap-4">
						<div className="space-y-2">
							<Label>Auth Profile</Label>
							<Select value={authMode} onValueChange={(value) => setAuthMode(value as "none" | "bearer_passthrough" | "header_passthrough")}>
								<SelectTrigger><SelectValue /></SelectTrigger>
								<SelectContent>
									<SelectItem value="none">None</SelectItem>
									<SelectItem value="bearer_passthrough">Bearer passthrough</SelectItem>
									<SelectItem value="header_passthrough">Header passthrough</SelectItem>
								</SelectContent>
							</Select>
							<p className="text-muted-foreground text-xs">
								Use an auth profile when this tool should reuse the caller&apos;s auth headers without hard-coding them in the static header list.
							</p>
						</div>
						<div className="space-y-2">
							<Label>Execution Profile</Label>
							<div className="grid grid-cols-2 gap-2">
								<Input
									value={timeoutSeconds}
									onChange={(e) => setTimeoutSeconds(e.target.value)}
									placeholder="Timeout seconds"
									type="number"
									min={1}
								/>
								<Input
									value={maxResponseBodyBytes}
									onChange={(e) => setMaxResponseBodyBytes(e.target.value)}
									placeholder="Max response bytes"
									type="number"
									min={1}
								/>
							</div>
							<p className="text-muted-foreground text-xs">
								Optional per-tool safety limits. Useful when a hosted API can return large payloads and you want tighter guardrails without touching the global gateway path.
							</p>
						</div>
					</div>

					{authMode === "header_passthrough" ? (
						<div className="space-y-2">
							<div className="flex items-center justify-between">
								<Label>Auth Header Mappings</Label>
								<Button type="button" variant="outline" size="sm" onClick={() => setAuthHeaderMappings((prev) => [...prev, { target: "", source: "" }])}>
									<Plus className="mr-1 h-3 w-3" /> Add Mapping
								</Button>
							</div>
							<div className="space-y-2">
								{authHeaderMappings.map((mapping, index) => (
									<div key={index} className="flex gap-2">
										<Input
											value={mapping.target}
											onChange={(e) =>
												setAuthHeaderMappings((prev) => prev.map((item, current) => (current === index ? { ...item, target: e.target.value } : item)))
											}
											placeholder="X-Tenant-ID"
										/>
										<Input
											value={mapping.source}
											onChange={(e) =>
												setAuthHeaderMappings((prev) => prev.map((item, current) => (current === index ? { ...item, source: e.target.value } : item)))
											}
											placeholder="x-tenant-id"
											className="font-mono text-xs"
										/>
										<Button type="button" variant="ghost" size="icon" onClick={() => setAuthHeaderMappings((prev) => prev.filter((_, current) => current !== index))}>
											<Trash2 className="h-4 w-4" />
										</Button>
									</div>
								))}
							</div>
							<p className="text-muted-foreground text-xs">
								Map target upstream headers to incoming request headers. Example: <code>X-Tenant-ID → x-tenant-id</code>.
							</p>
						</div>
					) : null}

					<div className="space-y-2">
						<Label>Body Template (optional)</Label>
						<Textarea
							value={bodyTemplate}
							onChange={(e) => setBodyTemplate(e.target.value)}
							rows={5}
							className="font-mono text-xs"
							placeholder='{"query":"{{args.query}}","tenant_id":"{{req.header.x-tenant-id}}"}'
						/>
						<p className="text-muted-foreground text-xs">
							Supported placeholders: <code>{"{{req.header.*}}"}</code>, <code>{"{{env.*}}"}</code>, <code>{"{{args.*}}"}</code>, <code>{"{{req.body.*}}"}</code>, <code>{"{{req.query.*}}"}</code>.
						</p>
					</div>

					<div className="space-y-2">
						<Label>Response Schema (optional)</Label>
						<Textarea
							value={responseSchemaText}
							onChange={(e) => setResponseSchemaText(e.target.value)}
							rows={6}
							className="font-mono text-xs"
							placeholder='{"type":"object","properties":{"summary":{"type":"string"}}}'
						/>
						<p className="text-muted-foreground text-xs">
							Optional JSON schema-like structure used for Hosted Tool preview and operator understanding. This does not change normal gateway routing behavior.
						</p>
					</div>

					<div className="space-y-2">
						<Label>Response Examples (optional)</Label>
						<Textarea
							value={responseExamplesText}
							onChange={(e) => setResponseExamplesText(e.target.value)}
							rows={5}
							className="font-mono text-xs"
							placeholder='[{"summary":"User profile loaded"},{"summary":"Tenant context missing"}]'
						/>
						<p className="text-muted-foreground text-xs">
							Optional JSON array used for operator guidance and import previews. Keep this small and representative so it stays safe to sync across the cluster.
						</p>
					</div>

					<div className="grid grid-cols-2 gap-4">
						<div className="space-y-2">
							<Label>Response JSON Path (optional)</Label>
							<Input
								value={responseJSONPath}
								onChange={(e) => setResponseJSONPath(e.target.value)}
								placeholder="data.summary"
								className="font-mono text-xs"
							/>
							<p className="text-muted-foreground text-xs">
								If set, Bifrost extracts only this JSON field from the upstream response before returning the tool result.
							</p>
						</div>
						<div className="space-y-2">
							<Label>Response Template (optional)</Label>
							<Textarea
								value={responseTemplate}
								onChange={(e) => setResponseTemplate(e.target.value)}
								rows={3}
								className="font-mono text-xs"
								placeholder={"User {{response.user.name}} belongs to {{response.team}}"}
							/>
							<p className="text-muted-foreground text-xs">
								Supports <code>{"{{response.*}}"}</code> placeholders and takes priority over JSON Path when both are set.
							</p>
						</div>
					</div>

					<div className="flex justify-end gap-2">
						<Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSaving}>Cancel</Button>
						<Button onClick={() => void save()} disabled={isSaving}>
							{isSaving ? "Saving..." : isEditing ? "Save Changes" : "Create Tool"}
						</Button>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}
