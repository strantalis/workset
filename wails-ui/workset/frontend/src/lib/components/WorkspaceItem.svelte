<script lang="ts">
	import { deriveSidebarLabelLimits, ellipsisMiddle } from '../names';
	import type { Workspace, Repo } from '../types';
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
	let repoMenu: string | null = $state(null);
	let workspaceTrigger: HTMLElement | null = $state(null);
	let repoTrigger: HTMLElement | null = $state(null);
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

	function closeMenus() {
		workspaceMenu = false;
		repoMenu = null;
	}

	function formatRepoRef(repo: Repo): string {
		if (!repo.remote && !repo.defaultBranch) return '';
		if (repo.remote && repo.defaultBranch) return `${repo.remote}/${repo.defaultBranch}`;
		return repo.defaultBranch ?? repo.remote ?? '';
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
				onClose={closeMenus}
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
									closeMenus();
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
						closeMenus();
						onAddRepo();
					}}
				>
					<Plus size={14} />
					Add repo
				</button>
				<button
					type="button"
					onclick={() => {
						closeMenus();
						onManageWorkspace('rename');
					}}
				>
					<Pencil size={14} />
					Rename
				</button>
				<button
					type="button"
					onclick={() => {
						closeMenus();
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
						closeMenus();
						onManageWorkspace('remove');
					}}
				>
					<Trash2 size={14} />
					Remove
				</button>
			</DropdownMenu>
		</div>
	</div>

	{#if isSingleRepo && workspace.repos[0]}
		{@const repo = workspace.repos[0]}
		{@const status = getRepoStatusDot(repo)}
		{@const repoRef = formatRepoRef(repo)}
		<div class="repo-item">
			<button
				class="repo-info-single"
				onclick={() => onSelectRepo(repo.id)}
				type="button"
				title={repo.name}
			>
				<span class="repo-name">{ellipsisMiddle(repo.name, labelLimits.repo)}</span>
				{#if repoRef}
					<span class="branch" title={repoRef}>
						{ellipsisMiddle(repoRef, labelLimits.ref)}
					</span>
				{/if}
				<svg
					class="status-dot {status.className}"
					viewBox="0 0 6 6"
					role="img"
					aria-label={status.label}
				>
					<title>{status.title}</title>
					<circle cx="3" cy="3" r="3" />
				</svg>
				<span class="last-used-inline">{formatLastUsed(workspace.lastUsed)}</span>
			</button>
			<div class="repo-actions">
				<button
					class="menu-trigger-small"
					type="button"
					aria-label="Repo actions"
					onclick={(event) => {
						repoTrigger = event.currentTarget;
						repoMenu = repo.id;
					}}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true">
						<circle cx="5" cy="12" r="1.5" fill="currentColor" />
						<circle cx="12" cy="12" r="1.5" fill="currentColor" />
						<circle cx="19" cy="12" r="1.5" fill="currentColor" />
					</svg>
				</button>
				<DropdownMenu
					open={repoMenu === repo.id}
					onClose={closeMenus}
					position="left"
					trigger={repoTrigger}
				>
					<button
						class="danger"
						type="button"
						onclick={() => {
							closeMenus();
							onManageRepo(repo.name, 'remove');
						}}
					>
						<svg viewBox="0 0 24 24" aria-hidden="true">
							<path
								d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
							/>
						</svg>
						Remove
					</button>
				</DropdownMenu>
			</div>
		</div>
	{:else if isMultiRepo && isExpanded}
		<div class="repo-list">
			{#each workspace.repos as repo (repo.id)}
				{@const status = getRepoStatusDot(repo)}
				{@const repoRef = formatRepoRef(repo)}
				<div class="repo-item">
					<button
						class="repo-button"
						onclick={() => onSelectRepo(repo.id)}
						type="button"
						title={repo.name}
					>
						<span class="repo-name">{ellipsisMiddle(repo.name, labelLimits.repo)}</span>
						<span class="repo-meta">
							{#if repoRef}
								<span class="branch" title={repoRef}>
									{ellipsisMiddle(repoRef, labelLimits.ref)}
								</span>
							{/if}
							<svg
								class="status-dot {status.className}"
								viewBox="0 0 6 6"
								role="img"
								aria-label={status.label}
							>
								<title>{status.title}</title>
								<circle cx="3" cy="3" r="3" />
							</svg>
						</span>
					</button>
					<div class="repo-actions">
						<button
							class="menu-trigger-small"
							type="button"
							aria-label="Repo actions"
							onclick={(event) => {
								repoTrigger = event.currentTarget;
								repoMenu = repo.id;
							}}
						>
							<svg viewBox="0 0 24 24" aria-hidden="true">
								<circle cx="5" cy="12" r="1.5" fill="currentColor" />
								<circle cx="12" cy="12" r="1.5" fill="currentColor" />
								<circle cx="19" cy="12" r="1.5" fill="currentColor" />
							</svg>
						</button>
						<DropdownMenu
							open={repoMenu === repo.id}
							onClose={closeMenus}
							position="left"
							trigger={repoTrigger}
						>
							<button
								class="danger"
								type="button"
								onclick={() => {
									closeMenus();
									onManageRepo(repo.name, 'remove');
								}}
							>
								<svg viewBox="0 0 24 24" aria-hidden="true">
									<path
										d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
									/>
								</svg>
								Remove
							</button>
						</DropdownMenu>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.workspace-item {
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
		padding: var(--space-2) var(--space-2);
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
		transition: background 0.15s ease;
		min-width: 0;
		width: 100%;
	}

	.workspace-info:hover {
		background: rgba(255, 255, 255, 0.02);
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

	.last-used-inline {
		margin-left: auto;
		font-size: 10px;
		color: var(--muted);
		opacity: 0.7;
		white-space: nowrap;
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
	.workspace-item:focus-within .last-used-header,
	.workspace-item:hover .last-used-inline,
	.workspace-item.active .last-used-inline,
	.workspace-item:focus-within .last-used-inline {
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
		opacity: 0;
		transition: opacity 0.15s ease;
		position: absolute;
		right: var(--space-2);
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

	.menu-trigger,
	.menu-trigger-small {
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

	.menu-trigger:hover,
	.menu-trigger-small:hover {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}

	.menu-trigger :global(svg),
	.menu-trigger-small :global(svg) {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 1.6;
		fill: none;
	}

	.repo-info {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: 0 var(--space-2) var(--space-1) 44px;
		font-size: 12px;
	}

	.repo-info-single {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: 0 var(--space-2) var(--space-1) 44px;
		font-size: 12px;
		background: none;
		border: none;
		color: inherit;
		cursor: pointer;
		width: 100%;
		text-align: left;
		transition: background 0.15s ease;
		flex-wrap: nowrap;
		min-width: 0;
	}

	.repo-info-single:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.repo-name {
		color: var(--text);
		font-weight: 500;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.branch {
		font-family: var(--font-mono);
		font-size: 10px;
		color: var(--muted);
		opacity: 0.7;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.repo-info-single .repo-name {
		flex: 1;
	}

	.repo-info-single .branch {
		max-width: 40%;
	}

	.repo-list {
		display: flex;
		flex-direction: column;
		padding-left: 44px;
		padding-bottom: var(--space-2);
		gap: 2px;
	}

	.repo-item {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: var(--space-1);
		border-radius: var(--radius-sm);
		transition: background 0.15s ease;
	}

	.repo-item:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.repo-button {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
		background: none;
		border: none;
		color: var(--text);
		padding: 5px var(--space-2);
		border-radius: var(--radius-sm);
		cursor: pointer;
		text-align: left;
		transition: all 0.15s ease;
		min-width: 0;
		width: 100%;
	}

	.repo-button:hover {
		background: rgba(255, 255, 255, 0.04);
	}

	.repo-button .repo-name {
		font-size: 12px;
		flex: 1;
	}

	.repo-meta {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		flex-shrink: 0;
		min-width: 0;
		max-width: 45%;
	}

	.repo-actions {
		opacity: 0;
		transition: opacity 0.15s ease;
	}

	.repo-item:hover .repo-actions {
		opacity: 1;
	}

	.status-dot {
		width: 4px;
		height: 4px;
		flex-shrink: 0;
		opacity: 0.8;
	}

	.status-dot.missing {
		fill: var(--danger);
	}

	.status-dot.unknown {
		fill: var(--muted);
	}

	.status-dot.modified {
		fill: var(--warning);
	}

	.status-dot.changes {
		fill: var(--accent);
	}

	.status-dot.clean {
		fill: var(--success);
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
