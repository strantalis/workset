<script lang="ts">
	import { Folder, Bot, Terminal, Github, Database, LayoutTemplate, Info } from '@lucide/svelte';

	interface Props {
		activeSection: string;
		onSelectSection: (section: string) => void;
		aliasCount?: number;
		groupCount?: number;
	}

	const { activeSection, onSelectSection, aliasCount = 0, groupCount = 0 }: Props = $props();

	type SidebarItem = {
		id: string;
		label: string;
		icon: typeof Folder;
		count?: number;
	};

	type SidebarGroup = {
		title: string;
		items: SidebarItem[];
	};

	const groups = $derived([
		{
			title: 'GENERAL',
			items: [
				{ id: 'workspace', label: 'Workspace', icon: Folder },
				{ id: 'agent', label: 'Agent', icon: Bot },
				{ id: 'session', label: 'Terminal', icon: Terminal },
			],
		},
		{
			title: 'INTEGRATIONS',
			items: [{ id: 'github', label: 'GitHub', icon: Github }],
		},
		{
			title: 'LIBRARY',
			items: [
				{ id: 'aliases', label: 'Repo Catalog', icon: Database, count: aliasCount },
				{ id: 'groups', label: 'Templates', icon: LayoutTemplate, count: groupCount },
			],
		},
		{
			title: 'INFO',
			items: [{ id: 'about', label: 'About', icon: Info }],
		},
	] as SidebarGroup[]);
</script>

<nav class="sidebar">
	<div class="sidebar-header">
		<h2 class="sidebar-title">Settings</h2>
		<p class="sidebar-subtitle">Configure your workset environment.</p>
	</div>

	{#each groups as group (group)}
		<div class="group">
			<div class="group-title">{group.title}</div>
			{#each group.items as item (item.id)}
				<button
					class="item"
					class:active={activeSection === item.id}
					type="button"
					onclick={() => onSelectSection(item.id)}
				>
					<item.icon size={16} class="item-icon" />
					<span class="label">{item.label}</span>
					{#if item.count !== undefined && item.count > 0}
						<span class="badge">{item.count}</span>
					{/if}
				</button>
			{/each}
		</div>
	{/each}
</nav>

<style>
	.sidebar {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		min-width: 220px;
		max-width: 240px;
		padding: var(--space-6) var(--space-5);
		background: var(--panel-soft);
		border-right: 1px solid var(--border);
		overflow-y: auto;
	}

	.sidebar-header {
		margin-bottom: var(--space-3);
		padding-bottom: var(--space-3);
		border-bottom: 1px solid var(--border);
	}

	.sidebar-title {
		font-size: var(--text-2xl);
		font-weight: 600;
		font-family: var(--font-display);
		color: var(--text);
		margin: 0;
		letter-spacing: -0.01em;
	}

	.sidebar-subtitle {
		font-size: var(--text-base);
		color: var(--muted);
		margin: var(--space-2) 0 0 0;
		line-height: 1.5;
	}

	.group {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.group-title {
		font-size: var(--text-xs);
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--subtle);
		padding: var(--space-2) var(--space-3);
		margin-top: var(--space-1);
		margin-bottom: var(--space-1);
	}

	.item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 10px 14px;
		border: none;
		background: transparent;
		color: var(--muted);
		font-size: var(--text-md);
		font-weight: 450;
		text-align: left;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.item:hover {
		background: var(--panel-strong);
		color: var(--text);
	}

	.item.active {
		background: var(--accent);
		color: white;
		font-weight: 500;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
	}

	.item :global(.item-icon) {
		flex-shrink: 0;
		color: currentColor;
		opacity: 0.8;
	}

	.item.active :global(.item-icon) {
		opacity: 1;
		color: white;
	}

	.label {
		flex: 1;
	}

	.badge {
		background: var(--border);
		color: var(--muted);
		font-size: var(--text-xs);
		font-weight: 600;
		padding: 2px 8px;
		border-radius: 999px;
		min-width: 22px;
		text-align: center;
	}

	.item.active .badge {
		background: rgba(0, 0, 0, 0.25);
		color: white;
	}

	@media (max-width: 720px) {
		.sidebar {
			flex-direction: row;
			min-width: unset;
			max-width: unset;
			padding: var(--space-4);
			border-right: none;
			border-bottom: 1px solid var(--border);
			overflow-x: auto;
			gap: 12px;
		}

		.sidebar-header {
			display: none;
		}

		.group {
			flex-direction: row;
			gap: 4px;
		}

		.group-title {
			display: none;
		}

		.item {
			white-space: nowrap;
			padding: 8px 12px;
		}
	}
</style>
