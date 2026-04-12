/**
 * MCP Tool Groups View
 * Organize and manage MCP tools across servers into logical groups
 */

"use client";

import { useState, useMemo } from "react";
import { toast } from "sonner";
import { Plus, Search, ToolCase, Server, Check, X, ChevronDown, ChevronRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	Collapsible,
	CollapsibleContent,
	CollapsibleTrigger,
} from "@/components/ui/collapsible";

import { useGetMCPClientsQuery, useUpdateMCPClientMutation } from "@/lib/store/apis/mcpApi";
import { getErrorMessage } from "@/lib/store";
import type { MCPClient } from "@/lib/types/mcp";

export default function MCPToolGroups() {
	const { data: mcpData, isLoading, refetch } = useGetMCPClientsQuery();
	const [updateMCPClient] = useUpdateMCPClientMutation();
	const [search, setSearch] = useState("");
	const [expandedServers, setExpandedServers] = useState<Set<string>>(new Set());

	const clients = mcpData?.clients ?? [];

	// Build a view of all tools grouped by MCP server
	const serverTools = useMemo(() => {
		return clients
			.filter((c) => c.tools && c.tools.length > 0)
			.map((client) => {
				const tools = client.tools.filter((tool) => {
					if (!search) return true;
					const q = search.toLowerCase();
					return (
						tool.name?.toLowerCase().includes(q) ||
						tool.description?.toLowerCase().includes(q)
					);
				});
				return {
					clientId: client.config.client_id,
					name: client.config.name,
					state: client.state,
					connectionType: client.config.connection_type,
					tools,
					allTools: client.tools,
					toolsToExecute: new Set(client.config.tools_to_execute ?? []),
					toolsToAutoExecute: new Set(client.config.tools_to_auto_execute ?? []),
				};
			})
			.filter((s) => s.tools.length > 0 || !search);
	}, [clients, search]);

	const totalTools = clients.reduce((sum, c) => sum + (c.tools?.length ?? 0), 0);
	const connectedServers = clients.filter((c) => c.state === "connected").length;

	const toggleServer = (clientId: string) => {
		setExpandedServers((prev) => {
			const next = new Set(prev);
			if (next.has(clientId)) {
				next.delete(clientId);
			} else {
				next.add(clientId);
			}
			return next;
		});
	};

	const handleToggleToolExecution = async (
		clientId: string,
		toolName: string,
		field: "tools_to_execute" | "tools_to_auto_execute",
		currentSet: Set<string>,
	) => {
		const newSet = new Set(currentSet);
		if (newSet.has(toolName)) {
			newSet.delete(toolName);
		} else {
			newSet.add(toolName);
		}
		try {
			await updateMCPClient({
				id: clientId,
				data: { [field]: Array.from(newSet) },
			}).unwrap();
			toast.success(`Tool "${toolName}" ${field === "tools_to_execute" ? "execution" : "auto-execution"} updated`);
		} catch (error) {
			toast.error(getErrorMessage(error));
		}
	};

	const handleEnableAllTools = async (clientId: string, tools: { name: string }[]) => {
		try {
			await updateMCPClient({
				id: clientId,
				data: { tools_to_execute: tools.map((t) => t.name) },
			}).unwrap();
			toast.success("All tools enabled for execution");
		} catch (error) {
			toast.error(getErrorMessage(error));
		}
	};

	const handleDisableAllTools = async (clientId: string) => {
		try {
			await updateMCPClient({
				id: clientId,
				data: { tools_to_execute: [] },
			}).unwrap();
			toast.success("All tools disabled for execution");
		} catch (error) {
			toast.error(getErrorMessage(error));
		}
	};

	return (
		<>
			<div className="mb-4 flex flex-wrap items-center justify-between gap-3">
				<div>
					<h1 className="text-lg font-semibold">MCP Tool Groups</h1>
					<p className="text-muted-foreground text-sm">
						Organize and govern tools across your MCP servers. Control which tools are available for execution and auto-execution.
					</p>
				</div>
			</div>

			{/* Summary Stats */}
			<div className="mb-4 grid gap-4 md:grid-cols-3">
				<Card className="shadow-none">
					<CardContent className="flex items-center gap-3 px-4 py-3">
						<Server className="text-muted-foreground h-5 w-5" />
						<div>
							<p className="text-muted-foreground text-xs">Connected Servers</p>
							<p className="text-lg font-semibold">{connectedServers} / {clients.length}</p>
						</div>
					</CardContent>
				</Card>
				<Card className="shadow-none">
					<CardContent className="flex items-center gap-3 px-4 py-3">
						<ToolCase className="text-muted-foreground h-5 w-5" />
						<div>
							<p className="text-muted-foreground text-xs">Total Tools</p>
							<p className="text-lg font-semibold">{totalTools}</p>
						</div>
					</CardContent>
				</Card>
				<Card className="shadow-none">
					<CardContent className="flex items-center gap-3 px-4 py-3">
						<Check className="text-muted-foreground h-5 w-5" />
						<div>
							<p className="text-muted-foreground text-xs">Enabled for Execution</p>
							<p className="text-lg font-semibold">
								{clients.reduce((sum, c) => sum + (c.config.tools_to_execute?.length ?? 0), 0)}
							</p>
						</div>
					</CardContent>
				</Card>
			</div>

			{/* Search */}
			<div className="mb-4 flex items-center gap-3">
				<div className="relative max-w-sm flex-1">
					<Search className="text-muted-foreground absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2" />
					<Input
						placeholder="Search tools by name or description..."
						value={search}
						onChange={(e) => setSearch(e.target.value)}
						className="pl-9"
					/>
				</div>
			</div>

			{/* Server + Tools List */}
			{isLoading ? (
				<div className="rounded-sm border p-8 text-center">
					<p className="text-muted-foreground text-sm">Loading MCP servers and tools...</p>
				</div>
			) : serverTools.length === 0 ? (
				<div className="rounded-sm border p-8 text-center">
					<ToolCase className="text-muted-foreground mx-auto h-12 w-12 mb-3" />
					<p className="text-muted-foreground text-sm">
						{clients.length === 0
							? "No MCP servers configured. Add servers in MCP Registry to see tools here."
							: "No tools found matching your search."}
					</p>
				</div>
			) : (
				<div className="space-y-3">
					{serverTools.map((server) => {
						const isExpanded = expandedServers.has(server.clientId);
						return (
							<Card key={server.clientId} className="shadow-none">
								<Collapsible open={isExpanded} onOpenChange={() => toggleServer(server.clientId)}>
									<CollapsibleTrigger asChild>
										<CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors py-3 px-4">
											<div className="flex items-center justify-between">
												<div className="flex items-center gap-3">
													{isExpanded ? (
														<ChevronDown className="h-4 w-4 text-muted-foreground" />
													) : (
														<ChevronRight className="h-4 w-4 text-muted-foreground" />
													)}
													<Server className="h-4 w-4" />
													<CardTitle className="text-sm font-medium">{server.name}</CardTitle>
													<Badge variant={server.state === "connected" ? "default" : "destructive"} className="text-xs">
														{server.state}
													</Badge>
													<span className="text-muted-foreground text-xs">
														{server.allTools.length} tool{server.allTools.length !== 1 ? "s" : ""}
													</span>
												</div>
												<div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
													<Button
														variant="outline"
														size="sm"
														className="text-xs h-7"
														onClick={() => handleEnableAllTools(server.clientId, server.allTools)}
													>
														Enable All
													</Button>
													<Button
														variant="outline"
														size="sm"
														className="text-xs h-7"
														onClick={() => handleDisableAllTools(server.clientId)}
													>
														Disable All
													</Button>
												</div>
											</div>
										</CardHeader>
									</CollapsibleTrigger>
									<CollapsibleContent>
										<CardContent className="px-4 pb-4 pt-0">
											<Table>
												<TableHeader>
													<TableRow>
														<TableHead className="w-[250px]">Tool Name</TableHead>
														<TableHead>Description</TableHead>
														<TableHead className="w-[120px] text-center">Execute</TableHead>
														<TableHead className="w-[120px] text-center">Auto Execute</TableHead>
													</TableRow>
												</TableHeader>
												<TableBody>
													{server.tools.map((tool) => (
														<TableRow key={tool.name}>
															<TableCell className="font-mono text-xs font-medium">{tool.name}</TableCell>
															<TableCell className="text-muted-foreground text-xs max-w-md truncate">
																{tool.description || "-"}
															</TableCell>
															<TableCell className="text-center">
																<Checkbox
																	checked={server.toolsToExecute.has(tool.name)}
																	onCheckedChange={() =>
																		handleToggleToolExecution(
																			server.clientId,
																			tool.name,
																			"tools_to_execute",
																			server.toolsToExecute,
																		)
																	}
																/>
															</TableCell>
															<TableCell className="text-center">
																<Checkbox
																	checked={server.toolsToAutoExecute.has(tool.name)}
																	onCheckedChange={() =>
																		handleToggleToolExecution(
																			server.clientId,
																			tool.name,
																			"tools_to_auto_execute",
																			server.toolsToAutoExecute,
																		)
																	}
																/>
															</TableCell>
														</TableRow>
													))}
												</TableBody>
											</Table>
										</CardContent>
									</CollapsibleContent>
								</Collapsible>
							</Card>
						);
					})}
				</div>
			)}
		</>
	);
}
