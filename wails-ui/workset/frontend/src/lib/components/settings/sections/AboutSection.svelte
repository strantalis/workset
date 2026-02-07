<script lang="ts">
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

	const copyVersionInfo = async (): Promise<void> => {
		const versionText = `Workset ${appVersion?.version}${appVersion?.dirty ? '+dirty' : ''} (${appVersion?.commit || 'unknown'})`;
		try {
			if (navigator.clipboard) {
				await navigator.clipboard.writeText(versionText);
			}
		} catch {
			// Ignore clipboard failures so the settings page remains interactive.
		}
	};
</script>

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
				<button type="button" class="copy-btn" title="Copy version info" onclick={copyVersionInfo}>
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
					<span class="value">{appVersion.version}{appVersion.dirty ? '+dirty' : ''}</span>
				</div>
				{#if appVersion.commit}
					<div class="version-row">
						<span class="label">Commit:</span>
						<span class="value">{appVersion.commit}</span>
					</div>
				{/if}
			</div>
			<div class="updates">
				<div class="update-row">
					<label for="update-channel">Channel</label>
					<select
						id="update-channel"
						class="update-select"
						value={updatePreferences.channel}
						onchange={(event) =>
							onUpdateChannelChange((event.currentTarget as HTMLSelectElement).value)}
					>
						<option value="stable">Stable</option>
						<option value="alpha">Alpha</option>
					</select>
				</div>
				<div class="update-actions">
					<button
						type="button"
						class="update-btn"
						disabled={updateBusy}
						onclick={onCheckForUpdates}
					>
						{updateBusy ? 'Checking...' : 'Check for Updates'}
					</button>
					{#if updateCheck?.status === 'update_available'}
						<button
							type="button"
							class="update-btn primary"
							disabled={updateBusy}
							onclick={onUpdateAndRestart}
						>
							{updateBusy ? 'Preparing...' : 'Update and Restart'}
						</button>
					{/if}
				</div>
				{#if updateCheck}
					<div class="update-note">{updateCheck.message}</div>
				{/if}
				{#if updateState?.phase === 'applying'}
					<div class="update-note">{updateState.message}</div>
				{/if}
				{#if updateError}
					<div class="update-error">{updateError}</div>
				{/if}
			</div>
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

<style>
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

	.updates {
		margin-top: 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.update-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		font-size: 12px;
		color: var(--muted);
	}

	.update-select {
		min-width: 120px;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 4px 8px;
		background: var(--panel);
		color: var(--text);
	}

	.update-actions {
		display: flex;
		gap: 8px;
	}

	.update-btn {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 16px;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		background: var(--panel-strong);
		color: var(--text);
		font-size: 13px;
		cursor: pointer;
		opacity: 1;
		transition: all 0.15s ease;
	}

	.update-btn:hover:not(:disabled) {
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 8%, var(--panel-strong));
	}

	.update-btn.primary {
		border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
		color: var(--accent);
	}

	.update-btn:disabled {
		cursor: not-allowed;
		opacity: 0.6;
	}

	.update-note {
		font-size: 12px;
		color: var(--muted);
	}

	.update-error {
		font-size: 12px;
		color: var(--danger);
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
</style>
