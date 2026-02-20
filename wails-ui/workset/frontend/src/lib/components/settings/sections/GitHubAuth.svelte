<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '../../ui/Button.svelte';
	import SettingsSection from '../SettingsSection.svelte';
	import { toErrorMessage } from '../../../errors';
	import type { GitHubAuthInfo } from '../../../types';
	import { openFileDialog } from '../../../api/settings';
	import {
		disconnectGitHub,
		fetchGitHubAuthInfo,
		setGitHubAuthMode,
		setGitHubCLIPath,
		setGitHubToken,
	} from '../../../api/github';

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
		<div class="message error ws-message ws-message-error">{error}</div>
	{:else if info}
		<!-- Status Card -->
		<div class="status-card" class:success-pulse={successPulse}>
			{#if authenticated && login}
				<div class="connected-status">
					Connected as {login} via {authMode === 'cli' ? 'GitHub CLI' : 'personal access token'}.
				</div>
			{:else if authenticated}
				<div class="connected-status">
					GitHub connected via {authMode === 'cli' ? 'GitHub CLI' : 'personal access token'}.
				</div>
			{:else}
				<div class="not-connected-status">Not connected. Select an auth method below.</div>
			{/if}

			<div class="status-meta">
				<div class="meta-item">
					<span class="meta-label">Mode</span>
					<span class="meta-value">{authMode === 'cli' ? 'GitHub CLI' : 'Personal token'}</span>
				</div>
				<div class="meta-item">
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
		</div>

		{#if error}
			<div class="message error ws-message ws-message-error">{error}</div>
		{/if}

		{#if statusMessage}
			<div class="message success ws-message ws-message-success">{statusMessage}</div>
		{/if}

		<!-- Mode Toggle -->
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
			<div class="auth-block">
				<div class="block-header">Authenticate with GitHub CLI</div>
				<div class="command">gh auth login</div>
				<p class="instructions">Run the command in your terminal, then refresh status here.</p>
				<Button variant="primary" onclick={loadInfo} disabled={saving || loading}>
					{saving || loading ? 'Checking…' : 'Check status'}
				</Button>

				{#if !cliInstalled}
					<div class="cli-not-installed">
						<div class="message warning ws-message ws-message-warning">
							GitHub CLI is not installed. Provide the `gh` binary path to continue.
						</div>
						<div class="cli-path-input">
							<label class="input-label" for="settings-cli-path">GitHub CLI path</label>
							<input
								id="settings-cli-path"
								type="text"
								bind:value={cliPath}
								placeholder="/Users/you/.nix-profile/bin/gh"
								spellcheck="false"
								autocomplete="off"
								onkeydown={(event) => {
									if (event.key === 'Enter') void saveCLIPath();
								}}
							/>
							<div class="input-actions">
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
					</div>
				{/if}
			</div>
		{:else}
			<div class="auth-block">
				<div class="input-group">
					<label class="input-label" for="settings-pat-token">Personal access token</label>
					<input
						id="settings-pat-token"
						type="password"
						bind:value={patToken}
						placeholder="ghp_••••••••••••••••"
						spellcheck="false"
						autocomplete="off"
						onkeydown={(event) => {
							if (event.key === 'Enter') void savePat();
						}}
					/>
				</div>
				<p class="instructions">
					Store a token with access to private repos. Workset keeps it in your OS keychain.
				</p>
				<div class="block-actions">
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
		font-size: var(--text-base);
		color: var(--muted);
	}

	.status-card {
		display: flex;
		flex-direction: column;
		gap: 12px;
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

	.connected-status {
		font-size: var(--text-md);
		color: var(--success);
		font-weight: 500;
	}

	.not-connected-status {
		font-size: var(--text-md);
		color: var(--warning);
		font-weight: 500;
	}

	.status-meta {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 16px;
		font-size: var(--text-sm);
	}

	.meta-item {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.meta-label {
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
		font-size: var(--text-xs);
		font-weight: 600;
	}

	.meta-value {
		color: var(--text);
		font-size: var(--text-sm);
	}

	.message.success {
		background: var(--success-subtle);
	}

	.message.warning {
		background: var(--warning-subtle);
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
		background: color-mix(in srgb, var(--text) 2%, transparent);
		color: var(--muted);
		padding: 8px 12px;
		font-size: var(--text-sm);
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.mode-toggle button.active {
		border-color: var(--accent);
		color: white;
		background: var(--accent);
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--text) 10%, transparent);
	}

	.mode-toggle button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.auth-block {
		display: flex;
		flex-direction: column;
		gap: 12px;
		margin-top: 16px;
		padding: 20px;
		border-radius: 12px;
		border: 1px solid var(--border-strong);
		background: var(--panel-strong);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
	}

	.block-header {
		font-size: var(--text-xs);
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.12em;
		color: var(--muted);
		margin-bottom: 4px;
	}

	.command {
		font-size: var(--text-3xl);
		font-weight: 700;
		letter-spacing: 0.02em;
		font-family: var(--font-mono);
		color: var(--accent);
		margin: 4px 0;
	}

	.instructions {
		color: var(--text);
		font-size: var(--text-base);
		margin: 0 0 8px 0;
		line-height: 1.5;
	}

	.input-label {
		display: block;
		color: var(--muted);
		font-size: var(--text-sm);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		margin-bottom: 6px;
	}

	input[type='text'],
	input[type='password'] {
		width: 100%;
		padding: 10px 12px;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		color: var(--text);
		font-size: var(--text-mono-base);
		font-family: var(--font-mono);
		transition: border-color var(--transition-fast);
	}

	input[type='text']:focus,
	input[type='password']:focus {
		outline: none;
		border-color: var(--accent);
	}

	.input-group {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.block-actions,
	.input-actions {
		display: flex;
		gap: 10px;
		flex-wrap: wrap;
	}

	.cli-not-installed {
		display: flex;
		flex-direction: column;
		gap: 12px;
		margin-top: 12px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}

	.cli-path-input {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
</style>
