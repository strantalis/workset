<script lang="ts">
	import { onMount } from 'svelte';
	import type { SettingsDefaultField } from '../../../types';
	import type { AgentCLIStatus } from '../../../types';
	import { toErrorMessage } from '../../../errors';
	import SettingsSection from '../SettingsSection.svelte';
	import Select from '../../ui/Select.svelte';
	import Button from '../../ui/Button.svelte';
	import {
		checkAgentStatus,
		openFileDialog,
		reloadLoginEnv,
		setAgentCLIPath,
	} from '../../../api/settings';

	type FieldId = SettingsDefaultField;

	interface Props {
		draft: Record<FieldId, string>;
		baseline: Record<FieldId, string>;
		onUpdate: (id: FieldId, value: string) => void;
	}

	const { draft, baseline, onUpdate }: Props = $props();

	type Field = {
		id: FieldId;
		label: string;
		description: string;
		type?: 'text' | 'select';
		options?: { label: string; value: string }[];
	};

	const fields: Field[] = [
		{
			id: 'agent',
			label: 'PREFERRED AGENT',
			description:
				'Used for PR title/description generation and commit messages; also the default coding agent for the terminal launcher.',
			type: 'select',
			options: [
				{ label: 'Codex (Default)', value: 'codex' },
				{ label: 'Claude Code', value: 'claude' },
			],
		},
		{
			id: 'agentModel',
			label: 'AGENT MODEL OVERRIDE',
			description:
				'Optional model for PR and commit message generation only. Leave blank to use the agent default.',
			type: 'text',
		},
	];

	const getValue = (id: FieldId): string => draft[id] ?? '';

	const isChanged = (id: FieldId): boolean => draft[id] !== baseline[id];

	let checking = $state(false);
	let savingPath = $state(false);
	let status = $state<AgentCLIStatus | null>(null);
	let statusError = $state<string | null>(null);
	let cliPath = $state('');
	let envReloading = $state(false);
	let envMessage = $state<string | null>(null);
	let envError = $state<string | null>(null);

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

<SettingsSection
	title="Agent Configuration"
	description="Choose which assistant Workset uses for generation tasks."
>
	<div class="fields-row">
		{#each fields as field (field.id)}
			<div class="field" class:changed={isChanged(field.id)}>
				<label for={field.id}>{field.label}</label>
				{#if field.type === 'select'}
					<Select
						id={field.id}
						value={getValue(field.id)}
						options={field.options ?? []}
						onchange={(val) => onUpdate(field.id, val)}
					/>
				{:else if field.type === 'text'}
					<input
						id={field.id}
						class="agent-input"
						type="text"
						value={getValue(field.id)}
						placeholder="e.g. gpt-4-0125-preview"
						spellcheck="false"
						autocomplete="off"
						oninput={(event) => {
							const target = event.currentTarget as HTMLInputElement;
							onUpdate(field.id, target.value);
						}}
					/>
				{/if}
				<p>{field.description}</p>
			</div>
		{/each}
	</div>

	<div class="cli-section">
		<label class="cli-label" for="agent-cli-path">AGENT CLI PATH</label>
		<div class="cli-input-row">
			<input
				id="agent-cli-path"
				class="agent-input cli-input"
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
			<div class="cli-note warning">{status.error}</div>
		{/if}
		{#if statusError}
			<div class="cli-note error">{statusError}</div>
		{/if}
	</div>

	<div class="reload-card">
		<div class="reload-header">
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" class="reload-icon">
				<path
					d="M8 2V5M8 11V14M2 8H5M11 8H14"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
				/>
			</svg>
			<span class="reload-title">Reload Environment</span>
		</div>
		<p class="reload-description">
			Reloads PATH and SSH agent variables from your login shell without restarting the app.
		</p>
		<div class="reload-actions">
			<Button variant="ghost" size="sm" onclick={reloadEnv} disabled={envReloading}>
				{envReloading ? 'Reloading…' : 'Reload Env'}
			</Button>
		</div>
		{#if envMessage}
			<div class="cli-note ok">{envMessage}</div>
		{/if}
		{#if envError}
			<div class="cli-note error">{envError}</div>
		{/if}
	</div>
</SettingsSection>

<style>
	.fields-row {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 16px;
		margin-bottom: 24px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.field.changed label::after {
		content: '*';
		color: var(--warning);
		margin-left: 4px;
	}

	.field label {
		font-size: var(--text-xs);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
	}

	.field p {
		margin: 0;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.cli-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin-bottom: 24px;
	}

	.cli-label {
		font-size: var(--text-xs);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
	}

	.cli-input-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.cli-input {
		flex: 1;
	}

	.agent-input {
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

	.agent-input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 1px var(--accent);
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

	.cli-note {
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.cli-note.warning {
		color: rgba(255, 200, 122, 0.9);
	}

	.cli-note.ok {
		color: rgba(131, 206, 164, 0.9);
	}

	.cli-note.error {
		color: rgba(255, 140, 140, 0.9);
	}

	.reload-card {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.reload-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.reload-icon {
		color: var(--text);
		opacity: 0.8;
	}

	.reload-title {
		font-size: var(--text-md);
		font-weight: 500;
		color: var(--text);
	}

	.reload-description {
		margin: 0;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.reload-actions {
		margin-top: 8px;
	}
</style>
