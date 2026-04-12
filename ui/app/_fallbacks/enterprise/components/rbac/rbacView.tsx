"use client";

import { useState, useEffect, useCallback } from "react";
import { toast } from "sonner";
import { Plus, Edit, Trash2, Shield, ShieldCheck } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
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
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alertDialog";

import {
	useGetRbacRolesQuery,
	useCreateRbacRoleMutation,
	useUpdateRbacRoleMutation,
	useDeleteRbacRoleMutation,
} from "@/lib/store/apis/rbacApi";
import { getErrorMessage } from "@/lib/store";
import {
	RbacRole,
	RbacPermissionInput,
	CreateRbacRoleRequest,
	UpdateRbacRoleRequest,
	RBAC_RESOURCES,
	RBAC_OPERATIONS,
} from "@/lib/types/rbac";

// ─── Permission Grid ───────────────────────────────────────────────────────────

interface PermissionGridProps {
	permissions: RbacPermissionInput[];
	onChange: (permissions: RbacPermissionInput[]) => void;
	readOnly?: boolean;
}

function PermissionGrid({ permissions, onChange, readOnly = false }: PermissionGridProps) {
	const permSet = new Set(permissions.map((p) => `${p.resource}:${p.operation}`));

	const toggle = (resource: string, operation: string) => {
		if (readOnly) return;
		const key = `${resource}:${operation}`;
		if (permSet.has(key)) {
			onChange(permissions.filter((p) => !(p.resource === resource && p.operation === operation)));
		} else {
			onChange([...permissions, { resource, operation }]);
		}
	};

	const toggleRow = (resource: string) => {
		if (readOnly) return;
		const allChecked = RBAC_OPERATIONS.every((op) => permSet.has(`${resource}:${op}`));
		if (allChecked) {
			onChange(permissions.filter((p) => p.resource !== resource));
		} else {
			const existing = permissions.filter((p) => p.resource !== resource);
			const newPerms = RBAC_OPERATIONS.map((op) => ({ resource, operation: op }));
			onChange([...existing, ...newPerms]);
		}
	};

	return (
		<div className="overflow-x-auto rounded-md border">
			<Table>
				<TableHeader>
					<TableRow>
						<TableHead className="w-52 min-w-[12rem] font-semibold">Resource</TableHead>
						{RBAC_OPERATIONS.map((op) => (
							<TableHead key={op} className="text-center font-medium">
								{op}
							</TableHead>
						))}
					</TableRow>
				</TableHeader>
				<TableBody>
					{RBAC_RESOURCES.map((resource) => {
						const rowAllChecked = RBAC_OPERATIONS.every((op) => permSet.has(`${resource}:${op}`));
						return (
							<TableRow key={resource}>
								<TableCell className="font-medium">
									<div className="flex items-center gap-2">
										{!readOnly && (
											<Checkbox
												checked={rowAllChecked}
												onCheckedChange={() => toggleRow(resource)}
												aria-label={`Toggle all permissions for ${resource}`}
											/>
										)}
										<span className="text-sm">{resource}</span>
									</div>
								</TableCell>
								{RBAC_OPERATIONS.map((op) => (
									<TableCell key={op} className="text-center">
										<Checkbox
											checked={permSet.has(`${resource}:${op}`)}
											onCheckedChange={() => toggle(resource, op)}
											disabled={readOnly}
											aria-label={`${op} permission for ${resource}`}
										/>
									</TableCell>
								))}
							</TableRow>
						);
					})}
				</TableBody>
			</Table>
		</div>
	);
}

// ─── Role Dialog ───────────────────────────────────────────────────────────────

interface RoleDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	editingRole?: RbacRole | null;
}

function RoleDialog({ open, onOpenChange, editingRole }: RoleDialogProps) {
	const isEditing = !!editingRole;

	const [createRole, { isLoading: isCreating }] = useCreateRbacRoleMutation();
	const [updateRole, { isLoading: isUpdating }] = useUpdateRbacRoleMutation();
	const isLoading = isCreating || isUpdating;

	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [permissions, setPermissions] = useState<RbacPermissionInput[]>([]);

	const resetForm = useCallback(() => {
		if (editingRole) {
			setName(editingRole.name);
			setDescription(editingRole.description ?? "");
			setPermissions(
				(editingRole.permissions ?? []).map((p) => ({
					resource: p.resource,
					operation: p.operation,
				})),
			);
		} else {
			setName("");
			setDescription("");
			setPermissions([]);
		}
	}, [editingRole]);

	useEffect(() => {
		if (open) resetForm();
	}, [open, resetForm]);

	const handleSubmit = async () => {
		if (!name.trim()) {
			toast.error("Role name is required");
			return;
		}

		try {
			if (isEditing && editingRole) {
				const req: UpdateRbacRoleRequest = {
					name: name.trim(),
					description: description.trim(),
					permissions,
				};
				await updateRole({ id: editingRole.id, data: req }).unwrap();
				toast.success("Role updated successfully");
			} else {
				const req: CreateRbacRoleRequest = {
					name: name.trim(),
					description: description.trim(),
					permissions,
				};
				await createRole(req).unwrap();
				toast.success("Role created successfully");
			}
			onOpenChange(false);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	const isSystemRole = isEditing && editingRole?.is_system;

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-h-[90vh] max-w-5xl overflow-y-auto">
				<DialogHeader>
					<DialogTitle>{isEditing ? "Edit Role" : "Create Role"}</DialogTitle>
					<DialogDescription>
						{isSystemRole
							? "System roles cannot be renamed, but their permissions can be viewed."
							: "Define the role name, description, and assign permissions."}
					</DialogDescription>
				</DialogHeader>

				<div className="flex flex-col gap-4">
					<div className="grid grid-cols-2 gap-4">
						<div className="flex flex-col gap-1.5">
							<Label htmlFor="role-name">Role Name</Label>
							<Input
								id="role-name"
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="e.g. security-auditor"
								disabled={isSystemRole || isLoading}
							/>
						</div>
						<div className="flex flex-col gap-1.5">
							<Label htmlFor="role-description">Description</Label>
							<Input
								id="role-description"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Short description of this role"
								disabled={isSystemRole || isLoading}
							/>
						</div>
					</div>

					<div className="flex flex-col gap-1.5">
						<Label>Permissions</Label>
						<PermissionGrid
							permissions={permissions}
							onChange={setPermissions}
							readOnly={isSystemRole}
						/>
					</div>
				</div>

				{!isSystemRole && (
					<DialogFooter>
						<Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading}>
							Cancel
						</Button>
						<Button onClick={handleSubmit} disabled={isLoading}>
							{isLoading ? "Saving..." : isEditing ? "Save Changes" : "Create Role"}
						</Button>
					</DialogFooter>
				)}
			</DialogContent>
		</Dialog>
	);
}

