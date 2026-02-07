<script lang="ts">
	import type { Workspace } from '../../types';

	interface Props {
		showArchived: boolean;
		workspaceError: string | null;
		activeWorkspaces: Workspace[];
		archivedWorkspaces: Workspace[];
		selectedWorkspaceId: string | null;
		confirmWorkspaceRemove: string | null;
		onShowArchivedChange: (value: boolean) => void;
		onSelectWorkspace: (id: string) => void;
		onOpenWorkspace: (id: string) => void;
		onArchiveWorkspace: (workspace: Workspace) => void;
		onUnarchiveWorkspace: (workspace: Workspace) => void;
		onConfirmRemoveWorkspace: (workspaceId: string | null) => void;
		onRemoveWorkspace: (workspace: Workspace) => void;
	}

	const {
		showArchived,
		workspaceError,
		activeWorkspaces,
		archivedWorkspaces,
		selectedWorkspaceId,
		confirmWorkspaceRemove,
		onShowArchivedChange,
		onSelectWorkspace,
		onOpenWorkspace,
		onArchiveWorkspace,
		onUnarchiveWorkspace,
		onConfirmRemoveWorkspace,
		onRemoveWorkspace,
	}: Props = $props();
</script>

<div class="list-header">
	<div class="section-title">Workspace list</div>
	<label class="toggle">
		<input
			type="checkbox"
			checked={showArchived}
			onchange={(event) => onShowArchivedChange((event.currentTarget as HTMLInputElement).checked)}
		/>
		<span>Show archived</span>
	</label>
</div>
{#if workspaceError}
	<div class="note error">{workspaceError}</div>
{/if}

<div class="workspace-column">
	{#if activeWorkspaces.length === 0}
		<div class="empty">No active workspaces yet.</div>
	{/if}
	{#each activeWorkspaces as workspace (workspace.id)}
		<div class:active={workspace.id === selectedWorkspaceId} class="workspace-card">
			<button class="select" type="button" onclick={() => onSelectWorkspace(workspace.id)}>
				<div class="name">{workspace.name}</div>
				<div class="path">{workspace.path}</div>
			</button>
			<div class="card-actions">
				<button class="ghost" type="button" onclick={() => onOpenWorkspace(workspace.id)}
					>Open</button
				>
				<button class="ghost" type="button" onclick={() => onArchiveWorkspace(workspace)}
					>Archive</button
				>
				{#if confirmWorkspaceRemove === workspace.id}
					<button class="danger" type="button" onclick={() => onRemoveWorkspace(workspace)}>
						Confirm remove
					</button>
					<button class="ghost" type="button" onclick={() => onConfirmRemoveWorkspace(null)}
						>Cancel</button
					>
				{:else}
					<button
						class="ghost"
						type="button"
						onclick={() => onConfirmRemoveWorkspace(workspace.id)}
					>
						Remove
					</button>
				{/if}
			</div>
		</div>
	{/each}

	{#if showArchived}
		<div class="divider">Archived</div>
		{#if archivedWorkspaces.length === 0}
			<div class="empty">No archived workspaces.</div>
		{/if}
		{#each archivedWorkspaces as workspace (workspace.id)}
			<div class:active={workspace.id === selectedWorkspaceId} class="workspace-card archived">
				<button class="select" type="button" onclick={() => onSelectWorkspace(workspace.id)}>
					<div class="name">{workspace.name}</div>
					<div class="path">{workspace.path}</div>
					{#if workspace.archivedReason}
						<div class="reason">{workspace.archivedReason}</div>
					{/if}
				</button>
				<div class="card-actions">
					<button class="ghost" type="button" onclick={() => onUnarchiveWorkspace(workspace)}>
						Unarchive
					</button>
					{#if confirmWorkspaceRemove === workspace.id}
						<button class="danger" type="button" onclick={() => onRemoveWorkspace(workspace)}>
							Confirm remove
						</button>
						<button class="ghost" type="button" onclick={() => onConfirmRemoveWorkspace(null)}
							>Cancel</button
						>
					{:else}
						<button
							class="ghost"
							type="button"
							onclick={() => onConfirmRemoveWorkspace(workspace.id)}
						>
							Remove
						</button>
					{/if}
				</div>
			</div>
		{/each}
	{/if}
</div>

<style>
	.list-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}

	.section-title {
		font-size: 13px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
		font-weight: 600;
	}

	.toggle {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		color: var(--muted);
		font-size: 12px;
	}

	.workspace-column {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.workspace-card {
		border: 1px solid var(--border);
		border-radius: 14px;
		padding: 12px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		background: var(--panel-soft);
	}

	.workspace-card.archived {
		border-style: dashed;
		opacity: 0.8;
	}

	.workspace-card.active {
		border-color: var(--accent);
		box-shadow: inset 0 0 0 1px rgba(45, 140, 255, 0.35);
	}

	.select {
		background: none;
		border: none;
		text-align: left;
		cursor: pointer;
		color: inherit;
	}

	.name {
		font-size: 15px;
		font-weight: 600;
	}

	.path,
	.reason {
		font-size: 12px;
		color: var(--muted);
	}

	.reason {
		margin-top: 6px;
	}

	.card-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.divider {
		margin-top: 12px;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
	}

	.empty {
		font-size: 13px;
		color: var(--muted);
		padding: 8px 0;
	}

	.note {
		font-size: 13px;
	}

	.note.error {
		color: var(--danger);
	}
</style>
