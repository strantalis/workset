<script lang="ts">
	import { deriveSidebarLabelLimits, ellipsisMiddle } from '../names';
	import type { Workspace, Repo } from '../types';
	import WorkspaceItemRepoSection from './workspace-item/WorkspaceItemRepoSection.svelte';
	import DropdownMenu from './ui/DropdownMenu.svelte';
	import { Pin, ChevronRight, MoreHorizontal, Plus, Pencil, Archive, Trash2 } from '@lucide/svelte';

	interface Props {
		workspace: Workspace;
		isActive: boolean;
		isPinned: boolean;
		draggable?: boolean;
		onSelectWorkspace: () => void;
		onSelectRepo: (repoId: string) => void;
		onAddRepo: () => void;
		onManageWorkspace: (action: 'rename' | 'archive' | 'remove') => void;
		onManageRepo: (repoId: string, action: 'remove') => void;
		onTogglePin: () => void;
		onSetColor: (color: string) => void;
		onDragStart: (workspaceId: string) => void;
		onDragEnd: () => void;
		onDrop: (workspaceId: string) => void;
		onToggleExpanded: () => void;
	}

	const {
		workspace,
		isActive,
		isPinned,
		draggable = true,
		onSelectWorkspace,
		onSelectRepo,
		onAddRepo,
		onManageWorkspace,
		onManageRepo,
		onTogglePin,
		onSetColor,
		onDragStart,
		onDragEnd,
		onDrop,
		onToggleExpanded,
	}: Props = $props();

	let workspaceMenu: boolean = $state(false);
	let workspaceTrigger: HTMLElement | null = $state(null);
	let isDragOver: boolean = $state(false);
	let itemElement: HTMLDivElement | null = $state(null);
	let labelLimits = $state(deriveSidebarLabelLimits(280));

	const isSingleRepo = $derived(workspace.repos.length === 1);
	const isMultiRepo = $derived(workspace.repos.length > 1);
	const isExpanded = $derived(workspace.expanded);

	// Color presets
	const colorPresets = [
		{ name: 'blue', hex: '#3b82f6' },
		{ name: 'green', hex: '#22c55e' },
		{ name: 'purple', hex: '#a855f7' },
		{ name: 'orange', hex: '#f97316' },
		{ name: 'red', hex: '#ef4444' },
		{ name: 'gray', hex: '#6b7280' },
	];

	function getColorStyle(color: string | undefined): string {
		if (!color) return '';
		const preset = colorPresets.find((c) => c.name === color);
		return preset ? `border-left: 4px solid ${preset.hex};` : '';
	}

	function formatLastUsed(lastUsed: string): string {
		if (!lastUsed) return '';
		const date = new Date(lastUsed);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMs / 3600000);
		const diffDays = Math.floor(diffMs / 86400000);

		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		if (diffDays < 7) return `${diffDays}d ago`;
		return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
	}

	function handleDragStart(e: DragEvent) {
		e.dataTransfer?.setData('text/plain', workspace.id);
		onDragStart(workspace.id);
	}

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
		isDragOver = true;
	}

	function handleDragLeave() {
		isDragOver = false;
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		e.stopPropagation();
		isDragOver = false;
		onDrop(workspace.id);
	}

	function closeWorkspaceMenu() {
		workspaceMenu = false;
	}

	function formatRepoRef(repo: Repo): string {
		const branch = repo.currentBranch ?? repo.defaultBranch;
		if (!repo.remote && !branch) return '';
		if (repo.remote && branch) return `${repo.remote}/${branch}`;
		return branch ?? repo.remote ?? '';
	}

	function getRepoStatusDot(repo: Repo): { className: string; label: string; title: string } {
		if (repo.statusKnown === false) {
			return { className: 'unknown', label: 'Status pending', title: 'Status pending' };
		} else if (repo.missing) {
			return { className: 'missing', label: 'Missing', title: 'Missing' };
		} else if (repo.diff.added + repo.diff.removed > 0) {
			return {
				className: 'changes',
				label: `+${repo.diff.added}/-${repo.diff.removed}`,
				title: `+${repo.diff.added}/-${repo.diff.removed}`,
			};
		} else if (repo.dirty) {
			return { className: 'modified', label: 'Modified', title: 'Modified' };
		} else {
			return { className: 'clean', label: 'Clean', title: 'Clean' };
		}
	}

	$effect(() => {
		if (!itemElement || typeof ResizeObserver === 'undefined') return;
		const observer = new ResizeObserver((entries) => {
			const width = entries[0]?.contentRect.width ?? 0;
			labelLimits = deriveSidebarLabelLimits(width);
		});
		observer.observe(itemElement);
		return () => observer.disconnect();
	});
