<script lang="ts">
	import { onMount } from 'svelte';
	import Alert from '../../ui/Alert.svelte';
	import Button from '../../ui/Button.svelte';
	import SettingsSection from '../SettingsSection.svelte';
	import { toErrorMessage } from '../../../errors';
	import type { GitHubAuthInfo } from '../../../types';
	import {
		disconnectGitHub,
		fetchGitHubAuthInfo,
		openFileDialog,
		setGitHubAuthMode,
		setGitHubCLIPath,
		setGitHubToken,
	} from '../../../api';

	let info = $state<GitHubAuthInfo | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let statusMessage = $state<string | null>(null);
	let patToken = $state('');
	let cliPath = $state('');
	let successPulse = $state(false);

	const authMode = $derived(info?.mode ?? 'cli');
	const authenticated = $derived(info?.status.authenticated ?? false);
	const login = $derived(info?.status.login ?? '');
	const cliInstalled = $derived(info?.cli.installed ?? false);

	const loadInfo = async (): Promise<void> => {
		loading = true;
		error = null;
		statusMessage = null;
		successPulse = false;
		try {
			info = await fetchGitHubAuthInfo();
			cliPath = info?.cli.configuredPath ?? '';
			if (info?.status.authenticated) {
				successPulse = true;
				setTimeout(() => {
					successPulse = false;
				}, 1500);
			}
		} catch (err) {
			error = toErrorMessage(err, 'Failed to load GitHub auth status.');
		} finally {
			loading = false;
		}
	};

	const selectMode = async (next: 'cli' | 'pat'): Promise<void> => {
		if (saving || loading || !info || authMode === next) return;
		saving = true;
		error = null;
		statusMessage = null;
		try {
			info = await setGitHubAuthMode(next);
			statusMessage =
				next === 'cli' ? 'Using GitHub CLI for authentication.' : 'Using a personal access token.';
		} catch (err) {
			error = toErrorMessage(err, 'Failed to update GitHub auth mode.');
		} finally {
			saving = false;
		}
	};

	const savePat = async (): Promise<void> => {
		const token = patToken.trim();
		if (!token) {
			error = 'Personal access token required.';
			return;
		}
		saving = true;
		error = null;
		statusMessage = null;
		try {
			await setGitHubToken(token, 'pat');
			patToken = '';
			info = await fetchGitHubAuthInfo();
			statusMessage = info?.status.login
				? `Connected as ${info.status.login}.`
				: 'GitHub token saved.';
		} catch (err) {
			error = toErrorMessage(err, 'Failed to save token.');
		} finally {
			saving = false;
		}
	};

	const saveCLIPath = async (): Promise<void> => {
		const path = cliPath.trim();
		if (!path) {
			error = 'GitHub CLI path required.';
			return;
		}
		saving = true;
		error = null;
		statusMessage = null;
		try {
			info = await setGitHubCLIPath(path);
			cliPath = info?.cli.configuredPath ?? path;
			statusMessage = info?.cli.installed
				? 'GitHub CLI path saved.'
				: 'Path saved. GitHub CLI still not detected.';
		} catch (err) {
			error = toErrorMessage(err, 'Failed to save GitHub CLI path.');
		} finally {
			saving = false;
		}
	};

	const browseCLIPath = async (): Promise<void> => {
		if (saving) return;
		error = null;
		try {
			const selected = await openFileDialog('Select GitHub CLI', '');
			if (!selected) return;
			cliPath = selected;
			await saveCLIPath();
		} catch (err) {
			error = toErrorMessage(err, 'Failed to open file dialog.');
		}
	};

	const forgetToken = async (): Promise<void> => {
		if (saving) return;
		saving = true;
		error = null;
		statusMessage = null;
		try {
			await disconnectGitHub();
			info = await fetchGitHubAuthInfo();
			statusMessage = 'Token removed.';
		} catch (err) {
			error = toErrorMessage(err, 'Failed to remove token.');
		} finally {
			saving = false;
		}
	};

	onMount(() => {
		void loadInfo();
	});
</script>

<SettingsSection
	title="GitHub authentication"
	description="Choose how Workset authenticates with GitHub. CLI is the default."
