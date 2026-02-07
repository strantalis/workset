<script lang="ts">
	import type { HookExecution } from '../../types';
	import Alert from '../ui/Alert.svelte';
	import Button from '../ui/Button.svelte';

	interface PendingHook {
		event: string;
		repo: string;
		hooks: string[];
		status?: string;
		reason?: string;
		running?: boolean;
		runError?: string;
		trusting?: boolean;
		trusted?: boolean;
	}

	interface Props {
		success: string | null;
		warnings: string[];
		hookRuns: HookExecution[];
		pendingHooks: PendingHook[];
		onRunPendingHook: (pending: PendingHook) => void | Promise<void>;
		onTrustPendingHook: (pending: PendingHook) => void | Promise<void>;
		onDone: () => void;
	}

	const {
		success,
		warnings,
		hookRuns,
		pendingHooks,
		onRunPendingHook,
		onTrustPendingHook,
		onDone,
	}: Props = $props();
</script>

<div class="hook-results-container">
	{#if success}
		<Alert variant="success">{success}</Alert>
	{/if}
	{#if warnings.length > 0}
		<Alert variant="warning">
			{#each warnings as warning (warning)}
				<div>{warning}</div>
			{/each}
		</Alert>
	{/if}

	{#if hookRuns.length > 0}
		<div class="hook-results-section">
			<h4 class="hook-results-heading">Hook runs</h4>
			<div class="hook-runs-list">
				{#each hookRuns as run (`${run.repo}:${run.event}:${run.id}`)}
					<div class="hook-run-row">
						<span class="hook-run-repo">{run.repo}</span>
						<code class="hook-run-id">{run.id}</code>
						<span
							class="hook-status-badge"
							class:ok={run.status === 'ok'}
							class:failed={run.status === 'failed'}
							class:running={run.status === 'running'}
							class:skipped={run.status === 'skipped'}
						>
							{run.status}
						</span>
						{#if run.log_path}
							<span class="hook-run-log" title={run.log_path}>log</span>
						{/if}
					</div>
				{/each}
			</div>
		</div>
	{/if}

	{#if pendingHooks.length > 0}
		<div class="hook-results-section">
			<h4 class="hook-results-heading">Pending hooks</h4>
			{#each pendingHooks as pending (`${pending.repo}:${pending.event}`)}
				<div class="pending-hook-card">
					<div class="pending-hook-info">
						<span class="pending-hook-repo">{pending.repo}</span>
						<span class="pending-hook-names">{pending.hooks.join(', ')}</span>
						{#if pending.trusted}
							<span class="hook-status-badge ok">trusted</span>
						{/if}
					</div>
					<div class="pending-hook-actions">
						<Button
							variant="primary"
							size="sm"
							disabled={pending.running || pending.trusted}
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
						<div class="pending-hook-error">{pending.runError}</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}

	<div class="hook-results-footer">
		<Button variant="primary" onclick={onDone}>Done</Button>
	</div>
</div>

<style>
	.hook-results-container {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.hook-results-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.hook-results-heading {
		margin: 0;
		font-size: 13px;
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	.hook-runs-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.hook-run-row {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: 13px;
	}

	.hook-run-repo {
		font-weight: 500;
		color: var(--text);
	}

	.hook-run-id {
		font-size: 12px;
		color: var(--muted);
	}

	.hook-status-badge {
		display: inline-flex;
		align-items: center;
		padding: 2px 8px;
		border-radius: var(--radius-sm);
		font-size: 11px;
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

	.hook-status-badge.skipped {
		background: rgba(255, 255, 255, 0.08);
		color: var(--muted);
	}

	.hook-run-log {
		font-size: 11px;
		color: var(--muted);
		cursor: help;
	}

	.pending-hook-card {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 12px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}

	.pending-hook-info {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.pending-hook-repo {
		font-weight: 500;
		font-size: 14px;
		color: var(--text);
	}

	.pending-hook-names {
		font-size: 12px;
		color: var(--muted);
	}

	.pending-hook-actions {
		display: flex;
		gap: 8px;
	}

	.pending-hook-error {
		color: var(--danger);
		font-size: 12px;
	}

	.hook-results-footer {
		display: flex;
		justify-content: flex-end;
		padding-top: 8px;
		border-top: 1px solid var(--border);
	}
</style>