</script>

<div
	bind:this={itemElement}
	class="workspace-item"
	class:active={isActive}
	class:drag-over={isDragOver}
	class:single-repo={isSingleRepo}
	class:multi-repo={isMultiRepo}
	style={getColorStyle(workspace.color)}
	{draggable}
	ondragstart={handleDragStart}
	ondragend={onDragEnd}
	ondragover={handleDragOver}
	ondragleave={handleDragLeave}
	ondrop={handleDrop}
	role="listitem"
>
	<div class="workspace-header">
		{#if isMultiRepo}
			<button
				class="toggle"
				class:expanded={isExpanded}
				onclick={onToggleExpanded}
				type="button"
				aria-label={isExpanded ? 'Collapse workspace' : 'Expand workspace'}
			>
				<ChevronRight size={16} />
			</button>
		{:else}
			<div class="toggle-placeholder"></div>
		{/if}

		<button class="workspace-info" onclick={onSelectWorkspace} type="button" title={workspace.name}>
			<span class="workspace-title">
				<span class="name">{ellipsisMiddle(workspace.name, labelLimits.workspace)}</span>
				{#if isMultiRepo}
					<span class="count">{workspace.repos.length}</span>
				{/if}
			</span>
		</button>
		{#if isMultiRepo}
			<span class="last-used-header">{formatLastUsed(workspace.lastUsed)}</span>
		{/if}

		<div class="actions">
			<button
				class="pin-btn"
				class:pinned={isPinned}
				onclick={onTogglePin}
				type="button"
				aria-label={isPinned ? 'Unpin workspace' : 'Pin workspace'}
			>
				<Pin size={18} fill={isPinned ? 'currentColor' : 'none'} />
			</button>
			<button
				class="menu-trigger"
				type="button"
				onclick={(e) => {
					workspaceTrigger = e.currentTarget;
					workspaceMenu = !workspaceMenu;
				}}
				aria-label="Workspace actions"
			>
				<MoreHorizontal size={16} />
			</button>
			<DropdownMenu
				open={workspaceMenu}
				onClose={closeWorkspaceMenu}
				position="left"
				trigger={workspaceTrigger}
			>
				<div class="color-picker">
					<span class="color-label">Color</span>
					<div class="color-options">
						{#each colorPresets as preset (preset.name)}
							<button
								class="color-option"
								class:selected={workspace.color === preset.name}
								style="background-color: {preset.hex}"
								onclick={() => {
									onSetColor(preset.name);
									closeWorkspaceMenu();
								}}
								type="button"
								aria-label="Set color to {preset.name}"
							></button>
						{/each}
					</div>
				</div>
				<button
					type="button"
					onclick={() => {
						closeWorkspaceMenu();
						onAddRepo();
					}}
				>
					<Plus size={14} />
					Add repo
				</button>
				<button
					type="button"
					onclick={() => {
						closeWorkspaceMenu();
						onManageWorkspace('rename');
					}}
				>
					<Pencil size={14} />
					Rename
				</button>
				<button
					type="button"
					onclick={() => {
						closeWorkspaceMenu();
						onManageWorkspace('archive');
					}}
				>
					<Archive size={14} />
					Archive
				</button>
				<button
					class="danger"
					type="button"
					onclick={() => {
						closeWorkspaceMenu();
						onManageWorkspace('remove');
					}}
				>
					<Trash2 size={14} />
					Remove
				</button>
			</DropdownMenu>
		</div>
	</div>

	<WorkspaceItemRepoSection
		{workspace}
		{isSingleRepo}
		{isMultiRepo}
		{isExpanded}
		{labelLimits}
		{onSelectRepo}
		{onManageRepo}
		{formatLastUsed}
		{formatRepoRef}
		{getRepoStatusDot}
	/>
</div>

<style>
	.workspace-item {
		--workspace-actions-width: 60px;
		display: flex;
		flex-direction: column;
		margin-bottom: var(--space-1);
		border-radius: var(--radius-md);
		transition: all 0.15s ease;
		background: rgba(255, 255, 255, 0.02);
	}

	.workspace-item:hover {
		background: rgba(255, 255, 255, 0.04);
	}

	.workspace-item.active {
		background: var(--accent-subtle);
	}

	.workspace-item.drag-over {
		background: rgba(255, 255, 255, 0.08);
		box-shadow: inset 0 0 0 2px var(--accent);
	}

	.workspace-header {
		display: grid;
		grid-template-columns: 20px minmax(0, 1fr) auto;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-2) var(--space-3) var(--space-2) var(--space-2);
		position: relative;
	}

	.toggle {
		background: none;
		border: none;
		color: var(--muted);
		cursor: pointer;
		padding: 2px;
		display: grid;
		place-items: center;
		transition: color 0.15s ease;
	}

	.toggle:hover {
		color: var(--text);
	}

	.toggle :global(svg) {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 2;
		fill: none;
		transition: transform 0.2s cubic-bezier(0.34, 1.56, 0.64, 1);
	}

	.toggle.expanded :global(svg) {
		transform: rotate(90deg);
	}

	.toggle-placeholder {
		width: 20px;
	}

	.workspace-info {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		background: none;
		border: none;
		color: var(--text);
		cursor: pointer;
		text-align: left;
		padding: 4px;
		border-radius: var(--radius-sm);
		transition:
			background 0.15s ease,
			padding-right 0.15s ease;
		min-width: 0;
		width: 100%;
	}

	.workspace-info:hover {
		background: rgba(255, 255, 255, 0.02);
	}

	.workspace-item.single-repo:hover .workspace-info,
	.workspace-item.single-repo.active .workspace-info,
	.workspace-item.single-repo:focus-within .workspace-info {
		padding-right: calc(var(--workspace-actions-width) + var(--space-1));
	}

	.workspace-title {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex: 1;
		min-width: 0;
	}

	.name {
		font-size: 13px;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
		min-width: 0;
	}

	.count {
		color: var(--muted);
		font-size: 11px;
		font-weight: 500;
		font-variant-numeric: tabular-nums;
		flex-shrink: 0;
		transition: opacity 0.15s ease;
	}

	.last-used-header {
		font-size: 10px;
		color: var(--muted);
		opacity: 0.7;
		white-space: nowrap;
		justify-self: end;
		padding-right: var(--space-1);
		transition: opacity 0.15s ease;
	}

	.workspace-item:hover .last-used-header,
	.workspace-item.active .last-used-header,
	.workspace-item:focus-within .last-used-header {
		opacity: 0;
	}

	.workspace-item:hover .count,
	.workspace-item.active .count,
	.workspace-item:focus-within .count {
		opacity: 0;
	}

	.actions {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		justify-content: flex-end;
		width: var(--workspace-actions-width);
		opacity: 0;
		transition: opacity 0.15s ease;
		position: absolute;
		right: var(--space-3);
		top: 50%;
		transform: translateY(-50%);
		pointer-events: none;
	}

	.workspace-item:hover .actions,
	.workspace-item.active .actions,
	.workspace-item:focus-within .actions {
		opacity: 1;
		pointer-events: auto;
	}

	.pin-btn {
		background: none;
		border: none;
		color: var(--muted);
		cursor: pointer;
		padding: 2px;
		display: grid;
		place-items: center;
		transition: all 0.15s ease;
	}

	.pin-btn:hover {
		color: var(--accent);
	}

	.pin-btn.pinned {
		color: var(--accent);
	}

	.pin-btn :global(svg) {
		width: 18px;
		height: 18px;
	}

	.menu-trigger {
		width: 24px;
		height: 24px;
		border-radius: var(--radius-sm);
		border: 1px solid var(--border);
		background: rgba(255, 255, 255, 0.02);
		color: var(--text);
		cursor: pointer;
		display: grid;
		place-items: center;
		padding: 0;
		transition: all 0.15s ease;
	}

	.menu-trigger:hover {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}

	.menu-trigger :global(svg) {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 1.6;
		fill: none;
	}

	.color-picker {
		padding: var(--space-2);
		border-bottom: 1px solid var(--border);
		margin-bottom: var(--space-1);
	}

	.color-label {
		font-size: 11px;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: var(--space-1);
		display: block;
	}

	.color-options {
		display: flex;
		gap: var(--space-1);
	}

	.color-option {
		width: 20px;
		height: 20px;
		border-radius: var(--radius-sm);
		border: 2px solid transparent;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.color-option:hover {
		transform: scale(1.1);
	}

	.color-option.selected {
		border-color: var(--text);
		box-shadow:
			0 0 0 2px var(--bg),
			0 0 0 3px var(--text);
	}

	:global(.dropdown-menu button.danger) {
		color: var(--danger);
	}

	:global(.dropdown-menu button) {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: none;
		border: none;
		color: var(--text);
		cursor: pointer;
		width: 100%;
		text-align: left;
		font-size: 13px;
		transition: background 0.15s ease;
		border-radius: var(--radius-sm);
	}

	:global(.dropdown-menu button:hover) {
		background: rgba(255, 255, 255, 0.06);
	}

	:global(.dropdown-menu button svg) {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 1.8;
		fill: none;
		flex-shrink: 0;
	}
</style>