// ─── Main RBAC View ────────────────────────────────────────────────────────────

export default function RBACView() {
	const { data, isLoading, error } = useGetRbacRolesQuery();
	const [deleteRole, { isLoading: isDeleting }] = useDeleteRbacRoleMutation();

	const roles = data?.roles ?? [];

	const [dialogOpen, setDialogOpen] = useState(false);
	const [editingRole, setEditingRole] = useState<RbacRole | null>(null);
	const [deletingRole, setDeletingRole] = useState<RbacRole | null>(null);

	const openCreate = () => {
		setEditingRole(null);
		setDialogOpen(true);
	};

	const openEdit = (role: RbacRole) => {
		setEditingRole(role);
		setDialogOpen(true);
	};

	const handleDeleteConfirm = async () => {
		if (!deletingRole) return;
		try {
			await deleteRole(deletingRole.id).unwrap();
			toast.success(`Role "${deletingRole.name}" deleted`);
		} catch (err) {
			toast.error(getErrorMessage(err));
		} finally {
			setDeletingRole(null);
		}
	};

	if (error) {
		return (
			<div className="flex h-64 items-center justify-center">
				<p className="text-destructive">Failed to load RBAC roles: {getErrorMessage(error)}</p>
			</div>
		);
	}

	return (
		<div className="flex flex-col gap-6">
			{/* Header */}
			<div className="flex items-center justify-between">
				<div className="flex flex-col gap-1">
					<h1 className="text-2xl font-semibold">Roles &amp; Permissions</h1>
					<p className="text-sm text-muted-foreground">
						Manage roles and their associated resource permissions.
					</p>
				</div>
				<Button onClick={openCreate} className="flex items-center gap-2">
					<Plus className="h-4 w-4" />
					Create Role
				</Button>
			</div>

			{/* Roles table */}
			<div className="rounded-md border">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Description</TableHead>
							<TableHead>Type</TableHead>
							<TableHead className="text-center"># Permissions</TableHead>
							<TableHead className="w-28 text-right">Actions</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading ? (
							<TableRow>
								<TableCell colSpan={5} className="text-center text-muted-foreground">
									Loading roles...
								</TableCell>
							</TableRow>
						) : roles.length === 0 ? (
							<TableRow>
								<TableCell colSpan={5} className="text-center text-muted-foreground">
									No roles found. Create one to get started.
								</TableCell>
							</TableRow>
						) : (
							roles.map((role) => (
								<TableRow key={role.id}>
									<TableCell className="font-medium">
										<div className="flex items-center gap-2">
											{role.is_system ? (
												<ShieldCheck className="h-4 w-4 text-primary" />
											) : (
												<Shield className="h-4 w-4 text-muted-foreground" />
											)}
											{role.name}
										</div>
									</TableCell>
									<TableCell className="max-w-xs truncate text-sm text-muted-foreground">
										{role.description || "—"}
									</TableCell>
									<TableCell>
										{role.is_system ? (
											<Badge variant="secondary">System</Badge>
										) : (
											<Badge variant="outline">Custom</Badge>
										)}
									</TableCell>
									<TableCell className="text-center">
										{(role.permissions ?? []).length}
									</TableCell>
									<TableCell className="text-right">
										<div className="flex items-center justify-end gap-1">
											<Button
												variant="ghost"
												size="sm"
												onClick={() => openEdit(role)}
												title={role.is_system ? "View role" : "Edit role"}
											>
												<Edit className="h-4 w-4" />
											</Button>
											{!role.is_system && (
												<Button
													variant="ghost"
													size="sm"
													onClick={() => setDeletingRole(role)}
													disabled={isDeleting}
													title="Delete role"
													className="text-destructive hover:text-destructive"
												>
													<Trash2 className="h-4 w-4" />
												</Button>
											)}
										</div>
									</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</Table>
			</div>

			{/* Create/Edit dialog */}
			<RoleDialog
				open={dialogOpen}
				onOpenChange={setDialogOpen}
				editingRole={editingRole}
			/>

			{/* Delete confirmation */}
			<AlertDialog open={!!deletingRole} onOpenChange={(open) => !open && setDeletingRole(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete Role</AlertDialogTitle>
						<AlertDialogDescription>
							Are you sure you want to delete the role &quot;{deletingRole?.name}&quot;? This action
							cannot be undone and will remove all permission assignments for this role.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={handleDeleteConfirm}
							className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						>
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}
