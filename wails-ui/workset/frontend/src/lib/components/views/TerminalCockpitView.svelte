<script lang="ts">
	import {
		ChevronDown,
		ChevronRight,
		FileTerminal,
		Folder,
		FolderOpen,
		Plus,
		Settings2,
		Terminal,
	} from '@lucide/svelte';
	import type { Workspace } from '../../types';
	import TerminalWorkspace from '../TerminalWorkspace.svelte';
	import ResizablePanel from '../ui/ResizablePanel.svelte';

	interface Props {
		workspace: Workspace | null;
		onOpenWorkspaceTerminal?: (workspaceId: string) => void;
		onAddRepo?: (workspaceId: string) => void;
	}

	const { workspace, onOpenWorkspaceTerminal = () => {}, onAddRepo = () => {} }: Props = $props();

	// ── Sidebar state ──────────────────────────────────────────
	let fileTreeExpanded = $state(true);

	// ── Derived data ───────────────────────────────────────────
	const repos = $derived(workspace?.repos ?? []);
</script>

<div class="cockpit">
	{#if workspace}
		<!-- ── CLI indicator + session status ─────────────────── -->
		<div class="cli-bar">
			<div class="cli-indicator">
				<Terminal size={16} />
				<span>workset-cli</span>
			</div>
			<div class="session-indicator">
				<span class="session-dot"></span>
				<span>Active Session</span>
			</div>
		</div>

		<!-- ── Main area: sidebar + terminal ──────────────────── -->
		<div class="cockpit-body">
			<ResizablePanel
				direction="horizontal"
				initialRatio={0.2}
				minRatio={0.12}
				maxRatio={0.4}
				storageKey="workset:terminal-cockpit:sidebarRatio"
			>
				<!-- Sidebar (first panel) -->
				<aside class="sidebar">
					<!-- CURRENT WORKSET section -->
					<div class="section">
						<div class="section-header">CURRENT WORKSET</div>
						<button
							type="button"
							class="workset-item active"
							onclick={() => onOpenWorkspaceTerminal(workspace.id)}
						>
							<span class="workset-icon"><FileTerminal size={13} /></span>
							<span class="workset-label">{workspace.name}</span>
						</button>
					</div>

					<!-- FILE SYSTEM section -->
					<div class="section file-section">
						<div class="section-header">
							<span>FILE SYSTEM</span>
							<button
								type="button"
								class="section-action"
								title="Add repository"
								onclick={() => onAddRepo(workspace.id)}
							>
								<Plus size={20} />
							</button>
						</div>
						<div class="file-tree">
							{#if repos.length === 0}
								<div class="tree-empty">No repositories</div>
							{:else}
								<!-- Workspace root node -->
								<button
									type="button"
									class="tree-root"
									onclick={() => (fileTreeExpanded = !fileTreeExpanded)}
								>
									{#if fileTreeExpanded}
										<ChevronDown size={12} />
										<FolderOpen size={13} />
									{:else}
										<ChevronRight size={12} />
										<Folder size={13} />
									{/if}
									<span class="tree-root-name">{workspace.name}</span>
								</button>
								{#if fileTreeExpanded}
									<div class="tree-children">
										{#each repos as repo (repo.id)}
											<div class="tree-repo" title={repo.path || repo.name}>
												<Folder size={12} />
												<span class="tree-repo-name">{repo.name}</span>
												{#if repo.missing || repo.dirty}
													<span class="repo-status-dot warning"></span>
												{/if}
											</div>
										{/each}
									</div>
								{/if}
							{/if}
						</div>
					</div>

					<!-- ENV + Config section -->
					<div class="section env-section">
						<div class="env-row">
							<span class="env-label">ENV</span>
							<button type="button" class="env-selector" disabled title="Coming soon">
								<span class="env-dot"></span>
								<span>Development (coming soon)</span>
								<ChevronDown size={10} />
							</button>
						</div>
						<div class="sync-row">
							<span>Auto-Sync</span>
							<label class="toggle" title="Coming soon">
								<input type="checkbox" checked disabled />
								<span class="toggle-track"></span>
							</label>
						</div>
						<button type="button" class="config-btn" disabled title="Coming soon">
							<Settings2 size={12} />
							<span>Advanced Config (coming soon)</span>
						</button>
					</div>
				</aside>

				{#snippet second()}
					<!-- Terminal area (second panel) -->
					<div class="terminal-area">
						<TerminalWorkspace workspaceId={workspace.id} workspaceName={workspace.name} />
					</div>
				{/snippet}
			</ResizablePanel>
		</div>
	{:else}
		<div class="empty-state">
			<FileTerminal size={28} />
			<h2>No workspace selected</h2>
			<p>Select a workspace to launch the Engineering Cockpit with live terminals.</p>
		</div>
	{/if}
</div>

<style>
	/* ── Layout ──────────────────────────────────────────── */
	.cockpit {
		display: flex;
		flex-direction: column;
		height: 100%;
		background: var(--bg);
	}

	.cockpit-body {
		flex: 1;
		min-height: 0;
		display: flex;
	}

	/* ── CLI bar (indicator + status) ─────────────────────── */
	.cli-bar {
		display: flex;
		align-items: center;
		padding: 0 16px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
		background: color-mix(in srgb, var(--panel) 60%, transparent);
		flex-shrink: 0;
		min-height: 48px;
		gap: 12px;
		backdrop-filter: blur(12px);
		-webkit-backdrop-filter: blur(12px);
	}

	.cli-indicator {
		display: flex;
		align-items: center;
		gap: 6px;
		font-family: var(--font-mono);
		font-size: var(--text-mono-md);
		color: var(--muted);
		flex-shrink: 0;
	}

	/* ── Sidebar ─────────────────────────────────────────── */
	.sidebar {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		background: color-mix(in srgb, var(--panel) 85%, transparent);
		backdrop-filter: blur(12px);
		-webkit-backdrop-filter: blur(12px);
	}

	.section {
		flex-shrink: 0;
	}

	.file-section {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 12px 4px;
		font-size: var(--text-xs);
		font-weight: 600;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--muted);
	}

	.section-action {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 4px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition:
			color var(--transition-fast),
			background var(--transition-fast);
	}

	.section-action:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 80%, transparent);
	}

	/* ── Current workset item ────────────────────────────── */
	.workset-item {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		margin: 4px 8px;
		padding: 7px 10px;
		border: none;
		border-radius: var(--radius-sm);
		background: transparent;
		color: var(--text);
		font-size: var(--text-sm);
		font-weight: 500;
		cursor: pointer;
		transition: background var(--transition-fast);
		text-align: left;
		/* constrain width to sidebar minus margins */
		width: calc(100% - 16px);
	}

	.workset-item.active {
		background: color-mix(in srgb, var(--accent) 16%, var(--panel-strong));
	}

	.workset-item:hover {
		background: color-mix(in srgb, var(--accent) 22%, var(--panel-strong));
	}

	.workset-icon {
		display: flex;
		align-items: center;
		color: var(--accent);
	}

	.workset-label {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ── File tree ───────────────────────────────────────── */
	.file-tree {
		flex: 1;
		overflow-y: auto;
		padding: 2px 0;
		scrollbar-width: thin;
		scrollbar-color: color-mix(in srgb, var(--border) 60%, transparent) transparent;
	}

	.tree-empty {
		padding: 12px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.tree-root {
		display: flex;
		align-items: center;
		gap: 4px;
		width: 100%;
		padding: 5px 10px;
		border: none;
		background: transparent;
		color: var(--text);
		font-size: var(--text-sm);
		font-weight: 500;
		cursor: pointer;
		text-align: left;
		transition: background var(--transition-fast);
	}

	.tree-root:hover {
		background: color-mix(in srgb, var(--panel-strong) 60%, transparent);
	}

	.tree-root-name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.tree-children {
		padding-left: 16px;
		margin-left: 8px;
		border-left: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
	}

	.tree-repo {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px 4px 16px;
		font-size: var(--text-sm);
		color: var(--muted);
		transition: color var(--transition-fast);
	}

	.tree-repo:hover {
		color: var(--text);
	}

	.tree-repo-name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.repo-status-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.repo-status-dot.warning {
		background: var(--warning);
	}

	/* ── ENV section ─────────────────────────────────────── */
	.env-section {
		padding: 10px 12px;
		border-top: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
		display: flex;
		flex-direction: column;
		gap: 10px;
		flex-shrink: 0;
	}

	.env-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.env-label {
		font-size: var(--text-xs);
		font-weight: 600;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--muted);
	}

	.env-selector {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px 8px;
		border: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
		border-radius: 999px;
		background: transparent;
		color: var(--muted);
		font-size: var(--text-xs);
		cursor: pointer;
		transition:
			color var(--transition-fast),
			border-color var(--transition-fast);
	}

	.env-selector:hover {
		color: var(--text);
		border-color: var(--border);
	}

	.env-selector:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.env-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--success);
	}

	.sync-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	/* ── Toggle switch ───────────────────────────────────── */
	.toggle {
		position: relative;
		display: inline-flex;
		cursor: pointer;
	}

	.toggle input {
		position: absolute;
		opacity: 0;
		width: 0;
		height: 0;
	}

	.toggle-track {
		width: 32px;
		height: 18px;
		border-radius: 9px;
		background: var(--border);
		position: relative;
		transition: background 0.2s ease;
	}

	.toggle-track::after {
		content: '';
		position: absolute;
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: white;
		top: 2px;
		left: 2px;
		transition: transform 0.2s ease;
	}

	.toggle input:checked + .toggle-track {
		background: var(--accent);
	}

	.toggle input:checked + .toggle-track::after {
		transform: translateX(14px);
	}

	.toggle input:disabled + .toggle-track {
		opacity: 0.6;
	}

	/* ── Config button ───────────────────────────────────── */
	.config-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 5px;
		padding: 6px 8px;
		border: none;
		border-radius: var(--radius-sm);
		background: transparent;
		color: var(--muted);
		font-size: var(--text-xs);
		cursor: pointer;
		transition:
			color var(--transition-fast),
			background var(--transition-fast);
	}

	.config-btn:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 60%, transparent);
	}

	.config-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* ── Terminal area ───────────────────────────────────── */
	.terminal-area {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		min-width: 0;
	}

	/* TerminalWorkspace renders a <section> that needs to fill the flex container */
	.terminal-area > :global(*) {
		flex: 1;
		min-width: 0;
	}

	.session-indicator {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--muted);
		white-space: nowrap;
		flex-shrink: 0;
		margin-left: auto;
	}

	.session-dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--success);
		flex-shrink: 0;
	}

	/* ── Empty state ─────────────────────────────────────── */
	.empty-state {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		color: var(--muted);
		padding: 24px;
	}

	.empty-state h2 {
		margin: 0;
		font-family: var(--font-display);
		font-size: var(--text-2xl);
		color: var(--text);
	}

	.empty-state p {
		margin: 0;
		font-size: var(--text-base);
		max-width: 40ch;
		text-align: center;
		line-height: 1.5;
	}
</style>
