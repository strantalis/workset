<script lang="ts">
	import { ExternalLink } from '@lucide/svelte';
	import type {
		AppVersion,
		UpdateCheckResult,
		UpdatePreferences,
		UpdateState,
	} from '../../../types';

	interface Props {
		appVersion: AppVersion | null;
		updatePreferences: UpdatePreferences;
		updateState: UpdateState | null;
		updateCheck: UpdateCheckResult | null;
		updateBusy: boolean;
		updateError: string | null;
		onUpdateChannelChange: (channel: string) => void;
		onCheckForUpdates: () => void;
		onUpdateAndRestart: () => void;
	}

	const {
		appVersion,
		updatePreferences,
		updateState,
		updateCheck,
		updateBusy,
		updateError,
		onUpdateChannelChange,
		onCheckForUpdates,
		onUpdateAndRestart,
	}: Props = $props();
	const COMMIT_DISPLAY_LENGTH = 12;

	const shortCommit = (commit: string): string =>
		commit.length > COMMIT_DISPLAY_LENGTH ? commit.slice(0, COMMIT_DISPLAY_LENGTH) : commit;

	const copyVersionInfo = async (): Promise<void> => {
		const versionText = `Workset ${appVersion?.version}${appVersion?.dirty ? '+dirty' : ''} (${appVersion?.commit || 'unknown'})`;
		try {
			if (navigator.clipboard) {
				await navigator.clipboard.writeText(versionText);
			}
		} catch {
			// Ignore clipboard failures
		}
	};

	let commitCopied = $state(false);

	const copyCommit = async (): Promise<void> => {
		if (!appVersion?.commit) return;
		try {
			await navigator.clipboard.writeText(appVersion.commit);
			commitCopied = true;
			setTimeout(() => {
				commitCopied = false;
			}, 1500);
		} catch {
			// Ignore clipboard failures
		}
	};

	const handleChannelChange = (event: Event): void => {
		const target = event.currentTarget as HTMLSelectElement;
		onUpdateChannelChange(target.value);
	};
</script>

