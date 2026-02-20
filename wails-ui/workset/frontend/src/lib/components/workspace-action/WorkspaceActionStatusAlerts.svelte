<script lang="ts">
	import type { WorkspaceActionPendingHook } from '../../services/workspaceActionHooks';
	import Alert from '../ui/Alert.svelte';
	import Button from '../ui/Button.svelte';

	interface Props {
		error: string | null;
		success: string | null;
		warnings: string[];
		pendingHooks: WorkspaceActionPendingHook[];
		onRunPendingHook: (pending: WorkspaceActionPendingHook) => Promise<void> | void;
		onTrustPendingHook: (pending: WorkspaceActionPendingHook) => Promise<void> | void;
	}

	const { error, success, warnings, pendingHooks, onRunPendingHook, onTrustPendingHook }: Props =
		$props();
</script>

{#if error}
	<Alert variant="error">{error}</Alert>
{/if}
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
{#if pendingHooks.length > 0}
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
</style>
