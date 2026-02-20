<script lang="ts">
	import WorkspaceItem from './WorkspaceItem.svelte';
	import { Pin } from '@lucide/svelte';
	import {
		workspaces,
		pinnedWorkspaces,
		unpinnedWorkspaces,
		activeWorkspaceId,
		selectWorkspace,
		selectRepo,
		toggleWorkspacePin,
		setWorkspaceColor,
		setWorkspaceExpanded,
		reorderWorkspaces,
	} from '../state';

	interface Props {
		onCreateWorkspace: () => void;
		onAddRepo: (workspaceId: string) => void;
		onManageWorkspace: (workspaceId: string, action: 'rename' | 'archive' | 'remove') => void;
		onManageRepo: (workspaceId: string, repoId: string, action: 'remove') => void;
		sidebarCollapsed?: boolean;
		onToggleSidebar: () => void;
	}

	const {
		onCreateWorkspace,
		onAddRepo,
		onManageWorkspace,
		onManageRepo,
		sidebarCollapsed = false,
		onToggleSidebar,
	}: Props = $props();

	let searchQuery = $state('');
	let draggedWorkspaceId: string | null = $state(null);
	let dragOverSection: 'pinned' | 'unpinned' | null = $state(null);

	function resetDragState() {
		draggedWorkspaceId = null;
		dragOverSection = null;
	}

	const filteredPinnedWorkspaces = $derived.by(() => {
		const query = searchQuery.toLowerCase().trim();
		if (!query) return $pinnedWorkspaces;
		return $pinnedWorkspaces.filter(
			(w) =>
				w.name.toLowerCase().includes(query) ||
				w.repos.some((r) => r.name.toLowerCase().includes(query)),
		);
	});

	const filteredUnpinnedWorkspaces = $derived.by(() => {
		const query = searchQuery.toLowerCase().trim();
		if (!query) return $unpinnedWorkspaces;
		return $unpinnedWorkspaces.filter(
			(w) =>
				w.name.toLowerCase().includes(query) ||
				w.repos.some((r) => r.name.toLowerCase().includes(query)),
		);
	});

	function handleDragStart(workspaceId: string) {
		draggedWorkspaceId = workspaceId;
	}

	function handleDragEnd() {
		resetDragState();
	}

	async function handleDrop(targetWorkspaceId: string, targetSection: 'pinned' | 'unpinned') {
		if (!draggedWorkspaceId || draggedWorkspaceId === targetWorkspaceId) {
			resetDragState();
			return;
		}

		const draggedWorkspace = $workspaces.find((w) => w.id === draggedWorkspaceId);
		const targetWorkspace = $workspaces.find((w) => w.id === targetWorkspaceId);

		if (!draggedWorkspace || !targetWorkspace) {
			resetDragState();
			return;
		}

		// Determine what section the dragged workspace is coming from
		const sourceSection = draggedWorkspace.pinned ? 'pinned' : 'unpinned';

		if (targetSection === 'unpinned') {
			if (sourceSection === 'pinned') {
				await toggleWorkspacePin(draggedWorkspaceId, false);
			}
			resetDragState();
			return;
		}

		if (sourceSection !== 'pinned') {
			await toggleWorkspacePin(draggedWorkspaceId, true);
		}

		let sectionWorkspaces = $pinnedWorkspaces;
		let draggedIndex = sectionWorkspaces.findIndex((w) => w.id === draggedWorkspaceId);
		const targetIndex = sectionWorkspaces.findIndex((w) => w.id === targetWorkspaceId);

		if (draggedIndex === -1) {
			sectionWorkspaces = [...sectionWorkspaces, draggedWorkspace];
			draggedIndex = sectionWorkspaces.length - 1;
		}

		if (draggedIndex !== -1 && targetIndex !== -1 && draggedIndex !== targetIndex) {
			const orders: Record<string, number> = {};
			const reordered = [...sectionWorkspaces];
			const [removed] = reordered.splice(draggedIndex, 1);
			reordered.splice(targetIndex, 0, removed);

			reordered.forEach((w, index) => {
				orders[w.id] = index;
			});

			await reorderWorkspaces(orders);
		}

		resetDragState();
	}

	function handleSectionDragOver(section: 'pinned' | 'unpinned', e: DragEvent) {
		e.preventDefault();
		dragOverSection = section;
	}

	function handleSectionDragLeave() {
		dragOverSection = null;
	}

	async function handleSectionDrop(section: 'pinned' | 'unpinned', e: DragEvent) {
		e.preventDefault();
		if (!draggedWorkspaceId) {
			resetDragState();
			return;
		}

		const draggedWorkspace = $workspaces.find((w) => w.id === draggedWorkspaceId);
		if (!draggedWorkspace) {
			resetDragState();
			return;
		}

		// Handle pin/unpin based on drop section
		if (section === 'pinned' && !draggedWorkspace.pinned) {
			await toggleWorkspacePin(draggedWorkspaceId, true);
		} else if (section === 'unpinned' && draggedWorkspace.pinned) {
			await toggleWorkspacePin(draggedWorkspaceId, false);
		}

		resetDragState();
	}

	function handleTogglePin(workspaceId: string) {
		const workspace = $workspaces.find((w) => w.id === workspaceId);
		if (workspace) {
			void toggleWorkspacePin(workspaceId, !workspace.pinned);
		}
	}

	function handleSetColor(workspaceId: string, color: string) {
		void setWorkspaceColor(workspaceId, color);
	}

	function handleToggleExpanded(workspaceId: string) {
		const workspace = $workspaces.find((w) => w.id === workspaceId);
		if (workspace) {
			void setWorkspaceExpanded(workspaceId, !workspace.expanded);
		}
	}

	function handleSelectRepo(workspaceId: string, repoId: string) {
		selectWorkspace(workspaceId);
		selectRepo(repoId);
	}
