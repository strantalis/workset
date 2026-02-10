<script lang="ts">
	import {
		Command,
		ChevronRight,
		GitBranch,
		Pin,
		GitPullRequest,
		AlertCircle,
	} from '@lucide/svelte';
	import type { WorksetSummary } from '../../view-models/worksetViewModel';

	interface Props {
		workset: WorksetSummary | null;
		shortcutNumber?: number;
		showShortcut?: boolean;
		showPaletteHint?: boolean;
		onOpenHub: () => void;
		onOpenPalette?: () => void;
	}

	const {
		workset,
		shortcutNumber,
		showShortcut = true,
		showPaletteHint = true,
		onOpenHub,
		onOpenPalette,
	}: Props = $props();

	const hasDiff = $derived((workset?.linesAdded ?? 0) > 0 || (workset?.linesRemoved ?? 0) > 0);
</script>

<div class="context-bar" role="region" aria-label="Workset context">
	{#if workset}
		<button class="workset-link" type="button" onclick={onOpenHub}>
			<span class="workset-name">{workset.label}</span>
			<span class="chevron"><ChevronRight size={12} /></span>
		</button>

		{#if workset.pinned}
			<Pin size={11} class="pin" />
		{/if}

		{#if showShortcut && shortcutNumber}
			<kbd class="shortcut"><Command size={9} />{shortcutNumber}</kbd>
		{/if}

		<div class="divider"></div>
		<div class="branch">
			<span class="branch-icon"><GitBranch size={12} /></span>
			{workset.branch}
		</div>
		<div class="divider"></div>
		<div class="health" aria-label="Repository health status">
			{#each workset.health as status, index (`${workset.id}-${index}`)}
				<span class="dot {status}"></span>
			{/each}
		</div>
		{#if hasDiff}
			<div class="divider"></div>
			<div class="diff">
				<span class="plus">+{workset.linesAdded}</span>
				<span class="minus">-{workset.linesRemoved}</span>
			</div>
		{/if}
		<div class="stats">
			{#if workset.dirtyCount > 0}
				<span class="warning"><AlertCircle size={10} /> {workset.dirtyCount} dirty</span>
			{/if}
			{#if workset.openPrs > 0}
				<span class="pr"><GitPullRequest size={10} /> {workset.openPrs} PRs</span>
			{/if}
		</div>
	{/if}

	{#if showPaletteHint}
		<div class="spacer"></div>
		<button type="button" class="palette-hint" onclick={() => onOpenPalette?.()}>
			<kbd class="shortcut"><Command size={9} />K</kbd>
			<span>Command palette</span>
		</button>
	{/if}
</div>

<style>
	.context-bar {
		height: 42px;
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 0 14px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.08);
		background: color-mix(in srgb, var(--panel) 88%, transparent);
		backdrop-filter: blur(10px);
		--wails-draggable: drag;
	}

	.workset-link {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		background: transparent;
		border: none;
		padding: 0;
		cursor: pointer;
	}

	.workset-name {
		font-weight: 600;
		font-size: var(--text-base);
		color: var(--text);
	}

	.chevron {
		display: inline-flex;
		align-items: center;
		opacity: 0;
		transition: opacity 150ms ease;
		color: var(--muted);
	}

	.workset-link,
	.palette-hint {
		-webkit-app-region: no-drag;
	}

	.workset-link:hover .chevron {
		opacity: 1;
	}

	.pin {
		color: var(--warning);
	}

	.shortcut {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		border: 1px solid var(--border);
		background: var(--panel-soft);
		border-radius: 6px;
		padding: 1px 5px;
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: var(--muted);
	}

	.divider {
		width: 1px;
		height: 14px;
		background: var(--border);
	}

	.branch,
	.diff,
	.stats {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.branch-icon {
		display: inline-flex;
		align-items: center;
		color: #2d8cff;
	}

	.health {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.dot {
		width: 6px;
		height: 6px;
		border-radius: 999px;
	}

	.dot.clean {
		background: var(--success);
	}

	.dot.modified {
		background: var(--warning);
	}

	.dot.ahead {
		background: var(--accent);
	}

	.dot.error {
		background: var(--danger);
	}

	.plus {
		color: var(--success);
	}

	.minus {
		color: var(--danger);
	}

	.warning,
	.pr {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.pr {
		color: #8b8aed;
	}

	.spacer {
		flex: 1;
	}

	.palette-hint {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 8px;
		padding: 3px 6px;
		color: var(--muted);
		font-size: var(--text-xs);
		cursor: pointer;
	}

	.palette-hint:hover {
		border-color: var(--border);
		color: var(--text);
	}
</style>
