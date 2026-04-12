"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Upload, FileJson, Terminal, Globe, Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";

import { useCreateMCPClientMutation } from "@/lib/store/apis/mcpApi";
import { getErrorMessage } from "@/lib/store";
import type { EnvVar } from "@/lib/types/mcp";

interface ParsedEndpoint {
	name: string;
	method: string;
	url: string;
	headers: Record<string, string>;
	body?: string;
	selected: boolean;
}

function envVar(val: string): EnvVar {
	return { value: val, env_var: "", from_env: false };
}

function headersToEnvVars(headers: Record<string, string>): Record<string, EnvVar> {
	const result: Record<string, EnvVar> = {};
	for (const [k, v] of Object.entries(headers)) {
		result[k] = envVar(v);
	}
	return result;
}

// ─── Postman Parser ──────────────────────────────────────────────────────────

function parsePostmanCollection(json: string): ParsedEndpoint[] {
	const collection = JSON.parse(json);
	const endpoints: ParsedEndpoint[] = [];

	function processItems(items: any[], prefix = "") {
		for (const item of items) {
			if (item.item) {
				processItems(item.item, prefix ? `${prefix}/${item.name}` : item.name);
				continue;
			}
			if (!item.request) continue;
			const req = item.request;
			const method = typeof req.method === "string" ? req.method : "GET";
			let url = "";
			if (typeof req.url === "string") {
				url = req.url;
			} else if (req.url?.raw) {
				url = req.url.raw;
			}
			const headers: Record<string, string> = {};
			if (Array.isArray(req.header)) {
				for (const h of req.header) {
					if (h.key && h.value) headers[h.key] = h.value;
				}
			}
			endpoints.push({
				name: item.name || `${method} ${url}`,
				method,
				url,
				headers,
				body: req.body?.raw,
				selected: true,
			});
		}
	}

	if (collection.item) {
		processItems(collection.item);
	}
	return endpoints;
}

// ─── OpenAPI Parser ──────────────────────────────────────────────────────────

function parseOpenAPISpec(text: string): ParsedEndpoint[] {
	let spec: any;
	try {
		spec = JSON.parse(text);
	} catch {
		// Try basic YAML parsing (key: value)
		try {
			spec = simpleYamlParse(text);
		} catch {
			throw new Error("Invalid JSON or YAML format");
		}
	}

	const endpoints: ParsedEndpoint[] = [];
	const title = spec.info?.title || "API";
	const paths = spec.paths || {};
	const servers = spec.servers || [];
	const baseUrl = servers[0]?.url || "";

	for (const [path, methods] of Object.entries(paths)) {
		if (typeof methods !== "object" || methods === null) continue;
		for (const [method, operation] of Object.entries(methods as Record<string, any>)) {
			if (["get", "post", "put", "delete", "patch"].indexOf(method.toLowerCase()) === -1) continue;
			const op = operation as any;
			const headers: Record<string, string> = {};
			if (Array.isArray(op.parameters)) {
				for (const param of op.parameters) {
					if (param.in === "header" && param.name) {
						headers[param.name] = param.example || param.schema?.default || `{{req.header.${param.name.toLowerCase()}}}`;
					}
				}
			}
			// Add security scheme headers
			if (op.security || spec.security) {
				const schemes = spec.components?.securitySchemes || {};
				for (const [name, scheme] of Object.entries(schemes)) {
					const s = scheme as any;
					if (s.type === "http" && s.scheme === "bearer") {
						headers["Authorization"] = "{{req.header.authorization}}";
					} else if (s.type === "apiKey" && s.in === "header") {
						headers[s.name] = `{{req.header.${s.name.toLowerCase()}}}`;
					}
				}
			}
			endpoints.push({
				name: op.summary || op.operationId || `${method.toUpperCase()} ${path}`,
				method: method.toUpperCase(),
				url: `${baseUrl}${path}`,
				headers,
				selected: true,
			});
		}
	}
	return endpoints;
}