</script>

<div class:collapsed={sidebarCollapsed} class="tree">
	<div class="tree-header">
		<button
			class="header-btn collapse-btn"
			type="button"
			aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
			onclick={onToggleSidebar}
		>
			{#if sidebarCollapsed}
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M9 18l6-6-6-6" />
				</svg>
			{:else}
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M15 18l-6-6 6-6" />
				</svg>
			{/if}
		</button>
		<span class="title" class:collapsed={sidebarCollapsed}>Workspaces</span>
		<div class="header-spacer"></div>
	</div>

	{#if !sidebarCollapsed}
		<div class="search-bar">
			<svg class="search-icon" viewBox="0 0 24 24" aria-hidden="true">
				<circle cx="11" cy="11" r="8" />
				<path d="m21 21-4.35-4.35" />
			</svg>
			<input
				type="text"
				placeholder="Filter workspaces..."
				bind:value={searchQuery}
				class="search-input"
			/>
			{#if searchQuery}
				<button
					class="clear-btn"
					onclick={() => (searchQuery = '')}
					type="button"
					aria-label="Clear search"
				>
					<svg viewBox="0 0 24 24" aria-hidden="true">
						<path d="M18 6 6 18M6 6l12 12" />
					</svg>
				</button>
			{/if}
		</div>

		<div class="workspace-list">
			<div
				class="section"
				class:drag-over={dragOverSection === 'pinned'}
				role="group"
				ondragover={(e) => handleSectionDragOver('pinned', e)}
				ondragleave={handleSectionDragLeave}
				ondrop={(e) => handleSectionDrop('pinned', e)}
			>
				<div class="section-header ws-section-title pinned"><Pin size={12} /> Pinned</div>
				{#if filteredPinnedWorkspaces.length === 0}
					<div class="section-empty">
						{searchQuery ? 'No pinned matches' : 'Drop here to pin'}
					</div>
				{:else}
					{#each filteredPinnedWorkspaces as workspace (workspace.id)}
						<WorkspaceItem
							{workspace}
							isActive={$activeWorkspaceId === workspace.id}
							isPinned={true}
							onSelectWorkspace={() => selectWorkspace(workspace.id)}
							onSelectRepo={(repoId) => handleSelectRepo(workspace.id, repoId)}
							onAddRepo={() => onAddRepo(workspace.id)}
							onManageWorkspace={(action) => onManageWorkspace(workspace.id, action)}
							onManageRepo={(repoId, action) => onManageRepo(workspace.id, repoId, action)}
							onTogglePin={() => handleTogglePin(workspace.id)}
							onSetColor={(color) => handleSetColor(workspace.id, color)}
							onDragStart={handleDragStart}
							onDragEnd={handleDragEnd}
							onDrop={(targetId) => handleDrop(targetId, 'pinned')}
							onToggleExpanded={() => handleToggleExpanded(workspace.id)}
						/>
					{/each}
				{/if}
			</div>

			<div
				class="section"
				class:drag-over={dragOverSection === 'unpinned'}
				role="group"
				ondragover={(e) => handleSectionDragOver('unpinned', e)}
				ondragleave={handleSectionDragLeave}
				ondrop={(e) => handleSectionDrop('unpinned', e)}
			>
				<div class="section-header ws-section-title">Recent</div>
				{#each filteredUnpinnedWorkspaces as workspace (workspace.id)}
					<WorkspaceItem
						{workspace}
						isActive={$activeWorkspaceId === workspace.id}
						isPinned={false}
						onSelectWorkspace={() => selectWorkspace(workspace.id)}
						onSelectRepo={(repoId) => handleSelectRepo(workspace.id, repoId)}
						onAddRepo={() => onAddRepo(workspace.id)}
						onManageWorkspace={(action) => onManageWorkspace(workspace.id, action)}
						onManageRepo={(repoId, action) => onManageRepo(workspace.id, repoId, action)}
						onTogglePin={() => handleTogglePin(workspace.id)}
						onSetColor={(color) => handleSetColor(workspace.id, color)}
						onDragStart={handleDragStart}
						onDragEnd={handleDragEnd}
						onDrop={(targetId) => handleDrop(targetId, 'unpinned')}
						onToggleExpanded={() => handleToggleExpanded(workspace.id)}
					/>
				{/each}
			</div>
		</div>

		<button class="new-workspace-btn" type="button" onclick={onCreateWorkspace}>
			<svg viewBox="0 0 24 24" aria-hidden="true">
				<path d="M12 5v14m-7-7h14" />
			</svg>
			<span>New Workspace</span>
		</button>
	{/if}
</div>

<style>
	.tree {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
	}

	.search-bar {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: rgba(255, 255, 255, 0.03);
		border: 1px solid transparent;
		border-radius: var(--radius-lg);
		margin: 0 var(--space-2);
		transition: all 0.2s ease;
		flex-shrink: 0;
	}

	.search-bar:hover {
		background: rgba(255, 255, 255, 0.05);
	}

	.search-bar:focus-within {
		background: rgba(255, 255, 255, 0.04);
		border-color: var(--accent);
		box-shadow: 0 0 0 3px var(--accent-subtle);
	}

	.search-icon {
		width: 14px;
		height: 14px;
		stroke: var(--muted);
		stroke-width: 1.5;
		fill: none;
		flex-shrink: 0;
	}

	.search-input {
		flex: 1;
		background: none;
		border: none;
		color: var(--text);
		font-size: var(--text-base);
		outline: none;
		min-width: 0;
	}

	.search-input::placeholder {
		color: var(--muted);
	}

	.clear-btn {
		background: none;
		border: none;
		padding: 2px;
		cursor: pointer;
		color: var(--muted);
		display: grid;
		place-items: center;
		transition: color 0.15s ease;
	}

	.clear-btn:hover {
		color: var(--text);
	}

	.clear-btn svg {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 2;
		fill: none;
	}

	.tree-header {
		display: grid;
		grid-template-columns: 28px 1fr 28px;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--text);
		padding: 0 var(--space-3) var(--space-3);
		flex-shrink: 0;
	}

	.tree-header .title.collapsed {
		opacity: 0;
	}

	.header-btn {
		width: 24px;
		height: 24px;
		border-radius: var(--radius-sm);
		border: none;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		display: grid;
		place-items: center;
		padding: 0;
		transition: all 0.15s ease;
		opacity: 0.6;
	}

	.header-btn:hover {
		background: rgba(255, 255, 255, 0.06);
		color: var(--text);
		opacity: 1;
	}

	.header-btn:active {
		transform: scale(0.92);
	}

	.header-btn svg {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 1.8;
		fill: none;
	}

	.header-spacer {
		width: 24px;
	}

	.new-workspace-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-3) var(--space-2);
		margin: var(--space-2);
		background: transparent;
		border: 1px dashed rgba(255, 255, 255, 0.15);
		border-radius: var(--radius-md);
		color: var(--muted);
		font-size: var(--text-base);
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
		flex-shrink: 0;
	}

	.new-workspace-btn:hover {
		background: rgba(255, 255, 255, 0.04);
		border-color: rgba(255, 255, 255, 0.25);
		color: var(--text);
	}

	.new-workspace-btn:active {
		transform: scale(0.98);
	}

	.new-workspace-btn svg {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 1.8;
		fill: none;
	}

	.workspace-list {
		display: flex;
		flex-direction: column;
		overflow-y: auto;
		overflow-x: hidden;
		padding: 0 var(--space-1) var(--space-2);
		flex: 1;
		min-height: 0;
		gap: var(--space-2);
	}

	.section {
		display: flex;
		flex-direction: column;
		transition: all 0.2s ease;
		border-radius: var(--radius-md);
		padding: var(--space-1);
	}

	.section.drag-over {
		background: rgba(255, 255, 255, 0.04);
		box-shadow: inset 0 0 0 2px var(--accent);
	}

	.section-header {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1) var(--space-2);
		opacity: 0.7;
	}

	.section-header.pinned :global(svg) {
		color: var(--success);
	}

	.section-empty {
		font-size: var(--text-sm);
		color: var(--muted);
		padding: var(--space-2);
		border: 1px dashed rgba(255, 255, 255, 0.12);
		border-radius: var(--radius-md);
		text-align: center;
		opacity: 0.7;
	}

	/* Collapsed state */
	.tree.collapsed .workspace-list {
		padding: 0 var(--space-1);
	}

	.tree.collapsed .section,
	.tree.collapsed .section-header {
		display: none;
	}

	.tree.collapsed .search-bar {
		display: none;
	}

	.tree.collapsed .new-workspace-btn span {
		display: none;
	}

	.tree.collapsed .new-workspace-btn {
		justify-content: center;
		padding: var(--space-2);
	}

	.tree.collapsed .tree-header {
		grid-template-columns: 1fr;
		justify-items: center;
		padding: 0 var(--space-2);
	}

	.tree.collapsed .tree-header .title,
	.tree.collapsed .tree-header > :global(*:last-child) {
		display: none;
	}
</style>
