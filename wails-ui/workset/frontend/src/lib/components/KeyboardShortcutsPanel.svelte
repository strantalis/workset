<script lang="ts">
	import { X } from '@lucide/svelte';

	interface Props {
		onClose: () => void;
	}

	const { onClose }: Props = $props();

	const isMac = navigator.platform?.startsWith('Mac') ?? false;
	const mod = isMac ? '⌘' : 'Ctrl';
	const alt = isMac ? 'Opt' : 'Alt';
	const shift = 'Shift';

	type ShortcutGroup = {
		title: string;
		items: { keys: string[]; action: string }[];
	};

	const groups: ShortcutGroup[] = [
		{
			title: 'Global',
			items: [
				{ keys: [mod, shift, 'P'], action: 'Command Palette' },
				{ keys: [mod, 'P'], action: 'File Search' },
				{ keys: [mod, 'K'], action: 'Toggle Code / Terminal' },
				{ keys: [mod, 'B'], action: 'Toggle Explorer' },
				{ keys: [mod, '1–5'], action: 'Switch Workset' },
				{ keys: [mod, '?'], action: 'Keyboard Shortcuts' },
			],
		},
		{
			title: 'Terminal',
			items: [
				{ keys: [mod, 'T'], action: 'New Tab' },
				{ keys: [mod, 'W'], action: 'Close Tab' },
				{ keys: [mod, '\\'], action: 'Split Vertically' },
				{ keys: [mod, shift, '\\'], action: 'Split Horizontally' },
				{ keys: [alt, '1–9'], action: 'Focus Tab' },
				{ keys: ['Ctrl', 'Tab'], action: 'Next Tab' },
				{ keys: ['Ctrl', shift, 'Tab'], action: 'Previous Tab' },
				{ keys: [mod, alt, '↑ ↓ ← →'], action: 'Focus Pane' },
				{ keys: [mod, '= / −'], action: 'Font Size' },
				{ keys: [mod, '0'], action: 'Reset Font Size' },
			],
		},
		{
			title: 'Editor',
			items: [
				{ keys: [mod, 'S'], action: 'Save' },
				{ keys: [alt, '['], action: 'Previous File' },
				{ keys: [alt, ']'], action: 'Next File' },
			],
		},
		{
			title: 'Navigation',
			items: [
				{ keys: ['↑ / ↓'], action: 'Navigate Lists' },
				{ keys: ['Enter'], action: 'Select / Confirm' },
				{ keys: ['Esc'], action: 'Close / Cancel' },
				{ keys: ['Tab'], action: 'Trap Focus' },
			],
		},
	];
</script>

<div
	class="shortcuts-overlay"
	role="dialog"
	aria-modal="true"
	aria-label="Keyboard shortcuts"
	tabindex="-1"
	onclick={onClose}
	onkeydown={(e) => {
		if (e.key === 'Escape') onClose();
	}}
>
	<div
		class="shortcuts-panel"
		role="presentation"
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => e.stopPropagation()}
	>
		<header class="shortcuts-header">
			<h2>Keyboard Shortcuts</h2>
			<button type="button" class="close-btn" aria-label="Close" onclick={onClose}>
				<X size={16} />
			</button>
		</header>
		<div class="shortcuts-body">
			{#each groups as group (group.title)}
				<section class="shortcut-group">
					<h3 class="group-title">{group.title}</h3>
					<div class="shortcut-list">
						{#each group.items as item (`${group.title}-${item.action}`)}
							<div class="shortcut-row">
								<span class="shortcut-action">{item.action}</span>
								<span class="shortcut-keys">
									{#each item.keys as key, i (`${group.title}-${item.action}-${i}-${key}`)}
										{#if i > 0}<span class="key-sep">+</span>{/if}
										<kbd class="ui-kbd">{key}</kbd>
									{/each}
								</span>
							</div>
						{/each}
					</div>
				</section>
			{/each}
		</div>
	</div>
</div>

<style>
	.shortcuts-overlay {
		position: fixed;
		inset: 0;
		z-index: 400;
		display: grid;
		place-items: center;
		background: rgba(3, 6, 10, 0.58);
		backdrop-filter: blur(6px);
		-webkit-backdrop-filter: blur(6px);
		padding: 24px;
	}

	.shortcuts-panel {
		width: min(560px, calc(100vw - 48px));
		max-height: calc(100vh - 96px);
		border-radius: 14px;
		border: 1px solid var(--glass-border);
		background: var(--glass-bg-strong);
		backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		-webkit-backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		box-shadow: var(--glass-shadow), var(--inset-highlight);
		overflow-y: auto;
		display: flex;
		flex-direction: column;
	}

	.shortcuts-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 16px 20px;
		border-bottom: 1px solid var(--border);
	}

	.shortcuts-header h2 {
		margin: 0;
		font-size: var(--text-base);
		font-weight: 600;
		color: var(--foreground);
	}

	.close-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
	}

	.close-btn:hover {
		background: var(--panel-soft);
		color: var(--foreground);
	}

	.shortcuts-body {
		padding: 12px 20px 20px;
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 20px;
	}

	.shortcut-group {
		min-width: 0;
	}

	.group-title {
		margin: 0 0 8px;
		font-size: var(--text-xs);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--muted);
	}

	.shortcut-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.shortcut-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 4px 0;
	}

	.shortcut-action {
		font-size: var(--text-sm);
		color: var(--foreground);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.shortcut-keys {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		flex-shrink: 0;
	}

	.key-sep {
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.shortcut-keys :global(.ui-kbd) {
		font-size: var(--text-sm);
		padding: 2px 6px;
		white-space: nowrap;
	}
</style>
