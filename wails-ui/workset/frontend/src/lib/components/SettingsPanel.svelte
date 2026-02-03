<script lang="ts">
	import { onMount } from 'svelte';
	import {
		createWorkspaceTerminal,
		fetchAppVersion,
		fetchSettings,
		fetchWorkspaceTerminalLayout,
		persistWorkspaceTerminalLayout,
		restartSessiond,
		setDefaultSetting,
		stopWorkspaceTerminal,
	} from '../api';
	import type { SessiondStatusResponse } from '../api';
	import type {
		SettingsDefaults,
		SettingsSnapshot,
		TerminalLayout,
		TerminalLayoutNode,
	} from '../types';
	import { activeWorkspace } from '../state';
	import { generateTerminalName } from '../names';
	import SettingsSidebar from './settings/SettingsSidebar.svelte';
	import WorkspaceDefaults from './settings/sections/WorkspaceDefaults.svelte';
	import AgentDefaults from './settings/sections/AgentDefaults.svelte';
	import SessionDefaults from './settings/sections/SessionDefaults.svelte';
	import GitHubAuth from './settings/sections/GitHubAuth.svelte';
	import AliasManager from './settings/sections/AliasManager.svelte';
	import GroupManager from './settings/sections/GroupManager.svelte';
	import Button from './ui/Button.svelte';
	import type { AppVersion } from '../types';

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
		{ id: 'agentLaunch', key: 'defaults.agent_launch' },
		{ id: 'terminalIdleTimeout', key: 'defaults.terminal_idle_timeout' },
		{ id: 'terminalProtocolLog', key: 'defaults.terminal_protocol_log' },
	];

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
	const LAYOUT_VERSION = 1;
	const LEGACY_STORAGE_PREFIX = 'workset:terminal-layout:';
	const MIGRATION_PREFIX = 'workset:terminal-layout:migrated:v';
	const MIGRATION_VERSION = 1;

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

	const newId = (): string => {
		if (typeof crypto !== 'undefined' && crypto.randomUUID) {
			return crypto.randomUUID();
		}
		return `term-${Math.random().toString(36).slice(2)}`;
	};

	const clearLegacyLayout = (workspaceId: string): void => {
		if (!workspaceId || typeof localStorage === 'undefined') return;
		try {
			localStorage.removeItem(`${LEGACY_STORAGE_PREFIX}${workspaceId}`);
			localStorage.setItem(`${MIGRATION_PREFIX}${MIGRATION_VERSION}:${workspaceId}`, '1');
		} catch {
			// Ignore storage failures.
		}
	};

	const loadLegacyLayout = (workspaceId: string): TerminalLayout | null => {
		if (!workspaceId || typeof localStorage === 'undefined') return null;
		try {
			const raw = localStorage.getItem(`${LEGACY_STORAGE_PREFIX}${workspaceId}`);
			if (!raw) return null;
			const parsed = JSON.parse(raw) as TerminalLayout;
			if (!parsed?.root) return null;
			return parsed;
		} catch {
			return null;
		}
	};

	const collectTerminalIds = (node: TerminalLayoutNode | null | undefined): string[] => {
		if (!node) return [];
		if (node.kind === 'pane') {
			return (node.tabs ?? []).map((tab) => tab.terminalId).filter(Boolean);
		}
		return [...collectTerminalIds(node.first), ...collectTerminalIds(node.second)];
	};

	const stopSessionsForLayout = async (
		workspaceId: string,
		layout: TerminalLayout | null,
	): Promise<void> => {
		if (!layout) return;
		const ids = Array.from(new Set(collectTerminalIds(layout.root)));
		if (ids.length === 0) return;
		await Promise.allSettled(
			ids.map((terminalId) => stopWorkspaceTerminal(workspaceId, terminalId)),
		);
	};

	const buildFreshLayout = (workspaceName: string, terminalId: string): TerminalLayout => {
		const tabId = newId();
		const paneId = newId();
		return {
			version: LAYOUT_VERSION,
			root: {
				id: paneId,
				kind: 'pane',
				tabs: [
					{
						id: tabId,
						terminalId,
						title: generateTerminalName(workspaceName, 0),
					},
				],
				activeTabId: tabId,
			},
			focusedPaneId: paneId,
		};
	};

	const handleResetTerminalLayout = async (): Promise<void> => {
		if (saving || restartingSessiond || resettingTerminalLayout) return;
		const workspace = $activeWorkspace;
		if (!workspace) {
			error = 'Select a workspace before resetting terminal layout.';
			return;
		}
		const confirmed = window.confirm(
			`Reset the terminal layout for "${workspace.name}"? This will close existing panes and stop running terminal sessions.`,
		);
		if (!confirmed) return;
		resettingTerminalLayout = true;
		error = null;
		success = null;
		try {
			let layoutToStop: TerminalLayout | null = null;
			try {
				const payload = await fetchWorkspaceTerminalLayout(workspace.id);
				layoutToStop = payload?.layout ?? loadLegacyLayout(workspace.id);
			} catch {
				layoutToStop = loadLegacyLayout(workspace.id);
			}
			await stopSessionsForLayout(workspace.id, layoutToStop);
			const created = await createWorkspaceTerminal(workspace.id);
			const layout = buildFreshLayout(workspace.name, created.terminalId);
			await persistWorkspaceTerminalLayout(workspace.id, layout);
			clearLegacyLayout(workspace.id);
			window.dispatchEvent(
				new CustomEvent('workset:terminal-layout-reset', {
					detail: { workspaceId: workspace.id },
				}),
			);
			success = `Terminal layout reset for ${workspace.name}.`;
		} catch (err) {
			error = `Failed to reset terminal layout: ${formatError(err)}`;
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

	onMount(() => {
		void loadSettings();
		void (async () => {
			try {
				appVersion = await fetchAppVersion();
			} catch {
				appVersion = null;
			}
		})();
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
					<div class="about-section">
						<div class="about-header">
							<img src="images/logo.png" alt="Workset" class="about-logo" />
							<h3>Workset</h3>
							<p class="tagline">Workspace management for multi-repo development</p>
						</div>

						{#if appVersion}
							<div class="info-block">
								<div class="version-header">
									<h4>Version</h4>
									<button
										type="button"
										class="copy-btn"
										title="Copy version info"
										onclick={async () => {
											const versionText = `Workset ${appVersion?.version}${appVersion?.dirty ? '+dirty' : ''} (${appVersion?.commit || 'unknown'})`;
											try {
												if (navigator.clipboard) {
													await navigator.clipboard.writeText(versionText);
												}
											} catch {
												// Silently fail if clipboard is not available
											}
										}}
									>
										<svg
											width="14"
											height="14"
											viewBox="0 0 24 24"
											fill="none"
											stroke="currentColor"
											stroke-width="2"
										>
											<rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
											<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
										</svg>
									</button>
								</div>
								<div class="version-info">
									<div class="version-row">
										<span class="label">Version:</span>
										<span class="value">{appVersion.version}{appVersion.dirty ? '+dirty' : ''}</span
										>
									</div>
									{#if appVersion.commit}
										<div class="version-row">
											<span class="label">Commit:</span>
											<span class="value">{appVersion.commit}</span>
										</div>
									{/if}
								</div>
								<button type="button" class="update-btn" disabled>
									<svg
										width="16"
										height="16"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="2"
									>
										<path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
										<path d="M21 3v6h-6" />
										<path d="M21 3l-9 9" />
									</svg>
									Check for Updates
									<span class="coming-soon">Coming soon</span>
								</button>
							</div>
						{/if}

						<div class="info-block">
							<h4>Built With</h4>
							<div class="tech-stack">
								<span class="tech-badge">Wails</span>
								<span class="tech-badge">Svelte</span>
								<span class="tech-badge">Go</span>
								<span class="tech-badge">TypeScript</span>
							</div>
						</div>

						<div class="info-block">
							<h4>Links</h4>
							<div class="links">
								<a
									href="https://github.com/anomalyco/workset"
									target="_blank"
									rel="noopener noreferrer"
									class="link"
								>
									<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
										<path
											d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"
										/>
									</svg>
									GitHub Repository
								</a>
								<a
									href="https://github.com/anomalyco/workset/issues"
									target="_blank"
									rel="noopener noreferrer"
									class="link"
								>
									<svg
										width="16"
										height="16"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="2"
									>
										<path
											d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-1-13h2v6h-2zm0 8h2v2h-2z"
										/>
									</svg>
									Report an Issue
								</a>
							</div>
						</div>

						<div class="copyright">
							© {new Date().getFullYear()} Sean Trantalis. Open source under MIT License.
						</div>
					</div>
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
		border-radius: 0;
		max-height: 100vh;
	}

	.about-section {
		padding: 0;
		max-width: 500px;
		margin: 0 auto;
	}

	.about-header {
		margin-bottom: 24px;
		text-align: center;
	}

	.about-logo {
		width: 48px;
		height: 48px;
		margin-bottom: 12px;
		opacity: 0.9;
	}

	.about-header h3 {
		font-size: 24px;
		font-weight: 600;
		margin: 0 0 6px 0;
		color: var(--text);
	}

	.tagline {
		font-size: 14px;
		color: var(--muted);
		margin: 0;
	}

	.info-block {
		margin-bottom: 20px;
		padding-bottom: 16px;
		border-bottom: 1px solid var(--border);
	}

	.info-block:last-of-type {
		border-bottom: none;
		margin-bottom: 0;
	}

	.info-block h4 {
		font-size: 12px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
		margin: 0 0 12px 0;
	}

	.version-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 12px;
	}

	.version-header h4 {
		margin: 0;
	}

	.copy-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.copy-btn:hover {
		border-color: var(--accent);
		color: var(--accent);
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}

	.update-btn {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 16px;
		padding: 8px 16px;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		background: var(--panel-strong);
		color: var(--muted);
		font-size: 13px;
		cursor: not-allowed;
		opacity: 0.6;
	}

	.update-btn:disabled {
		pointer-events: none;
	}

	.coming-soon {
		font-size: 10px;
		padding: 2px 6px;
		background: var(--border);
		border-radius: 4px;
		margin-left: 4px;
	}

	.version-info {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.version-row {
		display: flex;
		gap: 12px;
		font-size: 13px;
	}

	.version-row .label {
		color: var(--muted);
		min-width: 60px;
	}

	.version-row .value {
		color: var(--text);
		font-family: var(--font-mono);
	}

	.tech-stack {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.tech-badge {
		padding: 4px 10px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 999px;
		font-size: 12px;
		color: var(--text);
	}

	.links {
		display: flex;
		flex-direction: column;
		gap: 6px;
		align-items: flex-start;
	}

	.link {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		background: transparent;
		border: none;
		color: var(--text);
		text-decoration: none;
		font-size: 13px;
		transition: all 0.15s ease;
		border-radius: var(--radius-sm);
	}

	.link:hover {
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 8%, var(--panel-strong));
	}

	.link svg {
		flex-shrink: 0;
		opacity: 0.7;
	}

	.copyright {
		margin-top: 16px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
		font-size: 12px;
		color: var(--muted);
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