function simpleYamlParse(yaml: string): any {
	// Very basic YAML to JSON — handles simple key-value, nested objects via indentation
	// For full YAML support, js-yaml would be needed
	const lines = yaml.split("\n");
	const result: any = {};
	const stack: { obj: any; indent: number }[] = [{ obj: result, indent: -1 }];

	for (const line of lines) {
		const trimmed = line.trimEnd();
		if (!trimmed || trimmed.startsWith("#")) continue;
		const indent = line.length - line.trimStart().length;
		const match = trimmed.trim().match(/^([^:]+):\s*(.*)$/);
		if (!match) continue;
		const key = match[1].trim();
		const value = match[2].trim();

		while (stack.length > 1 && stack[stack.length - 1].indent >= indent) {
			stack.pop();
		}
		const parent = stack[stack.length - 1].obj;

		if (value) {
			// Remove quotes
			parent[key] = value.replace(/^["']|["']$/g, "");
		} else {
			parent[key] = {};
			stack.push({ obj: parent[key], indent });
		}
	}
	return result;
}

// ─── cURL Parser ─────────────────────────────────────────────────────────────

function parseCurlCommands(text: string): ParsedEndpoint[] {
	// Split by "curl " at line start (handling multi-line with \)
	const normalized = text.replace(/\\\n\s*/g, " ");
	const commands = normalized.split(/(?=curl\s)/i).filter((c) => c.trim().startsWith("curl"));

	return commands.map((cmd, i) => {
		let method = "GET";
		let url = "";
		const headers: Record<string, string> = {};
		let body: string | undefined;

		// Extract method
		const methodMatch = cmd.match(/-X\s+(\w+)/);
		if (methodMatch) method = methodMatch[1].toUpperCase();

		// Extract URL (first quoted string or bare URL after curl flags)
		const urlMatch = cmd.match(/["']?(https?:\/\/[^\s"']+)["']?/);
		if (urlMatch) url = urlMatch[1];

		// Extract headers
		const headerRegex = /-H\s+["']([^"']+)["']/g;
		let hm;
		while ((hm = headerRegex.exec(cmd)) !== null) {
			const colonIdx = hm[1].indexOf(":");
			if (colonIdx > 0) {
				const key = hm[1].substring(0, colonIdx).trim();
				const value = hm[1].substring(colonIdx + 1).trim();
				headers[key] = value;
			}
		}

		// Extract body
		const bodyMatch = cmd.match(/-d\s+['"]([^'"]+)['"]/);
		if (bodyMatch) {
			body = bodyMatch[1];
			if (method === "GET") method = "POST";
		}

		// Derive name from URL path
		const urlObj = (() => {
			try { return new URL(url); } catch { return null; }
		})();
		const pathName = urlObj ? urlObj.pathname.split("/").filter(Boolean).join(" ") : `endpoint-${i + 1}`;

		return {
			name: `${method} ${pathName}`,
			method,
			url,
			headers,
			body,
			selected: true,
		};
	});
}

// ─── Import Dialog ───────────────────────────────────────────────────────────

interface MCPImportDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onImported?: () => void;
}

export function MCPImportDialog({ open, onOpenChange, onImported }: MCPImportDialogProps) {
	const [tab, setTab] = useState("postman");
	const [inputText, setInputText] = useState("");
	const [endpoints, setEndpoints] = useState<ParsedEndpoint[]>([]);
	const [parseError, setParseError] = useState("");
	const [isImporting, setIsImporting] = useState(false);

	// Manual API Builder state
	const [manualMethod, setManualMethod] = useState("GET");
	const [manualUrl, setManualUrl] = useState("");
	const [manualName, setManualName] = useState("");
	const [manualHeaders, setManualHeaders] = useState<{ key: string; value: string }[]>([{ key: "Authorization", value: "{{req.header.authorization}}" }]);
	const [manualBody, setManualBody] = useState("");

	const [createMCPClient] = useCreateMCPClientMutation();

	const handleParse = () => {
		setParseError("");
		setEndpoints([]);
		try {
			let parsed: ParsedEndpoint[];
			switch (tab) {
				case "postman":
					parsed = parsePostmanCollection(inputText);
					break;
				case "openapi":
					parsed = parseOpenAPISpec(inputText);
					break;
				case "curl":
					parsed = parseCurlCommands(inputText);
					break;
				default:
					return;
			}
			if (parsed.length === 0) {
				setParseError("No API endpoints found in the input.");
				return;
			}
			setEndpoints(parsed);
		} catch (err: any) {
			setParseError(err.message || "Failed to parse input");
		}
	};

	const toggleEndpoint = (index: number) => {
		setEndpoints((prev) =>
			prev.map((ep, i) => (i === index ? { ...ep, selected: !ep.selected } : ep)),
		);
	};

	const handleImport = async () => {
		const selected = endpoints.filter((ep) => ep.selected);
		if (selected.length === 0) {
			toast.error("No endpoints selected for import");
			return;
		}

		setIsImporting(true);
		let successCount = 0;
		let errorCount = 0;

		for (const ep of selected) {
			try {
				await createMCPClient({
					name: ep.name,
					connection_type: "http",
					connection_string: envVar(ep.url),
					auth_type: Object.keys(ep.headers).length > 0 ? "headers" : "none",
					headers: Object.keys(ep.headers).length > 0 ? headersToEnvVars(ep.headers) : undefined,
					is_ping_available: false,
				}).unwrap();
				successCount++;
			} catch {
				errorCount++;
			}
		}

		setIsImporting(false);
		if (successCount > 0) {
			toast.success(`Imported ${successCount} API endpoint${successCount > 1 ? "s" : ""} as MCP tools`);
		}
		if (errorCount > 0) {
			toast.error(`Failed to import ${errorCount} endpoint${errorCount > 1 ? "s" : ""}`);
		}
		if (successCount > 0) {
			onImported?.();
			onOpenChange(false);
		}
	};

	const handleManualAdd = async () => {
		if (!manualUrl.trim()) {
			toast.error("URL is required");
			return;
		}
		const headers: Record<string, EnvVar> = {};
		for (const h of manualHeaders) {
			if (h.key.trim()) headers[h.key.trim()] = envVar(h.value);
		}

		try {
			await createMCPClient({
				name: manualName || `${manualMethod} API`,
				connection_type: "http",
				connection_string: envVar(manualUrl),
				auth_type: Object.keys(headers).length > 0 ? "headers" : "none",
				headers: Object.keys(headers).length > 0 ? headers : undefined,
				is_ping_available: false,
			}).unwrap();
			toast.success("API endpoint added as MCP tool");
			onImported?.();
			onOpenChange(false);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	const resetState = () => {
		setInputText("");
		setEndpoints([]);
		setParseError("");
		setManualUrl("");
		setManualName("");
		setManualMethod("GET");
		setManualHeaders([{ key: "Authorization", value: "{{req.header.authorization}}" }]);
		setManualBody("");
	};

	return (
		<Dialog open={open} onOpenChange={(v) => { if (!v) resetState(); onOpenChange(v); }}>
			<DialogContent className="max-h-[90vh] max-w-4xl overflow-y-auto">
				<DialogHeader>
					<DialogTitle>Import APIs as MCP Tools</DialogTitle>
					<DialogDescription>
						Transform your existing APIs into LLM-ready MCP tools. Import from Postman, OpenAPI, cURL, or configure manually.
					</DialogDescription>
				</DialogHeader>

				<Tabs value={tab} onValueChange={(v) => { setTab(v); setEndpoints([]); setParseError(""); }}>
					<TabsList className="grid w-full grid-cols-4">
						<TabsTrigger value="postman" className="flex items-center gap-1.5">
							<FileJson className="h-3.5 w-3.5" /> Postman
						</TabsTrigger>
						<TabsTrigger value="openapi" className="flex items-center gap-1.5">
							<Globe className="h-3.5 w-3.5" /> OpenAPI
						</TabsTrigger>
						<TabsTrigger value="curl" className="flex items-center gap-1.5">
							<Terminal className="h-3.5 w-3.5" /> cURL
						</TabsTrigger>
						<TabsTrigger value="manual" className="flex items-center gap-1.5">
							<Plus className="h-3.5 w-3.5" /> Manual
						</TabsTrigger>
					</TabsList>

					{/* Postman / OpenAPI / cURL */}
					{["postman", "openapi", "curl"].map((t) => (
						<TabsContent key={t} value={t} className="space-y-4">
							<div className="space-y-2">
								<Label>
									{t === "postman" ? "Paste Postman Collection JSON" : t === "openapi" ? "Paste OpenAPI 3.0+ Spec (JSON or YAML)" : "Paste cURL Commands"}
								</Label>
								<Textarea
									value={inputText}
									onChange={(e) => setInputText(e.target.value)}
									rows={8}
									className="font-mono text-xs"
									placeholder={
										t === "postman"
											? '{\n  "info": { "name": "My API" },\n  "item": [...]\n}'
											: t === "openapi"
												? 'openapi: "3.0.0"\ninfo:\n  title: Enterprise API\npaths:\n  /users/profile:\n    get: ...'
												: 'curl -X GET "https://api.company.com/users" \\\n  -H "Authorization: {{req.header.authorization}}"'
									}
								/>
								{parseError && <p className="text-destructive text-sm">{parseError}</p>}
							</div>
							<div className="flex gap-2">
								<Button onClick={handleParse} disabled={!inputText.trim()}>
									Parse
								</Button>
							</div>

							{/* Preview Table */}
							{endpoints.length > 0 && (
								<div className="space-y-3">
									<h4 className="text-sm font-medium">Parsed Endpoints ({endpoints.filter((e) => e.selected).length} selected)</h4>
									<Table>
										<TableHeader>
											<TableRow>
												<TableHead className="w-8"></TableHead>
												<TableHead>Name</TableHead>
												<TableHead>Method</TableHead>
												<TableHead>URL</TableHead>
												<TableHead>Headers</TableHead>
											</TableRow>
										</TableHeader>
										<TableBody>
											{endpoints.map((ep, i) => (
												<TableRow key={i} className={ep.selected ? "" : "opacity-50"}>
													<TableCell>
														<input type="checkbox" checked={ep.selected} onChange={() => toggleEndpoint(i)} />
													</TableCell>
													<TableCell className="font-medium text-xs">{ep.name}</TableCell>
													<TableCell><Badge variant="outline">{ep.method}</Badge></TableCell>
													<TableCell className="font-mono text-xs max-w-xs truncate">{ep.url}</TableCell>
													<TableCell className="text-xs">{Object.keys(ep.headers).length} header{Object.keys(ep.headers).length !== 1 ? "s" : ""}</TableCell>
												</TableRow>
											))}
										</TableBody>
									</Table>
									<div className="flex justify-end">
										<Button onClick={handleImport} disabled={isImporting || endpoints.filter((e) => e.selected).length === 0}>
											{isImporting ? "Importing..." : `Import ${endpoints.filter((e) => e.selected).length} Endpoint${endpoints.filter((e) => e.selected).length !== 1 ? "s" : ""}`}
										</Button>
									</div>
								</div>
							)}
						</TabsContent>
					))}

					{/* Manual API Builder */}
					<TabsContent value="manual" className="space-y-4">
						<div className="grid grid-cols-2 gap-4">
							<div className="space-y-2">
								<Label>Name</Label>
								<Input value={manualName} onChange={(e) => setManualName(e.target.value)} placeholder="e.g., Get User Profile" />
							</div>
							<div className="space-y-2">
								<Label>Method</Label>
								<Select value={manualMethod} onValueChange={setManualMethod}>
									<SelectTrigger><SelectValue /></SelectTrigger>
									<SelectContent>
										{["GET", "POST", "PUT", "DELETE", "PATCH"].map((m) => (
											<SelectItem key={m} value={m}>{m}</SelectItem>
										))}
									</SelectContent>
								</Select>
							</div>
						</div>
						<div className="space-y-2">
							<Label>URL</Label>
							<Input value={manualUrl} onChange={(e) => setManualUrl(e.target.value)} placeholder="https://api.company.com/users/profile" className="font-mono text-sm" />
						</div>

						<div className="space-y-2">
							<div className="flex items-center justify-between">
								<Label>Headers</Label>
								<Button variant="outline" size="sm" onClick={() => setManualHeaders((prev) => [...prev, { key: "", value: "" }])}>
									<Plus className="h-3 w-3 mr-1" /> Add Header
								</Button>
							</div>
							<div className="space-y-2">
								{manualHeaders.map((h, i) => (
									<div key={i} className="flex gap-2">
										<Input value={h.key} onChange={(e) => setManualHeaders((prev) => prev.map((hh, j) => j === i ? { ...hh, key: e.target.value } : hh))} placeholder="Header name" className="flex-1" />
										<Input value={h.value} onChange={(e) => setManualHeaders((prev) => prev.map((hh, j) => j === i ? { ...hh, value: e.target.value } : hh))} placeholder="Value or {{req.header.name}}" className="flex-1 font-mono text-xs" />
										<Button variant="ghost" size="sm" onClick={() => setManualHeaders((prev) => prev.filter((_, j) => j !== i))}><Trash2 className="h-3.5 w-3.5" /></Button>
									</div>
								))}
							</div>
							<p className="text-muted-foreground text-xs">
								Use <code className="bg-muted px-1 rounded">{"{{req.header.authorization}}"}</code> to forward the caller&apos;s auth token, or <code className="bg-muted px-1 rounded">{"{{env.API_KEY}}"}</code> for environment variables.
							</p>
						</div>

						{["POST", "PUT", "PATCH"].includes(manualMethod) && (
							<div className="space-y-2">
								<Label>Request Body (optional)</Label>
								<Textarea value={manualBody} onChange={(e) => setManualBody(e.target.value)} rows={4} className="font-mono text-xs" placeholder='{"query": "..."}' />
							</div>
						)}

						<div className="flex justify-end">
							<Button onClick={handleManualAdd}>
								<Upload className="h-4 w-4 mr-1" /> Add as MCP Tool
							</Button>
						</div>
					</TabsContent>
				</Tabs>
			</DialogContent>
		</Dialog>
	);
}
