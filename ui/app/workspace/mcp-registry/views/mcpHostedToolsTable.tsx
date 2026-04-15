"use client";

import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "@/components/ui/alertDialog";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useToast } from "@/hooks/use-toast";
import { useDeleteMCPHostedToolMutation } from "@/lib/store/apis/mcpApi";
import type { MCPHostedTool } from "@/lib/types/mcp";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { Activity, Pencil, Play, Trash2 } from "lucide-react";
import { useState } from "react";
import { MCPHostedToolForm } from "./mcpHostedToolForm";
import { MCPHostedToolObservabilityDialog } from "./mcpHostedToolObservabilityDialog";
import { MCPHostedToolPreviewDialog } from "./mcpHostedToolPreviewDialog";

interface MCPHostedToolsTableProps {
	tools: MCPHostedTool[];
	refetch?: () => void | Promise<unknown>;
}

export function MCPHostedToolsTable({ tools, refetch }: MCPHostedToolsTableProps) {
	const { toast } = useToast();
	const hasDeleteAccess = useRbac(RbacResource.MCPGateway, RbacOperation.Delete);
	const hasUpdateAccess = useRbac(RbacResource.MCPGateway, RbacOperation.Update);
	const [deleteHostedTool] = useDeleteMCPHostedToolMutation();
	const [selectedTool, setSelectedTool] = useState<MCPHostedTool | null>(null);
	const [previewTool, setPreviewTool] = useState<MCPHostedTool | null>(null);
	const [observabilityTool, setObservabilityTool] = useState<MCPHostedTool | null>(null);

	const handleDelete = async (tool: MCPHostedTool) => {
		try {
			await deleteHostedTool(tool.tool_id).unwrap();
			toast({
				title: "Deleted",
				description: `Hosted tool ${tool.name} removed successfully.`,
			});
			await refetch?.();
		} catch (error: any) {
			toast({
				title: "Error",
				description: error?.data?.error?.message || "Failed to delete hosted tool.",
				variant: "destructive",
			});
		}
	};

	const argumentCount = (tool: MCPHostedTool) => tool.tool_schema?.function?.parameters?.required?.length || 0;
	const queryParamCount = (tool: MCPHostedTool) => Object.keys(tool.query_params || {}).length;
	const hasStructuredResponse = (tool: MCPHostedTool) => !!tool.response_template || !!tool.response_json_path;
	const hasResponseSchema = (tool: MCPHostedTool) => !!tool.response_schema && Object.keys(tool.response_schema).length > 0;
	const responseExampleCount = (tool: MCPHostedTool) => tool.response_examples?.length || 0;
	const authModeLabel = (tool: MCPHostedTool) => {
		switch (tool.auth_profile?.mode) {
			case "bearer_passthrough":
				return "Bearer";
			case "header_passthrough":
				return "Header map";
			default:
				return "None";
		}
	};

	return (
		<Card className="mt-6">
			{selectedTool ? (
				<MCPHostedToolForm
					open={!!selectedTool}
					onOpenChange={(open) => {
						if (!open) {
							setSelectedTool(null);
						}
					}}
					tool={selectedTool}
					onSaved={refetch}
				/>
			) : null}
			<MCPHostedToolPreviewDialog
				open={!!previewTool}
				onOpenChange={(open) => {
					if (!open) {
						setPreviewTool(null);
					}
				}}
				tool={previewTool}
			/>
			<MCPHostedToolObservabilityDialog
				open={!!observabilityTool}
				onOpenChange={(open) => {
					if (!open) {
						setObservabilityTool(null);
					}
				}}
				tool={observabilityTool}
			/>
			<CardHeader>
				<CardTitle>Hosted API Tools</CardTitle>
				<CardDescription>
					Ordinary enterprise APIs imported as in-process MCP tools. These definitions are stored in ConfigStore and stay consistent across cluster nodes.
				</CardDescription>
			</CardHeader>
			<CardContent>
				<div className="overflow-hidden rounded-sm border">
					<Table data-testid="mcp-hosted-tools-table">
						<TableHeader>
							<TableRow className="bg-muted/50">
								<TableHead>Name</TableHead>
								<TableHead>Method</TableHead>
								<TableHead>URL</TableHead>
								<TableHead>Arguments</TableHead>
								<TableHead>Query Params</TableHead>
								<TableHead>Auth</TableHead>
								<TableHead>Headers</TableHead>
								<TableHead>Response</TableHead>
								<TableHead>Schema</TableHead>
								<TableHead>Examples</TableHead>
								<TableHead className="w-20 text-right"></TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{tools.length === 0 ? (
								<TableRow>
									<TableCell colSpan={11} className="text-muted-foreground h-24 text-center">
										No hosted tools yet.
									</TableCell>
								</TableRow>
							) : (
								tools.map((tool) => (
									<TableRow key={tool.tool_id}>
										<TableCell className="font-medium">{tool.name}</TableCell>
										<TableCell>
											<Badge variant="outline">{tool.method}</Badge>
										</TableCell>
										<TableCell className="max-w-[420px] truncate font-mono text-xs">{tool.url}</TableCell>
										<TableCell>{argumentCount(tool)}</TableCell>
										<TableCell>{queryParamCount(tool)}</TableCell>
										<TableCell>
											{tool.auth_profile?.mode && tool.auth_profile.mode !== "none" ? (
												<Badge variant="secondary">{authModeLabel(tool)}</Badge>
											) : (
												<span className="text-muted-foreground text-xs">None</span>
											)}
										</TableCell>
										<TableCell>{Object.keys(tool.headers || {}).length}</TableCell>
										<TableCell>
											{hasStructuredResponse(tool) ? (
												<Badge variant="secondary">
													{tool.response_template ? "Template" : "JSON Path"}
												</Badge>
											) : (
												<span className="text-muted-foreground text-xs">Raw</span>
											)}
										</TableCell>
										<TableCell>
											{hasResponseSchema(tool) ? (
												<Badge variant="secondary">Schema</Badge>
											) : (
												<span className="text-muted-foreground text-xs">None</span>
											)}
										</TableCell>
										<TableCell>
											{responseExampleCount(tool) > 0 ? (
												<Badge variant="secondary">
													{responseExampleCount(tool)} example{responseExampleCount(tool) > 1 ? "s" : ""}
												</Badge>
											) : (
												<span className="text-muted-foreground text-xs">None</span>
											)}
										</TableCell>
										<TableCell className="text-right">
											<div className="flex justify-end gap-1">
												<Button
													variant="ghost"
													size="icon"
													data-testid={`observe-hosted-tool-${tool.tool_id}`}
													onClick={() => setObservabilityTool(tool)}
												>
													<Activity className="h-4 w-4" />
												</Button>
												<Button
													variant="ghost"
													size="icon"
													data-testid={`preview-hosted-tool-${tool.tool_id}`}
													onClick={() => setPreviewTool(tool)}
												>
													<Play className="h-4 w-4" />
												</Button>
												{hasUpdateAccess ? (
													<Button
														variant="ghost"
														size="icon"
														data-testid={`edit-hosted-tool-${tool.tool_id}`}
														onClick={() => setSelectedTool(tool)}
													>
														<Pencil className="h-4 w-4" />
													</Button>
												) : null}
												{hasDeleteAccess ? (
													<AlertDialog>
														<AlertDialogTrigger asChild>
															<Button
																variant="ghost"
																size="icon"
																data-testid={`delete-hosted-tool-${tool.tool_id}`}
															>
																<Trash2 className="h-4 w-4 text-red-500" />
															</Button>
														</AlertDialogTrigger>
														<AlertDialogContent>
															<AlertDialogHeader>
																<AlertDialogTitle>Delete hosted tool?</AlertDialogTitle>
																<AlertDialogDescription>
																	This removes the tool from ConfigStore and unregisters it on all cluster nodes.
																</AlertDialogDescription>
															</AlertDialogHeader>
															<AlertDialogFooter>
																<AlertDialogCancel>Cancel</AlertDialogCancel>
																<AlertDialogAction onClick={() => void handleDelete(tool)}>
																	Delete
																</AlertDialogAction>
															</AlertDialogFooter>
														</AlertDialogContent>
													</AlertDialog>
												) : null}
											</div>
										</TableCell>
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				</div>
			</CardContent>
		</Card>
	);
}
