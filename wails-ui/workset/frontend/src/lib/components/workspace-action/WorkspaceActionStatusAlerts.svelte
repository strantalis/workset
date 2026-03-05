<script lang="ts">
	import type { HookExecution } from '../../types';
	import type { WorkspaceActionPendingHook } from '../../services/workspaceActionHooks';
	import Alert from '../ui/Alert.svelte';
	import Button from '../ui/Button.svelte';

	interface Props {
		error: string | null;
		success: string | null;
		warnings: string[];
		hookRuns: HookExecution[];
		pendingHooks: WorkspaceActionPendingHook[];
		showMessages?: boolean;
		showHooks?: boolean;
		onRunPendingHook: (pending: WorkspaceActionPendingHook) => Promise<void> | void;
		onTrustPendingHook: (pending: WorkspaceActionPendingHook) => Promise<void> | void;
	}

	const {
		error,
		success,
		warnings,
		hookRuns,
		pendingHooks,
		showMessages = true,
		showHooks = true,
		onRunPendingHook,
		onTrustPendingHook,
	}: Props = $props();

	const hookRunDotClass = (status: HookExecution['status']): string | null => {
		if (status === 'ok') return 'ws-dot-clean';
		if (status === 'failed') return 'ws-dot-error';
		if (status === 'running') return 'ws-dot-ahead';
		return null;
	};
</script>

{#if showMessages && error}
	<Alert variant="error">{error}</Alert>
{/if}
{#if showMessages && success}
	<Alert variant="success">{success}</Alert>
{/if}
{#if showMessages && warnings.length > 0}
	<Alert variant="warning">
		{#each warnings as warning (warning)}
			<div>{warning}</div>
		{/each}
	</Alert>
{/if}
{#if showHooks && hookRuns.length > 0}
	<Alert variant="info">
		<div class="hook-runs-section">
			<div class="hook-runs-heading">Hook runs</div>
			<div class="hook-runs-list">
				{#each hookRuns as run (`${run.repo}:${run.event}:${run.id}`)}
					<div class="hook-run-row">
						<span class="hook-run-repo">{run.repo}</span>
						<code class="hook-run-id ui-kbd">{run.id}</code>
						<span
							class="hook-status-badge"
							class:ok={run.status === 'ok'}
							class:failed={run.status === 'failed'}
							class:running={run.status === 'running'}
							class:skipped={run.status === 'skipped'}
						>
							{#if hookRunDotClass(run.status)}
								<span class={`ws-dot ws-dot-sm ${hookRunDotClass(run.status)}`} aria-hidden="true"
								></span>
							{/if}
							{run.status}
						</span>
						{#if run.log_path}
							<span class="hook-run-log" title={run.log_path}>log</span>
						{/if}
					</div>
				{/each}
			</div>
		</div>
	</Alert>
{/if}
{#if showHooks && pendingHooks.length > 0}
	<Alert variant="warning">
		{#each pendingHooks as pending (`${pending.repo}:${pending.event}`)}
			<div class="pending-hook-row">
				<div>
					{pending.repo} pending hooks: {pending.hooks.join(', ')}
					{#if pending.trusted}
						(trusted)
					{/if}
				</div>
				<div class="ws-pending-hook-actions">
					<Button
						variant="ghost"
						size="sm"
						disabled={pending.running}
						onclick={() => void onRunPendingHook(pending)}
					>
						{pending.running ? 'Running…' : 'Run now'}
					</Button>
					<Button
						variant="ghost"
						size="sm"
						disabled={pending.trusting || pending.trusted}
						onclick={() => void onTrustPendingHook(pending)}
					>
						{pending.trusting ? 'Trusting…' : pending.trusted ? 'Trusted' : 'Trust'}
					</Button>
				</div>
				{#if pending.runError}
					<div class="ws-pending-hook-error">{pending.runError}</div>
				{/if}
			</div>
		{/each}
	</Alert>
{/if}

<style>
	.pending-hook-row {
		display: grid;
		gap: 6px;
		margin-bottom: 10px;
	}

	.hook-runs-section {
		display: grid;
		gap: 8px;
	}

	.hook-runs-heading {
		font-weight: 600;
		font-size: var(--text-sm);
		letter-spacing: 0.02em;
		text-transform: uppercase;
		opacity: 0.9;
	}

	.hook-runs-list {
		display: grid;
		gap: 6px;
	}

	.hook-run-row {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: var(--text-base);
	}

	.hook-run-repo {
		font-weight: 500;
		color: var(--text);
	}

	.hook-run-id {
		color: var(--muted);
	}

	.hook-status-badge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 2px 8px;
		border-radius: var(--radius-sm);
		font-size: var(--text-xs);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		margin-left: auto;
	}

	.hook-status-badge.ok {
		background: rgba(74, 222, 128, 0.15);
		color: var(--success, #4ade80);
	}

	.hook-status-badge.failed {
		background: rgba(239, 68, 68, 0.15);
		color: var(--danger, #ef4444);
	}

	.hook-status-badge.running {
		background: rgba(59, 130, 246, 0.15);
		color: var(--accent);
	}

	.hook-status-badge.running :global(.ws-dot) {
		animation: hook-run-pulse 0.8s ease-in-out infinite;
	}

	.hook-status-badge.skipped {
		background: rgba(255, 255, 255, 0.08);
		color: var(--muted);
	}

	.hook-run-log {
		font-size: var(--text-xs);
		color: var(--muted);
		cursor: help;
	}

	@keyframes hook-run-pulse {
		0%,
		100% {
			transform: scale(1);
			opacity: 1;
		}
		50% {
			transform: scale(1.35);
			opacity: 0.65;
		}
	}
</style>
