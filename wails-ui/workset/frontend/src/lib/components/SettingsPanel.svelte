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
	import { X } from '@lucide/svelte';
	import SettingsSidebar from './settings/SettingsSidebar.svelte';
	import { createSettingsPanelSideEffects } from './settings/settingsPanelSideEffects';
	import WorkspaceDefaults from './settings/sections/WorkspaceDefaults.svelte';
	import SessionDefaults from './settings/sections/SessionDefaults.svelte';
	import SystemSection from './settings/sections/SystemSection.svelte';
	import GitHubAuth from './settings/sections/GitHubAuth.svelte';
	import AliasManager from './settings/sections/AliasManager.svelte';
	import AboutSection from './settings/sections/AboutSection.svelte';
	import Button from './ui/Button.svelte';
	import {
		dispatchUpdatePreferencesChanged,
		DEFAULT_UPDATE_PREFERENCES,
	} from '../updatePreferences';

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
		{ id: 'thread', key: 'defaults.thread' },
		{ id: 'remote', key: 'defaults.remote' },
		{ id: 'baseBranch', key: 'defaults.base_branch' },
		{ id: 'worksetRoot', key: 'defaults.workset_root' },
		{ id: 'repoStoreRoot', key: 'defaults.repo_store_root' },
		{ id: 'agent', key: 'defaults.agent' },
		{ id: 'agentModel', key: 'defaults.agent_model' },
		{ id: 'terminalIdleTimeout', key: 'defaults.terminal_idle_timeout' },
		{ id: 'terminalProtocolLog', key: 'defaults.terminal_protocol_log' },
		{ id: 'terminalDebugOverlay', key: 'defaults.terminal_debug_overlay' },
		{ id: 'terminalFontSize', key: 'defaults.terminal_font_size' },
		{ id: 'terminalCursorBlink', key: 'defaults.terminal_cursor_blink' },
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

	let activeSection = $state('defaults');
	let aliasCount = $state(0);
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

	const sectionFieldMap: Record<string, FieldId[]> = {
		defaults: ['thread', 'remote', 'baseBranch', 'agent', 'agentModel'],
		session: [
			'terminalIdleTimeout',
			'terminalProtocolLog',
			'terminalDebugOverlay',
			'terminalFontSize',
			'terminalCursorBlink',
		],
		system: ['worksetRoot', 'repoStoreRoot'],
	};

	const dirtySections = $derived(
		new Set(
			Object.entries(sectionFieldMap)
				.filter(([, ids]) => ids.some((id) => draft[id] !== baseline[id]))
				.map(([section]) => section),
		),
	);

	/** Sections that contain saveable draft fields (show footer Save/Reset). */
	const saveSections = new Set(['defaults', 'session', 'system']);

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
			(field) =>
				field.id === 'terminalDebugOverlay' ||
				field.id === 'terminalFontSize' ||
				field.id === 'terminalCursorBlink',
		);
		const shouldRestartSessiond = updates.some(
			(field) => field.id === 'terminalProtocolLog' || field.id === 'terminalIdleTimeout',
		);
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
					error = `Saved settings, but failed to restart session daemon to apply terminal settings: ${restartResult.error}`;
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
		const thread = $activeWorkspace;
		if (!thread) {
			error = 'Select a thread before resetting terminal layout.';
			return;
		}
		const confirmed = window.confirm(
			`Reset the terminal layout for "${thread.name}"? This will close existing panes and stop running terminal sessions.`,
		);
		if (!confirmed) {
			return;
		}
		resettingTerminalLayout = true;
		error = null;
		success = null;
		try {
			const result = await sideEffects.resetTerminalLayout(thread);
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
			defaults: 'Defaults',
			session: 'Terminal',
			system: 'System',
			github: 'GitHub',
			aliases: 'Repo Catalog',
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
			dispatchUpdatePreferencesChanged(updatePreferences);
			updateCheck = null;
		}
	};

	const handleUpdateAutoCheckChange = async (enabled: boolean): Promise<void> => {
		updateError = null;
		const result = await sideEffects.setAutoCheck(enabled);
		if (result.error) {
			updateError = result.error;
			return;
		}
		if (result.updatePreferences) {
			updatePreferences = result.updatePreferences;
			dispatchUpdatePreferencesChanged(updatePreferences);
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

<div class="panel" role="region" aria-label="Settings">
	{#if loading}
		<div class="state">Loading settings...</div>
	{:else if error && !snapshot}
		<div class="state error">
			<div class="message">{error}</div>
			<Button variant="ghost" onclick={loadSettings}>Retry</Button>
		</div>
	{:else if snapshot}
		<div class="body">
			<SettingsSidebar
				{activeSection}
				onSelectSection={selectSection}
				{aliasCount}
				{dirtySections}
			/>

			<div class="content">
				<header class="content-header">
					<h2 class="content-title">{getSectionTitle(activeSection)}</h2>
					<button class="close-btn" onclick={onClose} aria-label="Close settings">
						<X size={16} />
					</button>
				</header>
				<div class="content-body">
					{#if activeSection === 'defaults'}
						<WorkspaceDefaults {draft} {baseline} onUpdate={updateField} />
					{:else if activeSection === 'session'}
						<SessionDefaults {draft} {baseline} onUpdate={updateField} />
					{:else if activeSection === 'system'}
						<SystemSection
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
					{:else if activeSection === 'about'}
						<AboutSection
							{appVersion}
							{updatePreferences}
							{updateState}
							{updateCheck}
							{updateBusy}
							{updateError}
							onUpdateChannelChange={handleUpdateChannelChange}
							onAutoCheckChange={handleUpdateAutoCheckChange}
							onCheckForUpdates={handleCheckForUpdates}
							onUpdateAndRestart={handleUpdateAndRestart}
						/>
					{/if}
				</div>
			</div>
		</div>
		<footer class="footer">
			<div class="ws-spacer"></div>
			{#if error}
				<span class="status error">{error}</span>
			{:else if success}
				<span class="status success">{success}</span>
			{:else if dirtyCount() > 0}
				<span class="status dirty">{dirtyCount()} unsaved</span>
			{/if}
			{#if saveSections.has(activeSection)}
				<Button variant="ghost" onclick={resetChanges} disabled={dirtyCount() === 0 || saving}>
					Reset
				</Button>
				<Button variant="primary" onclick={saveChanges} disabled={saving || dirtyCount() === 0}>
					{#if saving}
						Saving…
					{:else if dirtyCount() > 0}
						Save ({dirtyCount()})
					{:else}
						Save
					{/if}
				</Button>
			{/if}
		</footer>
	{/if}
</div>

<style>
	.panel {
		width: 100%;
		height: 100%;
		display: flex;
		flex-direction: column;
		background: color-mix(in srgb, var(--bg) 90%, transparent);
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
		background: color-mix(in srgb, var(--panel-strong) 90%, var(--panel));
	}

	.content {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		background: color-mix(in srgb, var(--bg) 92%, var(--panel));
	}

	.content-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-4);
		padding: 20px 24px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
		flex-shrink: 0;
		background: color-mix(in srgb, var(--panel-strong) 86%, var(--panel));
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
		width: 32px;
		height: 32px;
		border-radius: 8px;
		border: 1px solid transparent;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.close-btn:hover {
		background: var(--hover-bg-solid);
		border-color: var(--border);
		color: var(--text);
	}

	.content-body {
		flex: 1;
		overflow-y: auto;
		padding: var(--space-5) var(--space-6);
		background: color-mix(in srgb, var(--bg) 94%, var(--panel));
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
		border-top: 1px solid color-mix(in srgb, var(--border) 68%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 88%, var(--panel));
		flex-shrink: 0;
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
		.body {
			flex-direction: column;
		}
	}
</style>
