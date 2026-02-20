<script lang="ts">
	import {
		Search,
		Command,
		Box,
		LayoutGrid,
		Terminal,
		GitPullRequest,
		GitBranch,
		Sparkles,
		Plus,
	} from '@lucide/svelte';
	import type { WorksetSummary } from '../../view-models/worksetViewModel';

	export type AppView =
		| 'workset-hub'
		| 'command-center'
		| 'terminal-cockpit'
		| 'pr-orchestration'
		| 'skill-registry'
		| 'onboarding';

	type PaletteItem = {
		id: string;
		type: 'workset' | 'view';
		label: string;
		description: string;
		view?: AppView;
		worksetId?: string;
	};

	interface Props {
		open: boolean;
		worksets: WorksetSummary[];
		shortcutMap?: Map<string, number>;
		onClose: () => void;
		onSelectView: (view: AppView) => void;
		onSelectWorkset: (workspaceId: string) => void;
	}

	const { open, worksets, shortcutMap, onClose, onSelectView, onSelectWorkset }: Props = $props();

	let query = $state('');
	let selectedIndex = $state(0);
	let inputRef = $state<HTMLInputElement | null>(null);

	const viewItems: PaletteItem[] = [
		{
			id: 'view:workset-hub',
			type: 'view',
			label: 'Workset Hub',
			description: 'Browse and organize worksets',
			view: 'workset-hub',
		},
		{
			id: 'view:command-center',
			type: 'view',
			label: 'Command Center',
			description: 'Repository status and local changes',
			view: 'command-center',
		},
		{
			id: 'view:terminal-cockpit',
			type: 'view',
			label: 'Engineering Cockpit',
			description: 'Workspace terminal control surface',
			view: 'terminal-cockpit',
		},
		{
			id: 'view:pr-orchestration',
			type: 'view',
			label: 'PR Orchestration',
			description: 'PR and review operations',
			view: 'pr-orchestration',
		},
		{
			id: 'view:skill-registry',
			type: 'view',
			label: 'Skill Registry',
			description: 'Manage agent skills',
			view: 'skill-registry',
		},
		{
			id: 'view:onboarding',
			type: 'view',
			label: 'New Workset',
			description: 'Create and initialize a new workset',
			view: 'onboarding',
		},
	];

	const worksetLookup = $derived.by(() => {
		const map = new Map<string, WorksetSummary>();
		for (const ws of worksets) {
			map.set(ws.id, ws);
		}
		return map;
	});

	const filteredWorksetItems = $derived.by(() => {
		const normalized = query.trim().toLowerCase();
		const wsItems: PaletteItem[] = worksets
			.filter((workset) => !workset.archived)
			.map((workset) => ({
				id: `workset:${workset.id}`,
				type: 'workset' as const,
				label: workset.label,
				description: workset.description,
				worksetId: workset.id,
			}));
		if (!normalized) return wsItems;
		return wsItems.filter((item) =>
			`${item.label} ${item.description}`.toLowerCase().includes(normalized),
		);
	});

	const filteredViewItems = $derived.by(() => {
		const normalized = query.trim().toLowerCase();
		if (!normalized) return viewItems;
		return viewItems.filter((item) =>
			`${item.label} ${item.description}`.toLowerCase().includes(normalized),
		);
	});

	const items = $derived([...filteredWorksetItems, ...filteredViewItems]);

	const resetState = (): void => {
		query = '';
		selectedIndex = 0;
	};

	const selectItem = (item: PaletteItem): void => {
		if (item.type === 'view' && item.view) {
			onSelectView(item.view);
			onClose();
			resetState();
			return;
		}
		if (item.worksetId) {
			onSelectWorkset(item.worksetId);
			onClose();
			resetState();
		}
	};

	const onKeydown = (event: KeyboardEvent): void => {
		if (!open) return;
		if (event.key === 'Escape') {
			event.preventDefault();
			onClose();
			resetState();
			return;
		}
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			selectedIndex = Math.min(selectedIndex + 1, Math.max(items.length - 1, 0));
			return;
		}
		if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedIndex = Math.max(selectedIndex - 1, 0);
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			const current = items[selectedIndex];
			if (current) {
				selectItem(current);
			}
		}
	};

	$effect(() => {
		if (!open) return;
		selectedIndex = 0;
		requestAnimationFrame(() => {
			inputRef?.focus();
		});
	});
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<div class="palette-overlay" role="presentation">
		<button
			type="button"
			class="overlay-dismiss"
			aria-label="Close command palette"
			onclick={onClose}
		></button>
		<div class="palette" role="dialog" aria-modal="true" aria-label="Command palette" tabindex="-1">
			<div class="search-row">
				<span class="search-icon"><Search size={16} /></span>
				<input
					bind:this={inputRef}
					type="text"
					placeholder="Switch workset, navigate, search repos..."
					bind:value={query}
				/>
				<kbd class="kbd ui-kbd">esc</kbd>
			</div>
			<div class="result-list">
				{#if items.length === 0}
					<div class="empty ws-empty-state">
						<p class="ws-empty-state-copy">No matching items</p>
					</div>
				{:else}
					{#if filteredWorksetItems.length > 0}
						<div class="section-header ws-section-title">WORKSETS</div>
						{#each filteredWorksetItems as item, i (item.id)}
							{@const globalIdx = i}
							{@const ws = worksetLookup.get(item.worksetId ?? '')}
							<button
								type="button"
								class:selected={selectedIndex === globalIdx}
								onmouseenter={() => (selectedIndex = globalIdx)}
								onclick={() => selectItem(item)}
							>
								<span class="icon">
									<Box size={14} />
								</span>
								<span class="text">
									<span class="label">{item.label}</span>
									<span class="description">{item.description}</span>
								</span>
								{#if ws}
									<span class="item-meta">
										{#if ws.linesAdded > 0 || ws.linesRemoved > 0}
											<span class="meta-diff">
												<span class="plus">+{ws.linesAdded}</span>
												<span class="minus">-{ws.linesRemoved}</span>
											</span>
										{/if}
										<span class="meta-branch">
											<GitBranch size={11} />
											{ws.branch}
										</span>
										<span class="meta-health">
											{#each ws.health as status, idx (`${ws.id}-health-${idx}`)}
												<span class="ws-dot ws-dot-sm ws-dot-{status}"></span>
											{/each}
										</span>
										{#if shortcutMap?.get(item.worksetId ?? '')}
											<kbd class="kbd ui-kbd">⌘{shortcutMap.get(item.worksetId ?? '')}</kbd>
										{/if}
									</span>
								{/if}
							</button>
						{/each}
					{/if}
					{#if filteredViewItems.length > 0}
						<div class="section-header ws-section-title">NAVIGATE</div>
						{#each filteredViewItems as item, i (item.id)}
							{@const globalIdx = filteredWorksetItems.length + i}
							<button
								type="button"
								class:selected={selectedIndex === globalIdx}
								onmouseenter={() => (selectedIndex = globalIdx)}
								onclick={() => selectItem(item)}
							>
								<span class="icon">
									{#if item.view === 'workset-hub'}
										<LayoutGrid size={14} />
									{:else if item.view === 'terminal-cockpit'}
										<Terminal size={14} />
									{:else if item.view === 'pr-orchestration'}
										<GitPullRequest size={14} />
									{:else if item.view === 'skill-registry'}
										<Sparkles size={14} />
									{:else if item.view === 'onboarding'}
										<Plus size={14} />
									{:else}
										<Command size={14} />
									{/if}
								</span>
								<span class="text">
									<span class="label">{item.label}</span>
									<span class="description">{item.description}</span>
								</span>
							</button>
						{/each}
					{/if}
				{/if}
			</div>
			<div class="footer">
				<span><kbd class="kbd ui-kbd">↑↓</kbd> navigate</span>
				<span><kbd class="kbd ui-kbd">↵</kbd> open</span>
				<span><kbd class="kbd ui-kbd">esc</kbd> close</span>
				<span><kbd class="kbd ui-kbd">⌘1-5</kbd> direct switch</span>
				<span><kbd class="kbd ui-kbd">⌘K</kbd> toggle</span>
			</div>
		</div>
	</div>
{/if}

<style>
	.palette-overlay {
		position: fixed;
		inset: 0;
		z-index: 400;
		display: grid;
		place-items: start center;
		padding-top: 80px;
		background: rgba(3, 6, 10, 0.72);
		backdrop-filter: blur(3px);
	}

	.overlay-dismiss {
		position: absolute;
		inset: 0;
		border: none;
		background: transparent;
		padding: 0;
	}

	.palette {
		position: relative;
		z-index: 1;
		width: min(620px, calc(100vw - 24px));
		border-radius: 14px;
		border: 1px solid var(--border);
		background: var(--panel);
		overflow: hidden;
		box-shadow: var(--shadow-lg);
	}

	.search-row {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px;
		border-bottom: 1px solid var(--border);
	}

	.search-icon {
		color: var(--muted);
	}

	.search-row input {
		flex: 1;
		background: transparent;
		border: none;
		color: var(--text);
		font-size: var(--text-md);
	}

	.search-row input:focus {
		outline: none;
	}

	.section-header {
		padding: 8px 8px 4px;
		user-select: none;
	}

	.result-list {
		max-height: min(48vh, 420px);
		overflow: auto;
		padding: 8px;
		display: grid;
		gap: 2px;
	}

	.result-list button {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		text-align: left;
		padding: 8px;
		border-radius: 10px;
		border: 1px solid transparent;
		background: transparent;
		color: inherit;
		cursor: pointer;
	}

	.result-list button:hover,
	.result-list button.selected {
		background: var(--panel-strong);
		border-color: var(--border);
	}

	.icon {
		display: inline-grid;
		place-items: center;
		width: 28px;
		height: 28px;
		border-radius: 8px;
		border: 1px solid var(--border);
		color: var(--muted);
		flex-shrink: 0;
	}

	.text {
		display: inline-grid;
		gap: 1px;
		flex: 1;
		min-width: 0;
	}

	.label {
		font-size: var(--text-base);
		color: var(--text);
	}

	.description {
		font-size: var(--text-xs);
		color: var(--muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.item-meta {
		display: inline-flex;
		align-items: center;
		gap: 10px;
		flex-shrink: 0;
		margin-left: auto;
	}

	.meta-diff {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
	}

	.meta-diff .plus {
		color: var(--success);
	}

	.meta-diff .minus {
		color: var(--danger);
	}

	.meta-branch {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: var(--muted);
	}

	.meta-health {
		display: inline-flex;
		align-items: center;
		gap: 3px;
	}

	.empty {
		padding: 20px;
	}

	.footer {
		display: flex;
		gap: 16px;
		padding: 8px 12px;
		border-top: 1px solid var(--border);
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.kbd {
		justify-content: center;
		min-width: 20px;
		height: 18px;
		padding: 0 4px;
	}
</style>
