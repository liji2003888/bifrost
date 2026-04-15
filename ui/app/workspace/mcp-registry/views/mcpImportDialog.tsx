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

import { useCreateMCPClientMutation, useCreateMCPHostedToolMutation, useValidateMCPClientMutation } from "@/lib/store/apis/mcpApi";
import { getErrorMessage } from "@/lib/store";
import type { EnvVar, MCPHostedTool, ValidateMCPClientResponse } from "@/lib/types/mcp";

interface ParsedEndpoint {
	name: string;
	method: string;
	url: string;
	headers: Record<string, string>;
	queryParams?: Record<string, string>;
	authProfile?: MCPHostedTool["auth_profile"];
	body?: string;
	responseSchema?: MCPHostedTool["response_schema"];
	responseExamples?: MCPHostedTool["response_examples"];
	toolSchema?: MCPHostedTool["tool_schema"];
	selected: boolean;
	validation?: ValidateMCPClientResponse;
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

function splitURLAndQuery(url: string): { url: string; queryParams: Record<string, string> } {
	try {
		const parsed = new URL(url);
		const queryParams: Record<string, string> = {};
		for (const [key, value] of parsed.searchParams.entries()) {
			queryParams[key] = value;
		}
		parsed.search = "";
		return { url: parsed.toString(), queryParams };
	} catch {
		return { url, queryParams: {} };
	}
}

function resolveEndpointSecurityHeaders(headers: Record<string, string>): {
	headers: Record<string, string>;
	authProfile?: MCPHostedTool["auth_profile"];
} {
	const staticHeaders: Record<string, string> = {};
	const passthroughMappings: Record<string, string> = {};
	const exactRuntimeHeaderPattern = /^\{\{\s*req\.header\.([^}]+)\s*\}\}$/i;

	for (const [headerKey, headerValue] of Object.entries(headers)) {
		const match = headerValue.trim().match(exactRuntimeHeaderPattern);
		if (!match) {
			staticHeaders[headerKey] = headerValue;
			continue;
		}
		passthroughMappings[headerKey] = match[1].trim().toLowerCase();
	}

	if (Object.keys(passthroughMappings).length === 0) {
		return { headers: staticHeaders };
	}
	if (
		Object.keys(passthroughMappings).length === 1 &&
		passthroughMappings.Authorization === "authorization"
	) {
		return {
			headers: staticHeaders,
			authProfile: { mode: "bearer_passthrough" },
		};
	}
	return {
		headers: staticHeaders,
		authProfile: {
			mode: "header_passthrough",
			header_mappings: passthroughMappings,
		},
	};
}

function resolveOpenAPIRef(schema: any, spec: any): any {
	if (!schema || typeof schema !== "object") {
		return schema;
	}
	if (typeof schema.$ref !== "string" || !schema.$ref.startsWith("#/")) {
		return schema;
	}
	const parts = schema.$ref.slice(2).split("/");
	let current: any = spec;
	for (const part of parts) {
		if (!current || typeof current !== "object") {
			return schema;
		}
		current = current[part];
	}
	return current || schema;
}

function buildToolPropertyFromOpenAPISchema(schema: any, spec: any): any {
	const resolved = resolveOpenAPIRef(schema, spec) || {};
	const property: Record<string, any> = {};
	if (resolved.type) property.type = resolved.type;
	if (resolved.description) property.description = resolved.description;
	if (resolved.enum) property.enum = resolved.enum;
	if (resolved.format) property.format = resolved.format;
	if (resolved.default !== undefined) property.default = resolved.default;
	if (resolved.type === "object" && resolved.properties) {
		property.properties = Object.fromEntries(
			Object.entries(resolved.properties).map(([key, value]) => [key, buildToolPropertyFromOpenAPISchema(value, spec)]),
		);
		if (Array.isArray(resolved.required) && resolved.required.length > 0) {
			property.required = resolved.required;
		}
	} else if (resolved.type === "array" && resolved.items) {
		property.items = buildToolPropertyFromOpenAPISchema(resolved.items, spec);
	}
	if (!property.type) {
		property.type = "string";
	}
	return property;
}

