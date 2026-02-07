<script lang="ts">
	import Modal from './Modal.svelte';
	import Alert from './ui/Alert.svelte';
	import Button from './ui/Button.svelte';
	import { onMount } from 'svelte';
	import type { GitHubAuthInfo, GitHubAuthStatus } from '../types';
	import { openFileDialog } from '../api/settings';
	import {
		fetchGitHubAuthInfo,
		setGitHubAuthMode,
		setGitHubCLIPath,
		setGitHubToken,
	} from '../api/github';

	interface Props {
		onClose?: () => void;
		onSuccess?: (status: GitHubAuthStatus | null) => void;
		notice?: string | null;
		cancelLabel?: string;
	}

	const { onClose, onSuccess, notice = null, cancelLabel = 'Cancel' }: Props = $props();

	let mode = $state<'cli' | 'pat'>('cli');
	let patToken = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);
	let statusText = $state<string | null>(null);
	let cliPath = $state('');
	let cliMissing = $state(false);

	const formatError = (err: unknown, fallback: string): string => {
		if (err instanceof Error) return err.message;
		if (typeof err === 'string') return err;
		if (err && typeof err === 'object' && 'message' in err) {
			const message = (err as { message?: string }).message;
			if (typeof message === 'string') return message;
		}
		return fallback;
	};

	const selectMode = (next: 'cli' | 'pat'): void => {
		if (mode === next) return;
		mode = next;
		error = null;
		statusText = null;
		cliMissing = false;
		if (next === 'cli') {
			patToken = '';
		}
	};

	const checkCliStatus = async (): Promise<void> => {
		loading = true;
		error = null;
		statusText = null;
		try {
			const info: GitHubAuthInfo = await setGitHubAuthMode('cli');
			cliPath = info.cli.configuredPath ?? cliPath;
			cliMissing = !info.cli.installed;
			if (!info.cli.installed) {
				error = 'GitHub CLI is not installed. Provide the `gh` binary path to continue.';
				return;
			}
			cliMissing = false;
			if (info.status.authenticated) {
				statusText = info.status.login ? `Connected as ${info.status.login}.` : 'GitHub connected.';
				onSuccess?.(info.status);
				return;
			}
			error = 'GitHub CLI is not authenticated. Run `gh auth login` and try again.';
		} catch (err) {
			error = formatError(err, 'Failed to check GitHub CLI auth.');
		} finally {
			loading = false;
		}
	};

	const saveCLIPath = async (): Promise<void> => {
		const path = cliPath.trim();
		if (!path) {
			error = 'GitHub CLI path required.';
			return;
		}
		loading = true;
		error = null;
		statusText = null;
		try {
			const info = await setGitHubCLIPath(path);
			cliPath = info.cli.configuredPath ?? path;
			if (info.cli.installed) {
				statusText = 'GitHub CLI path saved. Click “Check status” to continue.';
				cliMissing = false;
			} else {
				error = 'GitHub CLI still not detected. Verify the path.';
				cliMissing = true;
			}
		} catch (err) {
			error = formatError(err, 'Failed to save GitHub CLI path.');
		} finally {
			loading = false;
		}
	};

	const browseCLIPath = async (): Promise<void> => {
		if (loading) return;
		error = null;
		try {
			const selected = await openFileDialog('Select GitHub CLI', '');
			if (!selected) return;
			cliPath = selected;
			await saveCLIPath();
		} catch (err) {
			error = formatError(err, 'Failed to open file dialog.');
		}
	};

	const savePat = async (): Promise<void> => {
		const token = patToken.trim();
		if (!token) {
			error = 'Personal access token required.';
			return;
		}
		loading = true;
		error = null;
		statusText = null;
		try {
			const auth = await setGitHubToken(token, 'pat');
			patToken = '';
			statusText = auth.login ? `Connected as ${auth.login}.` : 'GitHub connected.';
			onSuccess?.(auth);
		} catch (err) {
			error = formatError(err, 'Failed to save token.');
		} finally {
			loading = false;
		}
	};

	onMount(() => {
		void (async () => {
			try {
				const info = await fetchGitHubAuthInfo();
				if (info.mode === 'pat') {
					mode = 'pat';
				}
				cliPath = info.cli.configuredPath ?? '';
				cliMissing = !info.cli.installed;
			} catch {
				// ignore
			}
		})();
	});
</script>

<Modal title="Connect GitHub" subtitle="Use GitHub CLI or a personal access token." {onClose}>
	<div class="mode-toggle">
		<button class:active={mode === 'cli'} type="button" onclick={() => selectMode('cli')}>
			GitHub CLI
		</button>
		<button class:active={mode === 'pat'} type="button" onclick={() => selectMode('pat')}>
			Personal token
		</button>
	</div>

	{#if error}
		<Alert variant="error">{error}</Alert>
	{:else if statusText}
		<Alert variant="info">{statusText}</Alert>
	{:else if notice}
		<Alert variant="info">{notice}</Alert>
	{/if}

	{#if mode === 'cli'}
		<div class="code-block">
			<div class="label">Authenticate with GitHub CLI</div>
			<div class="code">gh auth login</div>
			<div class="instructions">
				Workset will use your GitHub CLI session. Run the command in your terminal, then click
				“Check status”.
			</div>
			<div class="actions">
				<Button variant="primary" onclick={checkCliStatus} disabled={loading}>
					{loading ? 'Checking…' : 'Check status'}
				</Button>
			</div>
			{#if cliMissing}
				<div class="pat-block">
					<label class="label" for="modal-cli-path">GitHub CLI path</label>
					<input
						id="modal-cli-path"
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
							disabled={loading || cliPath.trim() === ''}
						>
							{loading ? 'Saving…' : 'Save path'}
						</Button>
						<Button variant="ghost" onclick={browseCLIPath} disabled={loading}>Browse…</Button>
					</div>
				</div>
			{/if}
		</div>
	{:else}
		<div class="pat-block">
			<label class="label" for="pat-token">Personal access token</label>
			<input
				id="pat-token"
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
			<div class="instructions">
				Use a token with access to private repos. Workset stores it in your OS keychain.
			</div>
			<div class="actions">
				<Button variant="primary" onclick={savePat} disabled={loading || patToken.trim() === ''}>
					{loading ? 'Saving…' : 'Save token'}
				</Button>
			</div>
		</div>
	{/if}

	<div class="footer">
		<Button variant="ghost" onclick={onClose}>{cancelLabel}</Button>
	</div>
</Modal>

<style>
	.mode-toggle {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 8px;
		margin-bottom: 12px;
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

	.code-block {
		display: flex;
		flex-direction: column;
		gap: 10px;
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

	.pat-block {
		display: flex;
		flex-direction: column;
		gap: 10px;
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

	.instructions {
		color: var(--text);
		font-size: 13px;
	}

	.actions {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}

	.footer {
		display: flex;
		gap: 8px;
		justify-content: flex-end;
	}
</style>
