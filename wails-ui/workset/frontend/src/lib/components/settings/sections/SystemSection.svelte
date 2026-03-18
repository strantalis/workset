<script lang="ts">
	import { onMount } from 'svelte';
	import type { SettingsDefaultField } from '../../../types';
	import type { AgentCLIStatus } from '../../../types';
	import { toErrorMessage } from '../../../errors';
	import SettingsSection from '../SettingsSection.svelte';
	import Button from '../../ui/Button.svelte';
	import {
		checkAgentStatus,
		openFileDialog,
		reloadLoginEnv,
		setAgentCLIPath,
	} from '../../../api/settings';
	import { RefreshCw } from '@lucide/svelte';

	type FieldId = SettingsDefaultField;

	interface Props {
		draft: Record<FieldId, string>;
		baseline: Record<FieldId, string>;
		onUpdate: (id: FieldId, value: string) => void;
		onRestartSessiond: () => void;
		onResetTerminalLayout: () => void;
		restartingSessiond?: boolean;
		resettingTerminalLayout?: boolean;
	}

	const {
		draft,
		baseline,
		onUpdate,
		onRestartSessiond,
		onResetTerminalLayout,
		restartingSessiond = false,
		resettingTerminalLayout = false,
	}: Props = $props();

	// --- Agent CLI state ---
	let checking = $state(false);
	let savingPath = $state(false);
	let status = $state<AgentCLIStatus | null>(null);
	let statusError = $state<string | null>(null);
	let cliPath = $state('');
	let envReloading = $state(false);
	let envMessage = $state<string | null>(null);
	let envError = $state<string | null>(null);

	// --- Maintenance completion states ---
	let restartCompleted = $state(false);
	let previousRestarting = $state(false);
	let resetCompleted = $state(false);
	let previousResetting = $state(false);

	$effect(() => {
		const isRestarting = restartingSessiond;
		if (previousRestarting && !isRestarting) {
			restartCompleted = true;
			setTimeout(() => {
				restartCompleted = false;
			}, 1500);
		}
		previousRestarting = isRestarting;
	});

	$effect(() => {
		const isResetting = resettingTerminalLayout;
		if (previousResetting && !isResetting) {
			resetCompleted = true;
			setTimeout(() => {
				resetCompleted = false;
			}, 1500);
		}
		previousResetting = isResetting;
	});

	// --- Storage path fields ---
	type Field = {
		id: FieldId;
		label: string;
		description: string;
		placeholder?: string;
	};

	const storageFields: Field[] = [
		{
			id: 'worksetRoot',
			label: 'Workset root',
			description: 'Base path for workset folders and nested thread directories.',
			placeholder: '~/.workset',
		},
		{
			id: 'repoStoreRoot',
			label: 'Repo store root',
			description: 'Local mirror cache for repo cloning.',
			placeholder: '~/.workset/repos',
		},
	];

	const getValue = (id: FieldId): string => draft[id] ?? '';
	const isChanged = (id: FieldId): boolean => draft[id] !== baseline[id];

	const handleInput = (id: FieldId, event: Event): void => {
		const target = event.target as HTMLInputElement | null;
		onUpdate(id, target?.value ?? '');
	};

	// --- Agent CLI handlers ---
	const checkStatus = async (): Promise<void> => {
		if (checking) return;
		statusError = null;
		status = null;
		const agent = getValue('agent').trim();
		if (!agent) {
			statusError = 'Agent command required.';
			return;
		}
		checking = true;
		try {
			status = await checkAgentStatus(agent);
			if (status?.configuredPath) {
				cliPath = status.configuredPath;
			}
		} catch (err) {
			statusError = toErrorMessage(err, 'Failed to check agent status.');
		} finally {
			checking = false;
		}
	};

	const saveCLIPath = async (): Promise<void> => {
		if (savingPath) return;
		const path = cliPath.trim();
		if (!path) {
			statusError = 'Agent CLI path required.';
			return;
		}
		const agent = getValue('agent').trim();
		if (!agent) {
			statusError = 'Agent command required.';
			return;
		}
		savingPath = true;
		statusError = null;
		status = null;
		try {
			status = await setAgentCLIPath(agent, path);
			cliPath = status?.configuredPath ?? path;
		} catch (err) {
			statusError = toErrorMessage(err, 'Failed to save agent CLI path.');
		} finally {
			savingPath = false;
		}
	};

	const browseCLIPath = async (): Promise<void> => {
		if (savingPath) return;
		statusError = null;
		try {
			const selected = await openFileDialog('Select agent CLI', '');
			if (!selected) return;
			cliPath = selected;
			await saveCLIPath();
		} catch (err) {
			statusError = toErrorMessage(err, 'Failed to open file dialog.');
		}
	};

	const reloadEnv = async (): Promise<void> => {
		if (envReloading) return;
		envReloading = true;
		envMessage = null;
		envError = null;
		try {
			const result = await reloadLoginEnv();
			if (result.appliedKeys && result.appliedKeys.length > 0) {
				envMessage = `Reloaded environment (${result.appliedKeys.join(', ')}).`;
			} else if (result.updated) {
				envMessage = 'Reloaded environment.';
			} else {
				envMessage = 'Environment already up to date.';
			}
		} catch (err) {
			envError = toErrorMessage(err, 'Failed to reload environment.');
		} finally {
			envReloading = false;
		}
	};

	onMount(() => {
		void checkStatus();
	});
