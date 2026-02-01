<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchSettings, setDefaultSetting, restartSessiond } from '../api';
	import type { SessiondStatusResponse } from '../api';
	import type { SettingsDefaults, SettingsSnapshot } from '../types';
	import SettingsSidebar from './settings/SettingsSidebar.svelte';
	import WorkspaceDefaults from './settings/sections/WorkspaceDefaults.svelte';
	import AgentDefaults from './settings/sections/AgentDefaults.svelte';
	import SessionDefaults from './settings/sections/SessionDefaults.svelte';
	import GitHubAuth from './settings/sections/GitHubAuth.svelte';
	import AliasManager from './settings/sections/AliasManager.svelte';
	import GroupManager from './settings/sections/GroupManager.svelte';
	import Button from './ui/Button.svelte';

	interface Props {
		onClose: () => void;
	}

	const { onClose }: Props = $props();

	type FieldId = keyof SettingsDefaults;
	type Field = {
		id: FieldId;
		key: string;
	};

	const allFields: Field[] = [
		{ id: 'workspace', key: 'defaults.workspace' },
		{ id: 'remote', key: 'defaults.remote' },
		{ id: 'baseBranch', key: 'defaults.base_branch' },
		{ id: 'workspaceRoot', key: 'defaults.workspace_root' },
		{ id: 'repoStoreRoot', key: 'defaults.repo_store_root' },
		{ id: 'agent', key: 'defaults.agent' },
		{ id: 'terminalRenderer', key: 'defaults.terminal_renderer' },
		{ id: 'terminalIdleTimeout', key: 'defaults.terminal_idle_timeout' },
		{ id: 'terminalProtocolLog', key: 'defaults.terminal_protocol_log' },
	];

	let snapshot: SettingsSnapshot | null = $state(null);
	let loading = $state(true);
	let saving = $state(false);
	let restartingSessiond = $state(false);
	let error: string | null = $state(null);
	let success: string | null = $state(null);
	let baseline: Record<FieldId, string> = $state({} as Record<FieldId, string>);
	let draft: Record<FieldId, string> = $state({} as Record<FieldId, string>);

	let activeSection = $state('workspace');
	let aliasCount = $state(0);
	let groupCount = $state(0);

	const formatError = (err: unknown): string => {
		if (err instanceof Error) {
			return err.message;
		}
		return 'Failed to update settings.';
	};

	const buildDraft = (defaults: SettingsDefaults): void => {
		const next: Record<FieldId, string> = {} as Record<FieldId, string>;
		allFields.forEach((field) => {
			next[field.id] = defaults[field.id] ?? '';
		});
		baseline = next;
		draft = { ...next };
	};

	const updateField = (id: FieldId, value: string): void => {
		draft = { ...draft, [id]: value };
	};

	const changedFields = (): Field[] =>
		allFields.filter((field) => draft[field.id] !== baseline[field.id]);

	const dirtyCount = (): number => changedFields().length;

	const loadSettings = async (): Promise<void> => {
		loading = true;
		error = null;
		success = null;
		try {
			const data = await fetchSettings();
			snapshot = data;
			buildDraft(data.defaults);
		} catch (err) {
			error = formatError(err);
		} finally {
			loading = false;
		}
	};

	const saveChanges = async (): Promise<void> => {
		if (saving || !snapshot) {
			return;
		}
		const updates = changedFields();
		if (updates.length === 0) {
			success = 'No changes to save.';
			return;
		}
		saving = true;
		error = null;
		success = null;
		for (const field of updates) {
			try {
				await setDefaultSetting(field.key, draft[field.id] ?? '');
			} catch (err) {
				error = `Failed to save: ${formatError(err)}`;
				break;
			}
		}
		if (!error) {
			baseline = { ...draft };
			success = `Saved ${updates.length} change${updates.length === 1 ? '' : 's'}.`;
		}
		saving = false;
	};

	const handleRestartSessiond = async (): Promise<void> => {
		if (saving || restartingSessiond) return;
		restartingSessiond = true;
		error = null;
		success = null;
		try {
			const status = await Promise.race([
				restartSessiond('settings_panel'),
				new Promise<SessiondStatusResponse>((_, reject) => {
					window.setTimeout(() => {
						reject(new Error('Session daemon restart timed out.'));
					}, 20000);
				}),
			]);
			if (status?.available) {
				success = status.warning
					? `Session daemon restarted. ${status.warning}`
					: 'Session daemon restarted.';
			} else {
				const warning = status?.warning ? ` ${status.warning}` : '';
				error = status?.error
					? `Failed to restart: ${status.error}${warning}`
					: `Failed to restart session daemon.${warning}`;
			}
		} catch (err) {
			error = `Failed to restart: ${formatError(err)}`;
		} finally {
			restartingSessiond = false;
		}
	};

	const resetChanges = (): void => {
		draft = { ...baseline };
		success = null;
		error = null;
	};

	const selectSection = (section: string): void => {
		activeSection = section;
		success = null;
		error = null;
	};

	onMount(() => {
		void loadSettings();
	});
