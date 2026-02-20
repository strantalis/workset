<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchSettings, setDefaultSetting } from '../api/settings';
	import type {
		AppVersion,
		SettingsDefaultField,
		SettingsDefaults,
		SettingsSnapshot,
		UpdateCheckResult,
		UpdatePreferences,
		UpdateState,
	} from '../types';
	import { activeWorkspace } from '../state';
	import { toErrorMessage } from '../errors';
	import SettingsSidebar from './settings/SettingsSidebar.svelte';
	import {
		createSettingsPanelSideEffects,
		DEFAULT_UPDATE_PREFERENCES,
	} from './settings/settingsPanelSideEffects';
	import WorkspaceDefaults from './settings/sections/WorkspaceDefaults.svelte';
	import AgentDefaults from './settings/sections/AgentDefaults.svelte';
	import SessionDefaults from './settings/sections/SessionDefaults.svelte';
	import GitHubAuth from './settings/sections/GitHubAuth.svelte';
	import AliasManager from './settings/sections/AliasManager.svelte';
	import GroupManager from './settings/sections/GroupManager.svelte';
	import AboutSection from './settings/sections/AboutSection.svelte';
	import Button from './ui/Button.svelte';

	interface Props {
		onClose: () => void;
	}

	const { onClose }: Props = $props();

	type FieldId = SettingsDefaultField;
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
		{ id: 'agentModel', key: 'defaults.agent_model' },
		{ id: 'terminalIdleTimeout', key: 'defaults.terminal_idle_timeout' },
		{ id: 'terminalProtocolLog', key: 'defaults.terminal_protocol_log' },
		{ id: 'terminalDebugOverlay', key: 'defaults.terminal_debug_overlay' },
	];

	const sideEffects = createSettingsPanelSideEffects();

	let snapshot: SettingsSnapshot | null = $state(null);
	let loading = $state(true);
	let saving = $state(false);
	let restartingSessiond = $state(false);
	let resettingTerminalLayout = $state(false);
	let error: string | null = $state(null);
	let success: string | null = $state(null);
	let baseline: Record<FieldId, string> = $state({} as Record<FieldId, string>);
	let draft: Record<FieldId, string> = $state({} as Record<FieldId, string>);

	let activeSection = $state('workspace');
	let aliasCount = $state(0);
	let groupCount = $state(0);
	let appVersion = $state<AppVersion | null>(null);
	let updatePreferences = $state<UpdatePreferences>(DEFAULT_UPDATE_PREFERENCES);
	let updateState = $state<UpdateState | null>(null);
	let updateCheck = $state<UpdateCheckResult | null>(null);
	let updateBusy = $state(false);
	let updateError = $state<string | null>(null);

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
			error = toErrorMessage(err, 'Failed to update settings.');
		} finally {
			loading = false;
		}
	};

	const saveChanges = async (): Promise<void> => {
		if (saving || !snapshot) {
			return;
		}

		const updates = changedFields();
		const shouldRefreshTerminalDefaults = updates.some(
			(field) => field.id === 'terminalDebugOverlay',
		);
		const shouldRestartSessiond = updates.some((field) => field.id === 'terminalProtocolLog');
		const statusMessage =
			updates.length === 1 ? 'Saved 1 change.' : `Saved ${updates.length} changes.`;

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
				error = `Failed to save: ${toErrorMessage(err, 'Failed to update settings.')}`;
				break;
			}
		}

		if (!error) {
			baseline = { ...draft };
			if (shouldRefreshTerminalDefaults) {
				const { refreshTerminalDefaults } = await import('../terminal/terminalService');
				await refreshTerminalDefaults();
			}
			if (shouldRestartSessiond) {
				const restartResult = await sideEffects.restartSessiond();
				if (restartResult.error) {
					error = `Saved settings, but failed to restart session daemon for terminal protocol logging: ${restartResult.error}`;
				} else if (restartResult.success) {
					success = `${statusMessage} ${restartResult.success}`;
				}
			}

			if (!error && !success) {
				success = statusMessage;
			}
		}

		saving = false;
	};

	const handleRestartSessiond = async (): Promise<void> => {
		if (saving || restartingSessiond) {
			return;
		}
		restartingSessiond = true;
		error = null;
		success = null;
		try {
			const result = await sideEffects.restartSessiond();
			error = result.error ?? null;
			success = result.success ?? null;
		} finally {
			restartingSessiond = false;
		}
	};

	const handleResetTerminalLayout = async (): Promise<void> => {
		if (saving || restartingSessiond || resettingTerminalLayout) {
			return;
		}
		const workspace = $activeWorkspace;
		if (!workspace) {
			error = 'Select a workspace before resetting terminal layout.';
			return;
		}
		const confirmed = window.confirm(
			`Reset the terminal layout for "${workspace.name}"? This will close existing panes and stop running terminal sessions.`,
		);
		if (!confirmed) {
			return;
		}
		resettingTerminalLayout = true;
		error = null;
		success = null;
		try {
			const result = await sideEffects.resetTerminalLayout(workspace);
			error = result.error ?? null;
			success = result.success ?? null;
		} finally {
			resettingTerminalLayout = false;
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

	const getSectionTitle = (section: string): string => {
		const titles: Record<string, string> = {
			workspace: 'Workspace',
			agent: 'Agent',
			session: 'Terminal',
			github: 'GitHub',
			aliases: 'Repo Catalog',
			groups: 'Templates',
			about: 'About',
		};
		return titles[section] ?? 'Settings';
	};

	const handleUpdateChannelChange = async (channel: string): Promise<void> => {
		updateError = null;
		const result = await sideEffects.setUpdateChannel(channel);
		if (result.error) {
			updateError = result.error;
			return;
		}
		if (result.updatePreferences) {
			updatePreferences = result.updatePreferences;
			updateCheck = null;
		}
	};

	const handleCheckForUpdates = async (): Promise<void> => {
		if (updateBusy) {
			return;
		}
		updateBusy = true;
		updateError = null;
		try {
			const result = await sideEffects.checkForUpdates(updatePreferences.channel);
			if (result.error) {
				updateError = result.error;
				return;
			}
			updateCheck = result.updateCheck ?? null;
			updateState = result.updateState ?? null;
		} finally {
			updateBusy = false;
		}
	};

	const handleUpdateAndRestart = async (): Promise<void> => {
		if (updateBusy) {
			return;
		}
		updateBusy = true;
		updateError = null;
		try {
			const result = await sideEffects.startUpdate(updatePreferences.channel);
			if (result.error) {
				updateError = result.error;
				return;
			}
			updateState = result.updateState ?? null;
		} finally {
			updateBusy = false;
		}
	};

	onMount(() => {
		void loadSettings();
		void (async () => {
			appVersion = await sideEffects.loadAppVersion();
		})();
		void (async () => {
			const bootstrap = await sideEffects.loadUpdateBootstrap();
			updatePreferences = bootstrap.updatePreferences;
			updateState = bootstrap.updateState;
		})();
	});
</script>

<div class="panel" role="dialog" aria-modal="true" aria-label="Settings">
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
				<header class="content-header">
					<h2 class="content-title">{getSectionTitle(activeSection)}</h2>
					<button class="close-btn" onclick={onClose} aria-label="Close settings">×</button>
				</header>
				<div class="content-body">
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
							onResetTerminalLayout={handleResetTerminalLayout}
							{restartingSessiond}
							{resettingTerminalLayout}
						/>
					{:else if activeSection === 'github'}
						<GitHubAuth />
					{:else if activeSection === 'aliases'}
						<AliasManager onAliasCountChange={(count) => (aliasCount = count)} />
					{:else if activeSection === 'groups'}
						<GroupManager onGroupCountChange={(count) => (groupCount = count)} />
					{:else if activeSection === 'about'}
						<AboutSection
							{appVersion}
							{updatePreferences}
							{updateState}
							{updateCheck}
							{updateBusy}
							{updateError}
							onUpdateChannelChange={handleUpdateChannelChange}
							onCheckForUpdates={handleCheckForUpdates}
							onUpdateAndRestart={handleUpdateAndRestart}
						/>
					{/if}
				</div>
			</div>
		</div>
		<footer class="footer">
			<div class="meta">
				<span class="config-label">Config</span>
				<span class="config-path">{snapshot.configPath}</span>
			</div>
			<div class="ws-spacer"></div>
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
		width: min(900px, 94vw);
		max-height: 86vh;
		display: flex;
		flex-direction: column;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		box-shadow: var(--shadow-lg);
		overflow: hidden;
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
		background: var(--panel);
	}

	.content {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		background: var(--bg);
	}

	.content-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-4);
		padding: 20px 24px;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
		background: var(--bg);
	}

	.content-title {
		font-size: var(--text-lg);
		font-weight: 600;
		font-family: var(--font-display);
		margin: 0;
		color: var(--text);
		letter-spacing: -0.01em;
	}

	.close-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 6px;
		border: none;
		background: transparent;
		color: var(--subtle);
		font-size: var(--text-xl);
		line-height: 1;
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.close-btn:hover {
		background: var(--panel-strong);
		color: var(--text);
	}

	.content-body {
		flex: 1;
		overflow-y: auto;
		padding: var(--space-5) var(--space-6);
		background: var(--bg);
	}

	.content-body::-webkit-scrollbar {
		width: 6px;
	}

	.content-body::-webkit-scrollbar-track {
		background: transparent;
	}

	.content-body::-webkit-scrollbar-thumb {
		background: rgba(255, 255, 255, 0.15);
		border-radius: 3px;
	}

	.content-body::-webkit-scrollbar-thumb:hover {
		background: rgba(255, 255, 255, 0.25);
	}

	.footer {
		display: flex;
		align-items: center;
		gap: var(--space-4);
		padding: var(--space-4) var(--space-6);
		border-top: 1px solid var(--border);
		background: var(--panel);
		flex-shrink: 0;
	}

	.config-label {
		font-size: var(--text-xs);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
		font-weight: 500;
	}

	.config-path {
		font-size: var(--text-mono-sm);
		color: var(--text);
		font-family: var(--font-mono);
		opacity: 0.7;
	}

	.status {
		font-size: var(--text-sm);
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