function buildBodyTemplateFromOpenAPISchema(schema: any, spec: any, pathPrefix = ""): any {
	const resolved = resolveOpenAPIRef(schema, spec) || {};
	if (resolved.type === "object" && resolved.properties) {
		return Object.fromEntries(
			Object.entries(resolved.properties).map(([key, value]) => {
				const nextPrefix = pathPrefix ? `${pathPrefix}.${key}` : key;
				return [key, buildBodyTemplateFromOpenAPISchema(value, spec, nextPrefix)];
			}),
		);
	}
	return `{{args.${pathPrefix}}}`;
}

function buildHostedToolSchema(name: string, description: string | undefined, properties: Record<string, any>, required: string[]): MCPHostedTool["tool_schema"] {
	return {
		type: "function",
		function: {
			name,
			description,
			parameters: {
				type: "object",
				properties,
				required,
			},
		},
	};
}

function extractOpenAPIResponseSchema(op: any, spec: any): MCPHostedTool["response_schema"] | undefined {
	const responses = op?.responses;
	if (!responses || typeof responses !== "object") {
		return undefined;
	}
	const preferredStatusCode = Object.keys(responses)
		.filter((code) => /^2\d\d$/.test(code))
		.sort()[0];
	const response = responses[preferredStatusCode] || responses.default;
	if (!response || typeof response !== "object") {
		return undefined;
	}
	const content = response.content || {};
	const mediaType =
		content["application/json"] ||
		content["application/*+json"] ||
		Object.values(content)[0];
	const schema = mediaType && typeof mediaType === "object" ? (mediaType as any).schema : undefined;
	if (!schema) {
		return undefined;
	}
	return buildToolPropertyFromOpenAPISchema(schema, spec);
}