<div class="about-container">
	<!-- Logo and Title -->
	<div class="about-header">
		<img src="images/logo.png" alt="Workset" class="about-logo" />
		<h1 class="app-title">Workset</h1>
		<p class="tagline">Workspace management for multi-repo development</p>
	</div>

	<!-- Version Card -->
	{#if appVersion}
		<div class="version-card">
			<div class="version-row">
				<span class="row-label">Version</span>
				<div class="row-value-with-action">
					<span class="row-value">{appVersion.version}{appVersion.dirty ? '+dirty' : ''}</span>
					<button
						type="button"
						class="copy-btn"
						title="Copy version info"
						onclick={copyVersionInfo}
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
			</div>
			{#if appVersion.commit}
				<div class="version-row">
					<span class="row-label">Commit</span>
					<div class="row-value-with-action">
						<span class="row-value commit-hash" title={appVersion.commit}
							>{shortCommit(appVersion.commit)}</span
						>
						<button
							type="button"
							class="copy-btn"
							class:copied={commitCopied}
							title={commitCopied ? 'Copied!' : 'Copy commit SHA'}
							onclick={copyCommit}
						>
							{#if commitCopied}
								<svg
									width="14"
									height="14"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
								>
									<polyline points="20 6 9 17 4 12" />
								</svg>
							{:else}
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
							{/if}
						</button>
					</div>
				</div>
			{/if}
			<div class="version-row">
				<span class="row-label">Channel</span>
				<select
					class="channel-select"
					value={updatePreferences.channel}
					onchange={handleChannelChange}
				>
					<option value="stable">Stable</option>
					<option value="alpha">Alpha</option>
				</select>
			</div>
		</div>
	{/if}

	<!-- Action Links -->
	<div class="action-links">
		<button type="button" class="link-btn" onclick={onCheckForUpdates} disabled={updateBusy}>
			{updateBusy ? 'Checking...' : 'Check for Updates'}
		</button>
		<span class="divider">|</span>
		<a
			href="https://github.com/anomalyco/workset"
			target="_blank"
			rel="noopener noreferrer"
			class="link-btn with-icon"
		>
			GitHub Repository
			<ExternalLink size={12} />
		</a>
		<span class="divider">|</span>
		<a
			href="https://github.com/anomalyco/workset/issues"
			target="_blank"
			rel="noopener noreferrer"
			class="link-btn with-icon"
		>
			Report an Issue
			<ExternalLink size={12} />
		</a>
	</div>

	<!-- Update Actions -->
	{#if updateCheck?.status === 'update_available'}
		<div class="update-banner">
			<span class="update-message">{updateCheck.message}</span>
			<button
				type="button"
				class="update-action-btn"
				onclick={onUpdateAndRestart}
				disabled={updateBusy}
			>
				{updateBusy ? 'Preparing...' : 'Update and Restart'}
			</button>
		</div>
	{/if}

	{#if updateState?.phase === 'applying'}
		<div class="update-banner info">
			<span class="update-message">{updateState.message}</span>
		</div>
	{/if}

	{#if updateError}
		<div class="update-banner error">
			<span class="update-message">{updateError}</span>
		</div>
	{/if}

	<!-- Copyright -->
	<p class="copyright">
		© {new Date().getFullYear()} Sean Trantalis. Open source under MIT License.
	</p>
</div>

<style>
	.about-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 48px var(--space-4);
		gap: var(--space-6);
		min-height: 100%;
	}

	.about-header {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-3);
	}

	.about-logo {
		width: 64px;
		height: 64px;
		margin-bottom: var(--space-2);
		opacity: 0.9;
	}

	.app-title {
		font-size: var(--text-3xl);
		font-weight: 700;
		color: var(--text);
		margin: 0;
		letter-spacing: -0.02em;
	}

	.tagline {
		font-size: var(--text-md);
		color: var(--muted);
		margin: 0;
	}

	.version-card {
		width: 100%;
		max-width: 400px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: var(--space-4);
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.version-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: var(--space-2) 0;
		border-bottom: 1px solid var(--border);
	}

	.version-row:last-child {
		border-bottom: none;
	}

	.row-label {
		font-size: var(--text-md);
		color: var(--muted);
	}

	.row-value {
		font-size: var(--text-mono-md);
		font-family: var(--font-mono);
		color: var(--text);
	}

	.commit-hash {
		max-width: 12ch;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.row-value-with-action {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.copy-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.copy-btn:hover {
		border-color: var(--accent);
		color: var(--accent);
		background: var(--accent-soft);
	}

	.copy-btn.copied {
		border-color: var(--success, var(--accent));
		color: var(--success, var(--accent));
		background: var(--success-soft, var(--accent-soft));
	}

	.channel-select {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 4px 8px;
		color: var(--text);
		font-size: var(--text-base);
		cursor: pointer;
	}

	.channel-select:focus {
		outline: none;
		border-color: var(--accent);
	}

	.action-links {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		flex-wrap: wrap;
		justify-content: center;
	}

	.link-btn {
		background: transparent;
		border: none;
		color: var(--muted);
		font-size: var(--text-sm);
		cursor: pointer;
		transition: color var(--transition-fast);
		text-decoration: none;
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.link-btn:hover {
		color: var(--accent);
	}

	.link-btn.with-icon {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.divider {
		color: var(--border);
		font-size: var(--text-sm);
	}

	.update-banner {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		background: var(--accent-soft);
		border: 1px solid var(--accent-soft);
		border-radius: var(--radius-md);
		max-width: 400px;
		width: 100%;
	}

	.update-banner.error {
		background: var(--danger-subtle);
		border-color: var(--danger-soft);
	}

	.update-banner.info {
		background: var(--panel-strong);
		border-color: var(--border);
	}

	.update-message {
		font-size: var(--text-base);
		color: var(--text);
		flex: 1;
	}

	.update-action-btn {
		padding: 6px 12px;
		background: var(--accent);
		border: none;
		border-radius: var(--radius-md);
		color: white;
		font-size: var(--text-sm);
		font-weight: 500;
		cursor: pointer;
		transition: all var(--transition-fast);
		white-space: nowrap;
	}

	.update-action-btn:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 90%, black);
	}

	.update-action-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.copyright {
		font-size: var(--text-xs);
		color: var(--subtle);
		margin: 0;
		margin-top: var(--space-4);
	}
</style>
