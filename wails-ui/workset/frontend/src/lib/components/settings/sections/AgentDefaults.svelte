<script lang="ts">
	import { onMount } from 'svelte';
	import type { SettingsDefaults } from '../../../types';
	import type { AgentCLIStatus } from '../../../types';
	import SettingsSection from '../SettingsSection.svelte';
	import Select from '../../ui/Select.svelte';
	import Button from '../../ui/Button.svelte';
	import { checkAgentStatus, openFileDialog, reloadLoginEnv, setAgentCLIPath } from '../../../api';

	type FieldId = keyof SettingsDefaults;

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
			label: 'Preferred agent',
			description:
				'Used for PR title/description generation and commit messages; also the default coding agent for the terminal launcher.',
			type: 'select',
			options: [
				{ label: 'Codex', value: 'codex' },
				{ label: 'Claude Code', value: 'claude' },
				{ label: 'OpenCode', value: 'opencode' },
				{ label: 'Pi', value: 'pi' },
				{ label: 'Cursor Agent', value: 'cursor' },
			],
		},
		{
			id: 'agentModel',
			label: 'Agent model override',
			description:
				'Optional model for PR and commit message generation only. Leave blank to use the agent default.',
			type: 'text',
		},
		{
			id: 'agentLaunch',
			label: 'Agent launch mode',
			description:
				'Auto uses a shell and PTY fallback. Strict requires an agent path with directory separators.',
			type: 'select',
			options: [
				{ label: 'Auto', value: 'auto' },
				{ label: 'Strict', value: 'strict' },
			],
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

	const formatError = (err: unknown, fallback: string): string => {
		if (err instanceof Error) return err.message;
		if (typeof err === 'string') return err;
		if (err && typeof err === 'object' && 'message' in err) {
			const message = (err as { message?: string }).message;
			if (typeof message === 'string') return message;
		}
		return fallback;
	};

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
			statusError = formatError(err, 'Failed to check agent status.');
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
			statusError = formatError(err, 'Failed to save agent CLI path.');
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
			statusError = formatError(err, 'Failed to open file dialog.');
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
			envError = formatError(err, 'Failed to reload environment.');
		} finally {
			envReloading = false;
		}
	};

	onMount(() => {
		void checkStatus();
	});
</script>

<SettingsSection
	title="Agent defaults"
	description="Choose which assistant Workset uses for generation tasks."
>
	<div class="fields">
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
						placeholder="gpt-5.1-codex-mini"
						spellcheck="false"
						autocomplete="off"
						oninput={(event) => {
							const target = event.currentTarget as HTMLInputElement;
							onUpdate(field.id, target.value);
						}}
					/>
				{/if}
				{#if field.id === 'agent'}
					<div class="agent-path">
						<label class="agent-label" for="agent-cli-path">Agent CLI path</label>
						<input
							id="agent-cli-path"
							class="agent-input"
							type="text"
							bind:value={cliPath}
							placeholder="/Users/you/.local/bin/agent"
							spellcheck="false"
							autocomplete="off"
							onkeydown={(event) => {
								if (event.key === 'Enter') void saveCLIPath();
							}}
						/>
						<div class="agent-actions">
							<Button
								variant="primary"
								size="sm"
								onclick={saveCLIPath}
								disabled={savingPath || cliPath.trim() === ''}
							>
								{savingPath ? 'Saving…' : 'Save path'}
							</Button>
							<Button variant="ghost" size="sm" onclick={browseCLIPath} disabled={savingPath}>
								Browse…
							</Button>
							<Button variant="ghost" size="sm" onclick={checkStatus} disabled={checking}>
								{checking ? 'Checking…' : 'Check status'}
							</Button>
						</div>
						{#if status}
							<span class="agent-status" class:ok={status.installed} class:bad={!status.installed}>
								{#if status.installed}
									Found{status.path ? ` · ${status.path}` : ''}
								{:else}
									Not detected
								{/if}
							</span>
						{/if}
						{#if status?.error}
							<div class="agent-note warning">{status.error}</div>
						{/if}
						{#if statusError}
							<div class="agent-note error">{statusError}</div>
						{/if}
						<div class="agent-env">
							<div class="agent-actions">
								<Button variant="ghost" size="sm" onclick={reloadEnv} disabled={envReloading}>
									{envReloading ? 'Reloading…' : 'Reload environment'}
								</Button>
							</div>
							<div class="agent-note">
								Reloads PATH and SSH agent variables from your login shell without restarting the
								app.
							</div>
							{#if envMessage}
								<div class="agent-note ok">{envMessage}</div>
							{/if}
							{#if envError}
								<div class="agent-note error">{envError}</div>
							{/if}
						</div>
					</div>
				{/if}
				<p>{field.description}</p>
			</div>
		{/each}
	</div>
</SettingsSection>

<style>
	.fields {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
		gap: 16px;
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
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: rgba(255, 255, 255, 0.7);
	}

	.field p {
		margin: 0;
		font-size: 12px;
		color: var(--muted);
	}

	.agent-path {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.agent-label {
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: rgba(255, 255, 255, 0.7);
	}

	.agent-input {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 10px;
		color: inherit;
		font-size: 13px;
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.agent-input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 1px var(--accent);
	}

	.agent-actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
	}

	.agent-env {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.agent-status {
		font-size: 12px;
		color: var(--muted);
		word-break: break-all;
	}

	.agent-status.ok {
		color: rgba(131, 206, 164, 0.9);
	}

	.agent-status.bad {
		color: rgba(255, 161, 136, 0.9);
	}

	.agent-note {
		font-size: 12px;
		color: var(--muted);
	}

	.agent-note.warning {
		color: rgba(255, 200, 122, 0.9);
	}

	.agent-note.ok {
		color: rgba(131, 206, 164, 0.9);
	}

	.agent-note.error {
		color: rgba(255, 140, 140, 0.9);
	}
</style>
