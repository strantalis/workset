<script lang="ts">
	import { onMount } from 'svelte';
	import {
		addGroupMember,
		createGroup,
		deleteGroup,
		getGroup,
		listAliases,
		listGroups,
		removeGroupMember,
		updateGroup,
	} from '../../../api';
	import type { Alias, Group, GroupSummary } from '../../../types';
	import SettingsSection from '../SettingsSection.svelte';
	import GroupMemberRow from './GroupMemberRow.svelte';
	import Button from '../../ui/Button.svelte';

	interface Props {
		onGroupCountChange: (count: number) => void;
	}

	const { onGroupCountChange }: Props = $props();

	let groups: GroupSummary[] = $state([]);
	let aliases: Alias[] = $state([]);
	let selectedGroup: Group | null = $state(null);
	let isNew = $state(false);
	let loading = $state(false);
	let error: string | null = $state(null);
	let success: string | null = $state(null);

	let formName = $state('');
	let formDescription = $state('');

	let addingMember = $state(false);
	let memberRepo = $state('');

	const formatError = (err: unknown): string => {
		if (err instanceof Error) return err.message;
		return 'An error occurred.';
	};

	const loadGroups = async (): Promise<void> => {
		try {
			groups = await listGroups();
			aliases = await listAliases();
			onGroupCountChange(groups.length);
		} catch (err) {
			error = formatError(err);
		}
	};

	const selectGroup = async (summary: GroupSummary): Promise<void> => {
		try {
			selectedGroup = await getGroup(summary.name);
			isNew = false;
			formName = selectedGroup.name;
			formDescription = selectedGroup.description ?? '';
			addingMember = false;
			error = null;
			success = null;
		} catch (err) {
			error = formatError(err);
		}
	};

	const startNew = (): void => {
		selectedGroup = null;
		isNew = true;
		formName = '';
		formDescription = '';
		addingMember = false;
		error = null;
		success = null;
	};

	const cancelEdit = (): void => {
		if (groups.length > 0) {
			void selectGroup(groups[0]);
		} else {
			selectedGroup = null;
			isNew = false;
			formName = '';
			formDescription = '';
		}
		error = null;
		success = null;
	};

	const handleSave = async (): Promise<void> => {
		const name = formName.trim();
		const description = formDescription.trim();

		if (!name) {
			error = 'Group name is required.';
			return;
		}

		loading = true;
		error = null;
		success = null;

		try {
			if (isNew) {
				await createGroup(name, description);
				success = `Created ${name}.`;
			} else {
				await updateGroup(name, description);
				success = `Updated ${name}.`;
			}
			await loadGroups();
			const summary = groups.find((g) => g.name === name);
			if (summary) {
				await selectGroup(summary);
			}
		} catch (err) {
			error = formatError(err);
		} finally {
			loading = false;
		}
	};

	const handleDelete = async (): Promise<void> => {
		if (!selectedGroup) return;

		const name = selectedGroup.name;
		loading = true;
		error = null;
		success = null;

		try {
			await deleteGroup(name);
			success = `Deleted ${name}.`;
			await loadGroups();
			if (groups.length > 0) {
				await selectGroup(groups[0]);
			} else {
				selectedGroup = null;
				isNew = false;
				formName = '';
				formDescription = '';
			}
		} catch (err) {
			error = formatError(err);
		} finally {
			loading = false;
		}
	};

	const startAddMember = (): void => {
		addingMember = true;
		memberRepo = '';
	};

	const cancelAddMember = (): void => {
		addingMember = false;
	};

	const handleRepoInput = (event: Event): void => {
		const target = event.target as HTMLInputElement | null;
		const value = target?.value ?? '';
		memberRepo = value;
	};

	const handleAddMember = async (): Promise<void> => {
		if (!selectedGroup) return;

		const repo = memberRepo.trim();
		if (!repo) {
			error = 'Repo name is required.';
			return;
		}

		loading = true;
		error = null;
		success = null;

		try {
			await addGroupMember(selectedGroup.name, repo);
			success = `Added ${repo}.`;
			selectedGroup = await getGroup(selectedGroup.name);
			addingMember = false;
		} catch (err) {
			error = formatError(err);
		} finally {
			loading = false;
		}
	};

	const handleRemoveMember = async (repo: string): Promise<void> => {
		if (!selectedGroup) return;

		loading = true;
		error = null;
		success = null;

		try {
			await removeGroupMember(selectedGroup.name, repo);
			success = `Removed ${repo}.`;
			selectedGroup = await getGroup(selectedGroup.name);
		} catch (err) {
			error = formatError(err);
		} finally {
			loading = false;
		}
	};

	onMount(() => {
		void loadGroups();
	});
</script>

<SettingsSection
	title="Groups"
	description="Collections of repos that can be applied to new workspaces."