</script>

<div class="panel" role="dialog" aria-modal="true" aria-label="Settings">
	<header class="header">
		<div>
			<div class="title">Settings</div>
			<div class="subtitle">Configure defaults, aliases, and groups.</div>
		</div>
		<Button variant="ghost" onclick={onClose}>Close</Button>
	</header>

	{#if loading}
		<div class="state">Loading settings...</div>
	{:else if error && !snapshot}
		<div class="state error">
			<div class="message">{error}</div>
			<Button variant="ghost" onclick={loadSettings}>Retry</Button>
		</div>
	{:else if snapshot}
		<div class="body">
			<SettingsSidebar {activeSection} onSelectSection={selectSection} {aliasCount} {groupCount} />

			<div class="content">
				{#if activeSection === 'workspace'}
					<WorkspaceDefaults {draft} {baseline} onUpdate={updateField} />
				{:else if activeSection === 'agent'}
					<AgentDefaults {draft} {baseline} onUpdate={updateField} />
				{:else if activeSection === 'session'}
					<SessionDefaults
						{draft}
						{baseline}
						onUpdate={updateField}
						onRestartSessiond={handleRestartSessiond}
						{restartingSessiond}
					/>
				{:else if activeSection === 'github'}
					<GitHubAuth />
				{:else if activeSection === 'aliases'}
					<AliasManager onAliasCountChange={(count) => (aliasCount = count)} />
				{:else if activeSection === 'groups'}
					<GroupManager onGroupCountChange={(count) => (groupCount = count)} />
				{/if}
			</div>
		</div>

		<footer class="footer">
			<div class="meta">
				<span class="config-label">Config</span>
				<span class="config-path">{snapshot.configPath}</span>
			</div>
			<div class="spacer"></div>
			{#if error}
				<span class="status error">{error}</span>
			{:else if success}
				<span class="status success">{success}</span>
			{:else if dirtyCount() > 0}
				<span class="status dirty">{dirtyCount()} unsaved</span>
			{/if}
			{#if activeSection === 'workspace' || activeSection === 'agent' || activeSection === 'session'}
				<Button variant="ghost" onclick={resetChanges} disabled={dirtyCount() === 0 || saving}>
					Reset
				</Button>
				<Button variant="primary" onclick={saveChanges} disabled={saving || dirtyCount() === 0}>
					{saving ? 'Saving...' : 'Save'}
				</Button>
			{/if}
		</footer>
	{/if}
</div>

<style>
	.panel {
		width: min(960px, 94vw);
		max-height: 86vh;
		display: flex;
		flex-direction: column;
		background: var(--panel-strong);
		border: 1px solid rgba(255, 255, 255, 0.08);
		border-radius: 20px;
		box-shadow: var(--shadow-lg), var(--inset-highlight);
		overflow: hidden;
	}

	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-4);
		padding: var(--space-5) var(--space-6);
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);
	}

	.title {
		font-size: 20px;
		font-weight: 600;
		font-family: var(--font-display);
	}

	.subtitle {
		color: var(--muted);
		font-size: 13px;
		margin-top: var(--space-1);
	}

	.state {
		padding: var(--space-6);
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
	}

	.state.error {
		color: var(--warning);
	}

	.body {
		display: flex;
		flex: 1;
		min-height: 0;
		padding: var(--space-5) var(--space-6);
		gap: var(--space-6);
	}

	.content {
		flex: 1;
		min-width: 0;
		overflow-y: auto;
		padding-right: var(--space-1);
		scrollbar-width: thin;
		scrollbar-color: rgba(255, 255, 255, 0.15) transparent;
	}

	.content::-webkit-scrollbar {
		width: 6px;
	}

	.content::-webkit-scrollbar-track {
		background: transparent;
	}

	.content::-webkit-scrollbar-thumb {
		background: rgba(255, 255, 255, 0.15);
		border-radius: 3px;
	}

	.content::-webkit-scrollbar-thumb:hover {
		background: rgba(255, 255, 255, 0.25);
	}

	.footer {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-4) var(--space-6);
		border-top: 1px solid rgba(255, 255, 255, 0.06);
		background: var(--panel);
	}

	.meta {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.config-label {
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
	}

	.config-path {
		font-size: 12px;
		color: var(--text);
		opacity: 0.7;
	}

	.spacer {
		flex: 1;
	}

	.status {
		font-size: 12px;
		font-weight: 500;
		padding: var(--space-1) 10px;
		border-radius: 999px;
	}

	.status.dirty {
		background: rgba(234, 179, 8, 0.15);
		color: var(--warning);
	}

	.status.success {
		background: rgba(74, 222, 128, 0.15);
		color: var(--success);
	}

	.status.error {
		background: var(--danger-subtle);
		color: var(--danger);
	}

	@media (max-width: 720px) {
		.panel {
			width: 100%;
			height: 100%;
			border-radius: 0;
			max-height: 100vh;
		}

		.body {
			flex-direction: column;
		}

		.meta {
			display: none;
		}
	}
</style>