function extractOpenAPIResponseExamples(op: any): MCPHostedTool["response_examples"] | undefined {
	const responses = op?.responses;
	if (!responses || typeof responses !== "object") {
		return undefined;
	}
	const preferredStatusCode = Object.keys(responses)
		.filter((code) => /^2\d\d$/.test(code))
		.sort()[0];
	const response = responses[preferredStatusCode] || responses.default;
	if (!response || typeof response !== "object") {
		return undefined;
	}
	const content = response.content || {};
	const mediaType =
		content["application/json"] ||
		content["application/*+json"] ||
		Object.values(content)[0];
	if (!mediaType || typeof mediaType !== "object") {
		return undefined;
	}
	const directExample = (mediaType as any).example;
	if (directExample !== undefined) {
		return [directExample];
	}
	const examples = (mediaType as any).examples;
	if (!examples || typeof examples !== "object") {
		return undefined;
	}
	const values = Object.values(examples)
		.map((entry: any) => entry?.value)
		.filter((entry: any) => entry !== undefined);
	return values.length > 0 ? values : undefined;
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
			let queryParams: Record<string, string> = {};
			if (Array.isArray(req.url?.query)) {
				for (const item of req.url.query) {
					if (item?.key && item?.value) {
						queryParams[item.key] = item.value;
					}
				}
			}
			const split = splitURLAndQuery(url);
			url = split.url;
			queryParams = { ...split.queryParams, ...queryParams };
			const resolvedSecurity = resolveEndpointSecurityHeaders(headers);
			const toolSchema = buildHostedToolSchema(
				item.name || `${method} ${url}`,
				item.request?.description,
				{},
				[],
			);
			endpoints.push({
				name: item.name || `${method} ${url}`,
				method,
				url,
				headers: resolvedSecurity.headers,
				queryParams,
				authProfile: resolvedSecurity.authProfile,
				body: req.body?.raw,
				responseSchema: undefined,
				responseExamples: undefined,
				toolSchema,
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
	const paths = spec.paths || {};
	const servers = spec.servers || [];
	const baseUrl = servers[0]?.url || "";

	for (const [path, methods] of Object.entries(paths)) {
		if (typeof methods !== "object" || methods === null) continue;
		for (const [method, operation] of Object.entries(methods as Record<string, any>)) {
			if (["get", "post", "put", "delete", "patch"].indexOf(method.toLowerCase()) === -1) continue;
			const op = operation as any;
			const headers: Record<string, string> = {};
			const queryParams: Record<string, string> = {};
			const toolProperties: Record<string, any> = {};
			const requiredArgs = new Set<string>();
			let resolvedPath = path;
			const combinedParameters = [
				...((Array.isArray((methods as any).parameters) ? (methods as any).parameters : []) as any[]),
				...(Array.isArray(op.parameters) ? op.parameters : []),
			];
			if (combinedParameters.length > 0) {
				for (const param of combinedParameters) {
					if (param.in === "header" && param.name) {
						headers[param.name] = param.example || param.schema?.default || `{{req.header.${param.name.toLowerCase()}}}`;
					} else if (param.in === "query" && param.name) {
						queryParams[param.name] = param.example || param.schema?.default || `{{args.${param.name}}}`;
						toolProperties[param.name] = buildToolPropertyFromOpenAPISchema(param.schema, spec);
						if (param.required) requiredArgs.add(param.name);
					} else if (param.in === "path" && param.name) {
						resolvedPath = resolvedPath.replace(`{${param.name}}`, `{{args.${param.name}}}`);
						toolProperties[param.name] = buildToolPropertyFromOpenAPISchema(param.schema, spec);
						requiredArgs.add(param.name);
					}
				}
			}
			// Add security scheme headers
			const applicableSecurity = Array.isArray(op.security) ? op.security : Array.isArray(spec.security) ? spec.security : [];
			if (applicableSecurity.length > 0) {
				const schemes = spec.components?.securitySchemes || {};
				for (const requirement of applicableSecurity) {
					for (const name of Object.keys(requirement)) {
						const scheme = schemes[name];
						if (!scheme) {
							continue;
						}
						const s = scheme as any;
						if (s.type === "http" && s.scheme === "bearer") {
							headers["Authorization"] = "{{req.header.authorization}}";
						} else if (s.type === "apiKey" && s.in === "header") {
							headers[s.name] = `{{req.header.${s.name.toLowerCase()}}}`;
						}
					}
				}
			}
			const split = splitURLAndQuery(`${baseUrl}${resolvedPath}`);
			const resolvedSecurity = resolveEndpointSecurityHeaders(headers);
			let bodyTemplate: string | undefined;
			const contentSchema = op.requestBody?.content?.["application/json"]?.schema;
			if (contentSchema) {
				const resolvedBodySchema = resolveOpenAPIRef(contentSchema, spec);
				if (resolvedBodySchema?.type === "object" && resolvedBodySchema.properties) {
					for (const [key, value] of Object.entries(resolvedBodySchema.properties)) {
						toolProperties[key] = buildToolPropertyFromOpenAPISchema(value, spec);
					}
					for (const item of resolvedBodySchema.required || []) {
						requiredArgs.add(String(item));
					}
					bodyTemplate = JSON.stringify(buildBodyTemplateFromOpenAPISchema(resolvedBodySchema, spec), null, 2);
				}
			}
			const name = op.summary || op.operationId || `${method.toUpperCase()} ${path}`;
			endpoints.push({
				name,
				method: method.toUpperCase(),
				url: split.url,
				headers: resolvedSecurity.headers,
				queryParams: { ...split.queryParams, ...queryParams },
				authProfile: resolvedSecurity.authProfile,
				body: bodyTemplate,
				responseSchema: extractOpenAPIResponseSchema(op, spec),
				responseExamples: extractOpenAPIResponseExamples(op),
				toolSchema: buildHostedToolSchema(name, op.description || op.summary, toolProperties, Array.from(requiredArgs)),
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
		const split = splitURLAndQuery(url);
		url = split.url;
		const urlObj = (() => {
			try { return new URL(url); } catch { return null; }
		})();
		const pathName = urlObj ? urlObj.pathname.split("/").filter(Boolean).join(" ") : `endpoint-${i + 1}`;
		const resolvedSecurity = resolveEndpointSecurityHeaders(headers);
		const toolSchema = buildHostedToolSchema(`${method} ${pathName}`, undefined, {}, []);

		return {
			name: `${method} ${pathName}`,
			method,
			url,
			headers: resolvedSecurity.headers,
			queryParams: split.queryParams,
			authProfile: resolvedSecurity.authProfile,
			body,
			responseSchema: undefined,
			responseExamples: undefined,
			toolSchema,
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
	const [createMCPHostedTool] = useCreateMCPHostedToolMutation();
	const [validateMCPClient] = useValidateMCPClientMutation();

	const validateEndpoint = async (ep: ParsedEndpoint): Promise<ValidateMCPClientResponse> => {
		return await validateMCPClient({
			name: ep.name,
			connection_type: "http",
			connection_string: envVar(ep.url),
			auth_type: Object.keys(ep.headers).length > 0 ? "headers" : "none",
			headers: Object.keys(ep.headers).length > 0 ? headersToEnvVars(ep.headers) : undefined,
		}).unwrap();
	};

	const getValidationBadgeVariant = (status?: ValidateMCPClientResponse["status"]) => {
		switch (status) {
			case "compatible":
				return "default";
			case "unverified":
				return "secondary";
			case "incompatible":
				return "destructive";
			default:
				return "outline";
		}
	};

	const getValidationLabel = (status?: ValidateMCPClientResponse["status"]) => {
		switch (status) {
			case "compatible":
				return "MCP-compatible";
			case "unverified":
				return "Hosted tool candidate";
			case "incompatible":
				return "Hosted tool candidate";
			default:
				return "Not validated";
		}
	};

	const createHostedToolFromEndpoint = async (ep: ParsedEndpoint) => {
		await createMCPHostedTool({
			name: ep.name,
			method: ep.method,
			url: ep.url,
			headers: Object.keys(ep.headers).length > 0 ? ep.headers : undefined,
			query_params: ep.queryParams && Object.keys(ep.queryParams).length > 0 ? ep.queryParams : undefined,
			auth_profile: ep.authProfile,
			body_template: ep.body,
			response_schema: ep.responseSchema,
			response_examples: ep.responseExamples,
			tool_schema: ep.toolSchema,
		}).unwrap();
	};

	const handleValidateEndpoints = async () => {
		const selected = endpoints.filter((ep) => ep.selected);
		if (selected.length === 0) {
			toast.error("No endpoints selected for validation");
			return;
		}

		const results = await Promise.all(
			endpoints.map(async (ep) => {
				if (!ep.selected) return ep;
				try {
					const validation = await validateEndpoint(ep);
					return { ...ep, validation };
				} catch (err) {
					return {
						...ep,
						validation: {
							status: "incompatible" as const,
							message: getErrorMessage(err),
							reason: "validation_failed",
						},
					};
				}
			}),
		);
		setEndpoints(results);
	};

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
			setEndpoints(parsed.map((ep) => ({ ...ep, validation: undefined })));
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
		let mcpClientCount = 0;
		let hostedToolCount = 0;
		let errorCount = 0;

		for (const ep of selected) {
			try {
				const validation = ep.validation ?? await validateEndpoint(ep);
				if (validation.status === "compatible") {
					await createMCPClient({
						name: ep.name,
						connection_type: "http",
						connection_string: envVar(ep.url),
						auth_type: Object.keys(ep.headers).length > 0 ? "headers" : "none",
						headers: Object.keys(ep.headers).length > 0 ? headersToEnvVars(ep.headers) : undefined,
						is_ping_available: false,
					}).unwrap();
					mcpClientCount++;
				} else {
					await createHostedToolFromEndpoint(ep);
					hostedToolCount++;
				}
			} catch {
				errorCount++;
			}
		}

		setIsImporting(false);
		if (mcpClientCount > 0 || hostedToolCount > 0) {
			const parts: string[] = [];
			if (mcpClientCount > 0) {
				parts.push(`${mcpClientCount} MCP server${mcpClientCount > 1 ? "s" : ""}`);
			}
			if (hostedToolCount > 0) {
				parts.push(`${hostedToolCount} hosted tool${hostedToolCount > 1 ? "s" : ""}`);
			}
			toast.success(`Imported ${parts.join(" and ")}`);
		}
		if (errorCount > 0) {
			toast.error(`Skipped or failed ${errorCount} endpoint${errorCount > 1 ? "s" : ""}`);
		}
		if (mcpClientCount > 0 || hostedToolCount > 0) {
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
			const split = splitURLAndQuery(manualUrl);
			const staticHeaders = Object.fromEntries(
				Object.entries(headers).map(([key, value]) => [key, value.value || ""]),
			);
			const resolvedSecurity = resolveEndpointSecurityHeaders(staticHeaders);
			const validation = await validateMCPClient({
				name: manualName || `${manualMethod} API`,
				connection_type: "http",
				connection_string: envVar(split.url),
				auth_type: Object.keys(headers).length > 0 ? "headers" : "none",
				headers: Object.keys(headers).length > 0 ? headers : undefined,
			}).unwrap();
			if (validation.status === "compatible") {
				await createMCPClient({
					name: manualName || `${manualMethod} API`,
					connection_type: "http",
					connection_string: envVar(split.url),
					auth_type: Object.keys(headers).length > 0 ? "headers" : "none",
					headers: Object.keys(headers).length > 0 ? headers : undefined,
					is_ping_available: false,
				}).unwrap();
				toast.success("MCP server added successfully");
			} else {
				await createMCPHostedTool({
					name: manualName || `${manualMethod} API`,
					method: manualMethod,
					url: split.url,
					headers: Object.keys(resolvedSecurity.headers).length > 0 ? resolvedSecurity.headers : undefined,
					query_params: Object.keys(split.queryParams).length > 0 ? split.queryParams : undefined,
					auth_profile: resolvedSecurity.authProfile,
					body_template: manualBody.trim() ? manualBody : undefined,
					response_schema: undefined,
					response_examples: undefined,
					tool_schema: buildHostedToolSchema(manualName || `${manualMethod} API`, undefined, {}, []),
				}).unwrap();
				toast.success("Hosted MCP tool added successfully");
			}
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
					<DialogTitle>Import MCP Servers or Hosted API Tools</DialogTitle>
					<DialogDescription>
						Import streamable HTTP or SSE MCP servers directly. Ordinary enterprise APIs that are not MCP-compatible will be imported as hosted MCP tools and executed in-process with request header and environment-variable templating.
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
								<Button variant="outline" onClick={handleValidateEndpoints} disabled={endpoints.filter((e) => e.selected).length === 0}>
									Analyze Import Mode
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
												<TableHead>Status</TableHead>
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
													<TableCell className="space-y-1">
														<Badge variant={getValidationBadgeVariant(ep.validation?.status)}>
															{getValidationLabel(ep.validation?.status)}
														</Badge>
														{ep.validation?.message ? (
															<p className="text-muted-foreground max-w-xs text-[11px] leading-4">{ep.validation.message}</p>
														) : null}
													</TableCell>
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
								<Upload className="h-4 w-4 mr-1" /> Analyze & Add
							</Button>
						</div>
					</TabsContent>
				</Tabs>
			</DialogContent>
		</Dialog>
	);
}
