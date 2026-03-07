<script lang="ts">
	import {
		AlertCircle,
		ArrowLeft,
		ArrowUpRight,
		ChevronRight,
		Command,
		GitBranch,
		GitPullRequest,
		Pin,
	} from '@lucide/svelte';
	import type { WorksetSummary } from '../../view-models/worksetViewModel';

	type ThreadStatus = 'active' | 'in-review' | 'merged' | 'stale';

	interface Props {
		workset: WorksetSummary | null;
		shortcutNumber?: number;
		showShortcut?: boolean;
		showPaletteHint?: boolean;
		showPopoutToggle?: boolean;
		workspacePoppedOut?: boolean;
		onTogglePopout?: () => void;
		onOpenPalette?: () => void;
	}

	const {
		workset,
		shortcutNumber,
		showShortcut = true,
		showPaletteHint = true,
		showPopoutToggle = false,
		workspacePoppedOut = false,
		onTogglePopout,
		onOpenPalette,
	}: Props = $props();

	const hasDiff = $derived((workset?.linesAdded ?? 0) > 0 || (workset?.linesRemoved ?? 0) > 0);
	const worksetLabel = $derived.by(() => {
		if (!workset) return '';
		const value = workset.workset.trim();
		return value.length > 0 ? value : workset.label;
	});

	const threadStatus = $derived.by<ThreadStatus>(() => {
		if (!workset) return 'active';
		if (workset.openPrs > 0) return 'in-review';
		if (workset.dirtyCount > 0) return 'active';
		if (workset.mergedPrs > 0) return 'merged';
		if (workset.lastActiveTs <= 0) return 'active';
		const age = Date.now() - workset.lastActiveTs;
		return age > 14 * 24 * 60 * 60 * 1000 ? 'stale' : 'active';
	});

	const threadStatusLabel = $derived.by(() => {
		if (threadStatus === 'in-review') return 'In Review';
		if (threadStatus === 'merged') return 'Merged';
		if (threadStatus === 'stale') return 'Stale';
		return 'Active';
	});
</script>