</script>

<SettingsSection title="System" description="Storage paths, CLI setup, and maintenance actions.">
	<!-- Paths subsection -->
	<div class="subsection">
		<h3 class="subsection-title">Paths</h3>
		<p class="subsection-hint">Where Workset stores data on disk. Changes saved via footer.</p>

		<div class="fields-stack">
			{#each storageFields as field (field.id)}
				<div class="field" class:changed={isChanged(field.id)}>
					<label for={field.id}>{field.label}</label>
					<input
						class="field-input"
						id={field.id}
						type="text"
						placeholder={field.placeholder ?? ''}
						value={getValue(field.id)}
						autocapitalize="off"
						autocorrect="off"
						spellcheck="false"
						oninput={(event) => handleInput(field.id, event)}
					/>
					<p class="ws-hint">{field.description}</p>
				</div>
			{/each}
		</div>

		<!-- Agent CLI path -->
		<div class="cli-section">
			<label class="field-label" for="agent-cli-path">Agent CLI path</label>
			<div class="cli-input-row">
				<input
					id="agent-cli-path"
					class="field-input cli-input"
					type="text"
					bind:value={cliPath}
					placeholder="/Users/you/.local/bin/agent"
					spellcheck="false"
					autocomplete="off"
					onkeydown={(event) => {
						if (event.key === 'Enter') void saveCLIPath();
					}}
				/>
				<Button variant="ghost" size="sm" onclick={browseCLIPath} disabled={savingPath}>
					Browse…
				</Button>
				{#if status?.installed}
					<span class="valid-badge">
						<svg width="12" height="12" viewBox="0 0 12 12" fill="none">
							<path
								d="M2 6L5 9L10 3"
								stroke="currentColor"
								stroke-width="1.5"
								stroke-linecap="round"
								stroke-linejoin="round"
							/>
						</svg>
						Valid
					</span>
				{/if}
			</div>
			{#if status}
				<span class="cli-status" class:ok={status.installed} class:bad={!status.installed}>
					{#if status.installed}
						CLI {status.path ? `· ${status.path}` : 'installed'}
					{:else}
						Not detected
					{/if}
				</span>
			{/if}
			{#if status?.error}
				<div class="note warning">{status.error}</div>
			{/if}
			{#if statusError}
				<div class="note error">{statusError}</div>
			{/if}
		</div>
	</div>

	<!-- Maintenance subsection -->
	<div class="subsection">
		<h3 class="subsection-title">Maintenance</h3>
		<p class="subsection-hint">These actions take effect immediately — no need to save.</p>

		<div class="maintenance-actions">
			<div class="maintenance-action" class:action-completed={restartCompleted}>
				<div class="action-info">
					<span class="action-label">Restart daemon</span>
					<span class="action-hint ws-hint"
						>Restart if terminals get stuck or after changing daemon settings.</span
					>
				</div>
				<button
					class="action-btn"
					type="button"
					onclick={onRestartSessiond}
					disabled={restartingSessiond}
					class:busy={restartingSessiond}
				>
					{#if restartingSessiond}
						<span class="spin-icon">⟳</span> Restarting…
					{:else}
						Restart
					{/if}
				</button>
			</div>

			<div class="maintenance-action" class:action-completed={resetCompleted}>
				<div class="action-info">
					<span class="action-label">Reset terminal layout</span>
					<span class="action-hint ws-hint">
						{#if resetCompleted}
							Terminal layout and sessions reset.
						{:else}
							Reset the layout for the current thread and stop running sessions.
						{/if}
					</span>
				</div>
				<button
					class="action-btn"
					type="button"
					onclick={onResetTerminalLayout}
					disabled={resettingTerminalLayout}
					class:busy={resettingTerminalLayout}
				>
					{#if resettingTerminalLayout}
						Resetting…
					{:else}
						Reset
					{/if}
				</button>
			</div>

			<div class="maintenance-action">
				<div class="action-info">
					<span class="action-label">Reload environment</span>
					<span class="action-hint ws-hint"
						>Reloads PATH and SSH agent variables from your login shell.</span
					>
				</div>
				<button
					class="action-btn"
					type="button"
					onclick={reloadEnv}
					disabled={envReloading}
					class:busy={envReloading}
				>
					{#if envReloading}
						<RefreshCw size={13} class="spin" />
						Reloading…
					{:else}
						Reload
					{/if}
				</button>
			</div>
			{#if envMessage}
				<div class="note ok">{envMessage}</div>
			{/if}
			{#if envError}
				<div class="note error">{envError}</div>
			{/if}
		</div>
	</div>
</SettingsSection>

<style>
	.subsection {
		padding-top: 16px;
	}

	.subsection + .subsection {
		margin-top: 12px;
		padding-top: 20px;
		border-top: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
	}

	.subsection-title {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--text);
		margin: 0 0 2px 0;
		letter-spacing: 0.01em;
	}

	.subsection-hint {
		font-size: var(--text-sm);
		color: var(--muted);
		margin: 0 0 12px 0;
	}

	/* Fields */
	.fields-stack {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.field.changed label::after {
		content: '*';
		color: var(--warning);
		margin-left: 4px;
	}

	.field label,
	.field-label {
		font-size: var(--text-sm);
		font-weight: 500;
		color: var(--muted);
		display: block;
		margin-bottom: 4px;
	}

	.field-input {
		width: 100%;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 10px;
		color: inherit;
		font-size: var(--text-base);
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.field-input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 1px var(--accent);
	}

	/* Agent CLI */
	.cli-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin-top: 16px;
		padding-top: 12px;
		border-top: 1px solid color-mix(in srgb, var(--border) 30%, transparent);
	}

	.cli-input-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.cli-input {
		flex: 1;
	}

	.valid-badge {
		display: flex;
		align-items: center;
		gap: 4px;
		font-size: var(--text-sm);
		font-weight: 500;
		color: rgba(131, 206, 164, 0.9);
	}

	.cli-status {
		font-size: var(--text-sm);
		color: var(--muted);
		word-break: break-all;
	}

	.cli-status.ok {
		color: rgba(131, 206, 164, 0.9);
	}

	.cli-status.bad {
		color: rgba(255, 161, 136, 0.9);
	}

	.note {
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.note.warning {
		color: rgba(255, 200, 122, 0.9);
	}

	.note.ok {
		color: rgba(131, 206, 164, 0.9);
	}

	.note.error {
		color: rgba(255, 140, 140, 0.9);
	}

	/* Maintenance */
	.maintenance-actions {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.maintenance-action {
		display: grid;
		grid-template-columns: 1fr auto;
		align-items: center;
		gap: 12px;
		padding: 10px 14px;
		border-radius: var(--radius-md);
		background: color-mix(in srgb, var(--text) 2%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.maintenance-action.action-completed {
		animation: actionPulse 1.2s ease-out;
		border-color: var(--success-soft);
	}

	.action-info {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}

	.action-label {
		font-size: var(--text-base);
		font-weight: 500;
		color: var(--text);
	}

	.action-hint {
		font-size: var(--text-sm);
	}

	.action-btn {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 6px 14px;
		font-size: var(--text-sm);
		cursor: pointer;
		white-space: nowrap;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		width: auto;
		transition:
			border-color var(--transition-fast),
			color var(--transition-fast),
			transform var(--transition-fast);
	}

	.action-btn:hover:not(:disabled) {
		border-color: var(--accent);
	}

	.action-btn:active:not(:disabled) {
		transform: scale(0.96) translateY(1px);
	}

	.action-btn.busy {
		border-color: var(--warning);
		color: var(--warning);
	}

	.action-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.spin-icon {
		display: inline-block;
		animation: spin 1s linear infinite;
	}

	.action-btn :global(.spin) {
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}

	@keyframes actionPulse {
		0% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0.4);
		}
		50% {
			box-shadow: 0 0 16px 6px rgba(var(--success-rgb), 0.15);
			border-color: var(--success);
		}
		100% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0);
		}
	}

	@media (max-width: 600px) {
		.cli-input-row {
			flex-direction: column;
			align-items: stretch;
		}

		.maintenance-action {
			grid-template-columns: 1fr;
		}
	}
</style>
