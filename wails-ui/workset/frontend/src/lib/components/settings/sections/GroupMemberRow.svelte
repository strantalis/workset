<script lang="ts">
	import type { GroupMember } from '../../../types';
	import Button from '../../ui/Button.svelte';

	interface Props {
		member: GroupMember;
		loading?: boolean;
		onRemove: () => void;
	}

	const { member, loading = false, onRemove }: Props = $props();
</script>

<div class="member">
	<div class="member-header">
		<span class="member-name">{member.repo}</span>
		<div class="member-actions">
			<Button variant="danger" size="sm" onclick={onRemove} disabled={loading}>Remove</Button>
		</div>
	</div>
</div>

<style>
	.member {
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		background: var(--panel);
		overflow: hidden;
	}

	.member-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		width: 100%;
		padding: 10px var(--space-3);
		border: none;
		background: transparent;
		color: var(--text);
		font-size: var(--text-base);
		text-align: left;
		cursor: pointer;
		transition: background var(--transition-fast);
	}

	.member-header:hover {
		background: color-mix(in srgb, var(--text) 4%, transparent);
	}

	.member-name {
		font-weight: 500;
		flex-shrink: 0;
	}

	.member-actions {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-left: auto;
	}
</style>