>
	{#if loading}
		<div class="state">Loading GitHub status…</div>
	{:else if error && !info}
		<Alert variant="error">{error}</Alert>
	{:else if info}
		<div class="status-card" class:success-pulse={successPulse}>
			{#if error}
				<Alert variant="error">{error}</Alert>
			{/if}
			{#if authenticated}
				<Alert variant="success">
					{#if login}
						Connected as {login} via {authMode === 'cli' ? 'GitHub CLI' : 'personal access token'}.
					{:else}
						GitHub connected via {authMode === 'cli' ? 'GitHub CLI' : 'personal access token'}.
					{/if}
				</Alert>
			{:else}
				<Alert variant="warning">Not connected. Select an auth method below.</Alert>
			{/if}
			<div class="status-meta">
				<div>
					<span class="meta-label">Mode</span>
					<span class="meta-value">{authMode === 'cli' ? 'GitHub CLI' : 'Personal token'}</span>
				</div>
				<div>
					<span class="meta-label">CLI</span>
					<span class="meta-value">
						{#if info.cli.installed}
							Installed{info.cli.version ? ` · v${info.cli.version}` : ''}
						{:else}
							Not installed
						{/if}
					</span>
				</div>
			</div>
			{#if info.cli.error}
				<div class="status-note">CLI status error: {info.cli.error}</div>
			{/if}
			{#if statusMessage}
				<div class="status-note">{statusMessage}</div>
			{/if}
		</div>

		<div class="mode-toggle">
			<button
				class:active={authMode === 'cli'}
				type="button"
				onclick={() => selectMode('cli')}
				disabled={saving || loading}
			>
				GitHub CLI
			</button>
			<button
				class:active={authMode === 'pat'}
				type="button"
				onclick={() => selectMode('pat')}
				disabled={saving || loading}
			>
				Personal token
			</button>
		</div>

		{#if authMode === 'cli'}
			<div class="cli-block">
				<div class="label">Authenticate with GitHub CLI</div>
				<div class="code">gh auth login</div>
				<p class="instructions">Run the command in your terminal, then refresh status here.</p>
				<div class="actions">
					<Button variant="primary" onclick={loadInfo} disabled={saving || loading}>
						{loading ? 'Checking…' : 'Check status'}
					</Button>
				</div>
				{#if !cliInstalled}
					<Alert variant="warning"
						>GitHub CLI is not installed. Provide the `gh` binary path to continue.</Alert
					>
					<div class="cli-path">
						<label class="label" for="settings-cli-path">GitHub CLI path</label>
						<input
							id="settings-cli-path"
							class="pat-input"
							type="text"
							bind:value={cliPath}
							placeholder="/Users/you/.nix-profile/bin/gh"
							spellcheck="false"
							autocomplete="off"
							onkeydown={(event) => {
								if (event.key === 'Enter') void saveCLIPath();
							}}
						/>
						<div class="actions">
							<Button
								variant="primary"
								onclick={saveCLIPath}
								disabled={saving || cliPath.trim() === ''}
							>
								{saving ? 'Saving…' : 'Save path'}
							</Button>
							<Button variant="ghost" onclick={browseCLIPath} disabled={saving}>Browse…</Button>
						</div>
					</div>
				{/if}
			</div>
		{:else}
			<div class="pat-block">
				<label class="label" for="settings-pat-token">Personal access token</label>
				<input
					id="settings-pat-token"
					class="pat-input"
					type="password"
					bind:value={patToken}
					placeholder="ghp_••••••••••••••••"
					spellcheck="false"
					autocomplete="off"
					onkeydown={(event) => {
						if (event.key === 'Enter') void savePat();
					}}
				/>
				<p class="instructions">
					Store a token with access to private repos. Workset keeps it in your OS keychain.
				</p>
				<div class="actions">
					<Button variant="primary" onclick={savePat} disabled={saving || patToken.trim() === ''}>
						{saving ? 'Saving…' : 'Save token'}
					</Button>
					<Button variant="ghost" onclick={forgetToken} disabled={saving}>Forget token</Button>
				</div>
			</div>
		{/if}
	{/if}
</SettingsSection>

<style>
	.state {
		font-size: 13px;
		color: var(--muted);
	}

	.status-card {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 16px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		transition:
			box-shadow var(--transition-fast),
			border-color var(--transition-fast);
	}

	.status-card.success-pulse {
		animation: statusPulse 1.2s ease-out;
		border-color: var(--success-soft);
	}

	@keyframes statusPulse {
		0% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0.5);
		}
		50% {
			box-shadow: 0 0 20px 8px rgba(var(--success-rgb), 0.2);
			border-color: var(--success);
		}
		100% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0);
		}
	}

	.status-meta {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
		gap: 8px;
		font-size: 12px;
		color: var(--muted);
	}

	.meta-label {
		text-transform: uppercase;
		letter-spacing: 0.08em;
		margin-right: 6px;
	}

	.meta-value {
		color: var(--text);
	}

	.status-note {
		font-size: 12px;
		color: var(--muted);
	}

	.mode-toggle {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 8px;
		margin-top: 16px;
	}

	.mode-toggle button {
		border-radius: 10px;
		border: 1px solid var(--border);
		background: rgba(255, 255, 255, 0.02);
		color: var(--muted);
		padding: 8px 12px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.mode-toggle button.active {
		border-color: var(--accent);
		color: var(--text);
		box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.1);
	}

	.mode-toggle button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.cli-block,
	.pat-block {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin-top: 16px;
		padding: 16px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: rgba(255, 255, 255, 0.02);
	}

	.label {
		color: var(--muted);
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
	}

	.code {
		font-size: 18px;
		font-weight: 600;
		letter-spacing: 0.08em;
		font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
		color: var(--accent);
	}

	.instructions {
		color: var(--text);
		font-size: 13px;
		margin: 0;
	}

	.pat-input {
		width: 100%;
		padding: 10px 12px;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: rgba(10, 10, 10, 0.6);
		color: var(--text);
		font-size: 13px;
	}

	.actions {
		display: flex;
		gap: 10px;
		flex-wrap: wrap;
	}
</style>