>
	<div class="manager">
		<div class="list-header">
			<span class="list-count">{groups.length} group{groups.length === 1 ? '' : 's'}</span>
			<Button variant="ghost" size="sm" onclick={startNew}>+ New</Button>
		</div>

		{#if groups.length > 0 || isNew}
			<div class="list">
				{#each groups as group (group)}
					<button
						class="list-item"
						class:active={selectedGroup?.name === group.name && !isNew}
						type="button"
						onclick={() => selectGroup(group)}
					>
						<span class="item-name">{group.name}</span>
						<span class="item-count"
							>({group.repo_count} repo{group.repo_count === 1 ? '' : 's'})</span
						>
					</button>
				{/each}
				{#if isNew}
					<button class="list-item active" type="button">
						<span class="item-name new">New group</span>
					</button>
				{/if}
			</div>

			{#if error}
				<div class="message error">{error}</div>
			{:else if success}
				<div class="message success">{success}</div>
			{/if}

			<div class="detail">
				<div class="detail-header">
					{#if isNew}
						New group
					{:else if selectedGroup}
						{selectedGroup.name}
					{/if}
				</div>
				<div class="form">
					<label class="field">
						<span>Name</span>
						<input
							type="text"
							bind:value={formName}
							placeholder="core-services"
							disabled={!isNew && !!selectedGroup}
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</label>
					<label class="field">
						<span>Description</span>
						<input
							type="text"
							bind:value={formDescription}
							placeholder="Core backend microservices"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</label>
				</div>
				<div class="actions">
					{#if !isNew && selectedGroup}
						<Button variant="danger" onclick={handleDelete} disabled={loading}>Delete group</Button>
					{/if}
					<div class="spacer"></div>
					{#if isNew}
						<Button variant="ghost" onclick={cancelEdit} disabled={loading}>Cancel</Button>
					{/if}
					<Button variant="primary" onclick={handleSave} disabled={loading}>
						{loading ? 'Saving...' : isNew ? 'Create group' : 'Save group'}
					</Button>
				</div>

				{#if selectedGroup && !isNew}
					<div class="members-section">
						<div class="members-header">
							<span class="members-label">
								Members ({selectedGroup.members.length})
							</span>
							<Button variant="ghost" size="sm" onclick={startAddMember}>+ Add repo</Button>
						</div>

						{#if addingMember}
							<div class="add-member-form">
								<div class="form-row">
									<label class="field">
										<span>Repo name (registered repo)</span>
										<input
											type="text"
											value={memberRepo}
											oninput={handleRepoInput}
											placeholder="auth-api"
											list="repo-options"
											autocapitalize="off"
											autocorrect="off"
											spellcheck="false"
										/>
										<datalist id="repo-options">
											{#each aliases as alias (alias)}
												<option value={alias.name}></option>
											{/each}
										</datalist>
									</label>
								</div>
								<div class="add-member-actions">
									<Button variant="ghost" size="sm" onclick={cancelAddMember} disabled={loading}>
										Cancel
									</Button>
									<Button variant="primary" size="sm" onclick={handleAddMember} disabled={loading}>
										{loading ? 'Adding...' : 'Add repo'}
									</Button>
								</div>
							</div>
						{/if}

						<div class="members-list">
							{#each selectedGroup.members as member (member.repo)}
								<GroupMemberRow
									{member}
									{loading}
									onRemove={() => handleRemoveMember(member.repo)}
								/>
							{/each}
							{#if selectedGroup.members.length === 0 && !addingMember}
								<div class="empty-members">No members yet. Add repos to this group.</div>
							{/if}
						</div>
					</div>
				{/if}
			</div>
		{:else}
			<div class="empty">
				<p>No groups defined yet.</p>
				<Button variant="ghost" onclick={startNew}>Create your first group</Button>
			</div>
		{/if}
	</div>
</SettingsSection>

<style>
	.manager {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.list-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
	}

	.list-count {
		font-size: 12px;
		color: var(--muted);
	}

	.list {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		max-height: 160px;
		overflow-y: auto;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: var(--space-1);
		background: var(--panel);
	}

	.list-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: 10px var(--space-3);
		border: none;
		background: transparent;
		color: var(--text);
		font-size: 13px;
		font-family: inherit;
		text-align: left;
		border-radius: var(--radius-sm);
		cursor: pointer;
		transition: background var(--transition-fast);
	}

	.list-item:hover {
		background: rgba(255, 255, 255, 0.04);
	}

	.list-item.active {
		background: rgba(255, 255, 255, 0.08);
	}

	.item-name {
		font-weight: 500;
	}

	.item-name.new {
		font-style: italic;
		color: var(--accent);
	}

	.item-count {
		font-size: 12px;
		color: var(--muted);
	}

	.detail {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		padding: var(--space-4);
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}

	.detail-header {
		font-size: 14px;
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.form {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 12px;
		color: var(--muted);
	}

	.field input {
		background: var(--panel-strong);
		border: 1px solid rgba(255, 255, 255, 0.08);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 10px var(--space-3);
		font-size: 13px;
		font-family: inherit;
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.field input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 2px var(--accent-soft);
	}

	.field input:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.actions {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding-top: var(--space-2);
		border-top: 1px solid var(--border);
	}

	.spacer {
		flex: 1;
	}

	.members-section {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		padding-top: var(--space-3);
		border-top: 1px solid var(--border);
	}

	.members-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.members-label {
		font-size: 13px;
		font-weight: 600;
	}

	.members-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
		max-height: 240px;
		overflow-y: auto;
	}

	.empty-members {
		padding: var(--space-4);
		text-align: center;
		font-size: 13px;
		color: var(--muted);
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}

	.add-member-form {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		padding: var(--space-3);
		background: var(--panel);
		border: 1px solid var(--accent-soft);
		border-radius: var(--radius-md);
	}

	.form-row {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
		gap: var(--space-3);
	}

	.add-member-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
	}

	.message {
		font-size: 13px;
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius-md);
	}

	.message.error {
		background: var(--danger-subtle);
		color: var(--danger);
	}

	.message.success {
		background: rgba(74, 222, 128, 0.1);
		color: var(--success);
	}

	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-3);
		padding: 32px;
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		text-align: center;
	}

	.empty p {
		margin: 0;
		color: var(--muted);
		font-size: 14px;
	}
</style>