<div class="context-bar" role="region" aria-label="Workset context">
	<!-- Reserve horizontal space for macOS traffic lights (HiddenInset titlebar) -->
	<span class="traffic-light-zone"></span>
	<img src="/images/appicon.png" alt="Workset" class="app-icon" />

	{#if workset}
		<div class="crumb">
			<span class="workset-name">{worksetLabel}</span>
			{#if workset.pinned}
				<Pin size={10} class="pin" />
			{/if}
			{#if showShortcut && shortcutNumber}
				<kbd class="shortcut ui-kbd"><Command size={8} />{shortcutNumber}</kbd>
			{/if}
		</div>

		<ChevronRight size={11} class="crumb-chevron" />

		<div class="crumb thread-crumb">
			<span class="status-dot status-{threadStatus}"></span>
			<span class="thread-name">{workset.label}</span>
			<span class="thread-status">{threadStatusLabel}</span>
		</div>

		<div class="divider"></div>
		<div class="branch ws-inline">
			<span class="branch-icon"><GitBranch size={12} /></span>
			{workset.branch}
		</div>
		<div class="divider"></div>
		<div class="health" aria-label="Repository health status">
			{#each workset.health as status, index (`${workset.id}-${index}`)}
				<span class="ws-dot ws-dot-sm ws-dot-{status}"></span>
			{/each}
		</div>
		{#if hasDiff}
			<div class="divider"></div>
			<div class="diff ws-inline">
				<span class="plus">+{workset.linesAdded}</span>
				<span class="minus">-{workset.linesRemoved}</span>
			</div>
		{/if}
		<div class="stats ws-inline">
			{#if workset.dirtyCount > 0}
				<span class="warning ws-inline"><AlertCircle size={10} /> {workset.dirtyCount} dirty</span>
			{/if}
			{#if workset.openPrs > 0}
				<span class="pr ws-inline"><GitPullRequest size={10} /> {workset.openPrs} PRs</span>
			{/if}
		</div>
	{:else}
		<span class="muted">No workset selected</span>
	{/if}

	<div class="ws-spacer"></div>

	{#if showPopoutToggle}
		<button
			type="button"
			class="popout-action"
			aria-label={workspacePoppedOut ? 'Return workspace to main window' : 'Open workspace popout'}
			title={workspacePoppedOut ? 'Return to main window' : 'Open workspace popout'}
			onclick={() => onTogglePopout?.()}
		>
			{#if workspacePoppedOut}
				<ArrowLeft size={14} />
				<span>Return</span>
			{:else}
				<ArrowUpRight size={14} />
				<span>Popout</span>
			{/if}
		</button>
	{/if}

	{#if showPaletteHint}
		<button type="button" class="palette-hint" onclick={() => onOpenPalette?.()}>
			<kbd class="shortcut ui-kbd"><Command size={9} />K</kbd>
			<span>to switch</span>
		</button>
	{/if}
</div>

<style>
	.context-bar {
		height: 34px;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 0 10px 0 0;
		border-bottom: 1px solid var(--glass-border);
		background:
			linear-gradient(
				180deg,
				rgba(255, 255, 255, 0.1) 0%,
				rgba(255, 255, 255, 0.03) 44%,
				rgba(255, 255, 255, 0) 100%
			),
			var(--glass-bg);
		backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		-webkit-backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		box-shadow:
			var(--inset-highlight),
			0 6px 18px rgba(5, 10, 20, 0.22);
		--wails-draggable: drag;
		min-width: 0;
		overflow: hidden;
		position: relative;
	}

	.context-bar::after {
		content: '';
		position: absolute;
		left: 0;
		right: 0;
		top: 0;
		height: 1px;
		background: rgba(255, 255, 255, 0.16);
		pointer-events: none;
	}

	/*
	 * macOS traffic-light clearance: HiddenInset, last button right edge ≈ x=68px.
	 * Zone is a passive spacer; app icon sits immediately after it as its own flex item.
	 */
	.traffic-light-zone {
		width: 68px;
		flex-shrink: 0;
	}

	.app-icon {
		width: 18px;
		height: 18px;
		border-radius: 4px;
		object-fit: contain;
		flex-shrink: 0;
	}

	.worksets-link,
	.palette-hint,
	.popout-action,
	.quick-open-action {
		-webkit-app-region: no-drag;
	}

	.crumb {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}

	.workset-name,
	.thread-name {
		font-size: var(--text-sm);
		color: var(--text);
		max-width: min(30vw, 360px);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.pin {
		color: var(--warning);
	}

	.crumb-chevron {
		color: color-mix(in srgb, var(--muted) 72%, white);
		flex-shrink: 0;
	}

	.thread-crumb {
		gap: 7px;
	}

	.status-dot {
		width: 6px;
		height: 6px;
		border-radius: 999px;
		flex-shrink: 0;
	}

	.status-dot.status-active {
		background: var(--success);
	}

	.status-dot.status-in-review {
		background: #8b8aed;
	}

	.status-dot.status-merged {
		background: var(--accent);
	}

	.status-dot.status-stale {
		background: color-mix(in srgb, var(--muted) 68%, white);
	}

	.thread-status {
		padding: 1px 6px;
		border-radius: 999px;
		border: 1px solid var(--border);
		background: color-mix(in srgb, var(--panel-strong) 74%, transparent);
		font-size: var(--text-xs);
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
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.branch-icon {
		display: inline-flex;
		align-items: center;
		color: var(--accent);
	}

	.health {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.plus {
		color: var(--success);
	}

	.minus {
		color: var(--danger);
	}

	.warning,
	.pr {
		gap: 4px;
	}

	.pr {
		color: #8b8aed;
	}

	.palette-hint,
	.popout-action,
	.quick-open-action {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 8px;
		padding: 3px 9px;
		color: var(--muted);
		font-size: var(--text-xs);
		cursor: pointer;
	}

	.palette-hint:hover,
	.popout-action:hover,
	.quick-open-action:hover {
		border-color: var(--border);
		color: var(--text);
	}

	.muted {
		color: var(--muted);
		font-size: var(--text-sm);
	}

	@media (max-width: 1320px) {
		.thread-status {
			display: none;
		}

		.stats {
			display: none;
		}

		.workset-name,
		.thread-name {
			max-width: min(24vw, 280px);
		}
	}

	@media (max-width: 1080px) {
		.traffic-light-zone {
			width: 56px;
		}

		.branch {
			max-width: 22ch;
			overflow: hidden;
			text-overflow: ellipsis;
			white-space: nowrap;
		}

		.palette-hint span {
			display: none;
		}
	}
</style>
