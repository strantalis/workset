<script lang="ts">
	import { onMount } from 'svelte';
	import { Plus, Layers, Trash2, Pencil, Search, Check } from '@lucide/svelte';
	import {
		addGroupMember,
		createGroup,
		deleteGroup,
		getGroup,
		listAliases,
		listGroups,
		removeGroupMember,
		updateGroup,
	} from '../../../api/settings';
	import type { Alias, GroupSummary } from '../../../types';
	import { toErrorMessage } from '../../../errors';
	import SettingsSection from '../SettingsSection.svelte';
	import Button from '../../ui/Button.svelte';

	interface Props {
		onGroupCountChange: (count: number) => void;
	}

	const { onGroupCountChange }: Props = $props();

	let groups: GroupSummary[] = $state([]);
	let aliases: Alias[] = $state([]);
	let loading = $state(false);
	let error: string | null = $state(null);
	let success: string | null = $state(null);

	// Editor state
	let isEditing: string | null = $state(null); // 'new' or group name
	let formName = $state('');
	let formDescription = $state('');
	let selectedRepos: string[] = $state([]);
	let repoSearch = $state('');

	const loadGroups = async (): Promise<void> => {
		try {
			groups = await listGroups();
			aliases = await listAliases();
			onGroupCountChange(groups.length);
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		}
	};

	const startNew = (): void => {
		isEditing = 'new';
		formName = '';
		formDescription = '';
		selectedRepos = [];
		repoSearch = '';
		error = null;
		success = null;
	};

	const startEdit = async (summary: GroupSummary): Promise<void> => {
		try {
			const group = await getGroup(summary.name);
			isEditing = summary.name;
			formName = group.name;
			formDescription = group.description ?? '';
			selectedRepos = group.members.map((m) => m.repo);
			repoSearch = '';
			error = null;
			success = null;
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		}
	};

	const cancelEdit = (): void => {
		isEditing = null;
		formName = '';
		formDescription = '';
		selectedRepos = [];
		repoSearch = '';
		error = null;
	};

	const handleSave = async (): Promise<void> => {
		const name = formName.trim();
		const description = formDescription.trim();

		if (!name) {
			error = 'Template name is required.';
			return;
		}

		loading = true;
		error = null;
		success = null;

		try {
			if (isEditing === 'new') {
				await createGroup(name, description);
				// Add selected repos as members
				for (const repo of selectedRepos) {
					await addGroupMember(name, repo);
				}
				success = `Created template "${name}".`;
			} else {
				await updateGroup(name, description);
				// Update members - get current group to compare
				const currentGroup = await getGroup(name);
				const currentRepos = currentGroup.members.map((m) => m.repo);
				// Add new repos
				for (const repo of selectedRepos) {
					if (!currentRepos.includes(repo)) {
						await addGroupMember(name, repo);
					}
				}
				// Remove repos that are no longer selected
				for (const repo of currentRepos) {
					if (!selectedRepos.includes(repo)) {
						await removeGroupMember(name, repo);
					}
				}
				success = `Updated template "${name}".`;
			}
			await loadGroups();
			isEditing = null;
			formName = '';
			formDescription = '';
			selectedRepos = [];
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		} finally {
			loading = false;
		}
	};

	const handleDelete = async (name: string): Promise<void> => {
		const confirmed = window.confirm(`Delete template "${name}"?`);
		if (!confirmed) return;

		loading = true;
		error = null;
		success = null;

		try {
			await deleteGroup(name);
			success = `Deleted template "${name}".`;
			await loadGroups();
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		} finally {
			loading = false;
		}
	};

	const toggleRepo = (repo: string): void => {
		if (selectedRepos.includes(repo)) {
			selectedRepos = selectedRepos.filter((r) => r !== repo);
		} else {
			selectedRepos = [...selectedRepos, repo];
		}
	};

	const filteredRepos = $derived(
		aliases.filter((a) => a.name.toLowerCase().includes(repoSearch.toLowerCase())),
	);

	onMount(() => {
		void loadGroups();
	});
</script>

<SettingsSection
	title="Workset Templates"
	description="Create reusable templates to quickly provision new worksets. Templates allow you to define the exact set of repositories needed for a specific domain."
>
	<div class="templates-container">
		<!-- Header -->
		<div class="templates-header">
			<div class="header-text">
				<h3 class="section-title">Your Templates</h3>
				<p class="section-desc">
					{groups.length}
					{groups.length === 1 ? 'template' : 'templates'} defined
				</p>
			</div>
			{#if !isEditing}
				<Button variant="primary" size="sm" onclick={startNew} class="new-template-btn">
					<Plus size={14} />
					New Template
				</Button>
			{/if}
		</div>

		<!-- Messages -->
		{#if error && !isEditing}
			<div class="message error">{error}</div>
		{:else if success && !isEditing}
			<div class="message success">{success}</div>
		{/if}

		<!-- Editor Form -->
		{#if isEditing}
			<div class="editor-card">
				<div class="editor-header">
					<h3>{isEditing === 'new' ? 'Create New Template' : 'Edit Template'}</h3>
				</div>

				<div class="editor-fields">
					<div class="form-field">
						<label for="tpl-name">Template Name</label>
						<input
							id="tpl-name"
							type="text"
							bind:value={formName}
							placeholder="e.g. Platform Core"
							disabled={isEditing !== 'new'}
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</div>

					<div class="form-field">
						<label for="tpl-desc">Description</label>
						<input
							id="tpl-desc"
							type="text"
							bind:value={formDescription}
							placeholder="What is this template used for?"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</div>

					<div class="repos-section">
						<label class="repos-label" for="tpl-repos-search">Repositories</label>
						<div class="repos-box">
							<div class="repos-search">
								<Search size={14} class="search-icon" />
								<input
									id="tpl-repos-search"
									type="text"
									placeholder="Search repositories..."
									bind:value={repoSearch}
								/>
							</div>
							<div class="repos-list">
								{#each filteredRepos as alias (alias.name)}
									{@const isSelected = selectedRepos.includes(alias.name)}
									<button
										class="repo-row"
										class:selected={isSelected}
										onclick={() => toggleRepo(alias.name)}
										type="button"
									>
										<div class="checkbox">
											{#if isSelected}
												<Check size={10} />
											{/if}
										</div>
										<span class="repo-name">{alias.name}</span>
									</button>
								{/each}
							</div>
							<div class="repos-footer">
								<span>{selectedRepos.length} selected</span>
								{#if selectedRepos.length > 0}
									<button type="button" class="clear-btn" onclick={() => (selectedRepos = [])}>
										Clear all
									</button>
								{/if}
							</div>
						</div>
					</div>
				</div>

				{#if error}
					<div class="message error">{error}</div>
				{/if}

				<div class="editor-actions">
					<Button variant="ghost" size="sm" onclick={cancelEdit} disabled={loading}>Cancel</Button>
					<Button
						variant="primary"
						size="sm"
						onclick={handleSave}
						disabled={loading || !formName.trim()}
					>
						{loading ? 'Saving...' : isEditing === 'new' ? 'Create Template' : 'Save Changes'}
					</Button>
				</div>
			</div>
		{:else}
			<!-- Template Cards Grid -->
			{#if groups.length === 0}
				<div class="empty-state">
					<Layers size={32} class="empty-icon" />
					<h4>No templates defined</h4>
					<p>Create your first template to start organizing your repos.</p>
					<Button variant="primary" onclick={startNew}>Create Template</Button>
				</div>
			{:else}
				<div class="templates-grid">
					{#each groups as group (group.name)}
						<div class="template-card">
							<div class="card-header">
								<div class="card-icon">
									<Layers size={20} />
								</div>
								<div class="card-title-section">
									<h4>{group.name}</h4>
									<span class="repo-count"
										>{group.repo_count}
										{group.repo_count === 1 ? 'repository' : 'repositories'}</span
									>
								</div>
								<div class="card-actions">
									<button class="action-btn" onclick={() => startEdit(group)} title="Edit">
										<Pencil size={14} />
									</button>
									<button
										class="action-btn danger"
										onclick={() => handleDelete(group.name)}
										disabled={loading}
										title="Delete"
									>
										<Trash2 size={14} />
									</button>
								</div>
							</div>

							{#if group.description}
								<p class="card-description">{group.description}</p>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		{/if}
	</div>
</SettingsSection>

<style>
	.templates-container {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.templates-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: var(--space-3);
	}

	.header-text {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.section-title {
		font-size: var(--text-md);
		font-weight: 600;
		color: var(--text);
		margin: 0;
	}

	.section-desc {
		font-size: var(--text-base);
		color: var(--muted);
		margin: 0;
	}

	:global(.new-template-btn) {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.message {
		font-size: var(--text-base);
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius-md);
	}

	.message.error {
		background: var(--danger-subtle);
		color: var(--danger);
	}

	.message.success {
		background: var(--success-subtle);
		color: var(--success);
	}

	.editor-card {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: var(--space-5);
	}

	.editor-header h3 {
		font-size: var(--text-lg);
		font-weight: 600;
		color: var(--text);
		margin: 0 0 var(--space-4) 0;
	}

	.editor-fields {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.form-field {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.form-field label {
		font-size: var(--text-xs);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
	}

	.form-field input {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 10px 12px;
		font-size: var(--text-base);
		transition: border-color var(--transition-fast);
	}

	.form-field input:focus {
		outline: none;
		border-color: var(--accent);
	}

	.form-field input:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.repos-section {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.repos-label {
		font-size: var(--text-xs);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
	}

	.repos-box {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.repos-search {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		border-bottom: 1px solid var(--border);
	}

	:global(.repos-search .search-icon) {
		color: var(--muted);
		flex-shrink: 0;
	}

	.repos-search input {
		flex: 1;
		background: transparent;
		border: none;
		color: var(--text);
		font-size: var(--text-base);
		outline: none;
	}

	.repos-search input::placeholder {
		color: var(--subtle);
	}

	.repos-list {
		max-height: 200px;
		overflow-y: auto;
		padding: var(--space-1);
	}

	.repo-row {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-2) var(--space-3);
		border: none;
		background: transparent;
		color: var(--text);
		font-size: var(--text-mono-base);
		font-family: var(--font-mono);
		text-align: left;
		border-radius: var(--radius-sm);
		cursor: pointer;
		transition: background var(--transition-fast);
	}

	.repo-row:hover {
		background: color-mix(in srgb, var(--text) 5%, transparent);
	}

	.repo-row.selected {
		background: var(--accent-soft);
		color: var(--accent);
	}

	.checkbox {
		width: 16px;
		height: 16px;
		border: 1px solid var(--border);
		border-radius: 4px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		background: var(--panel);
	}

	.repo-row.selected .checkbox {
		background: var(--accent);
		border-color: var(--accent);
		color: white;
	}

	.repos-footer {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: var(--space-2) var(--space-3);
		background: var(--panel);
		border-top: 1px solid var(--border);
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.clear-btn {
		background: transparent;
		border: none;
		color: var(--muted);
		font-size: var(--text-sm);
		cursor: pointer;
		transition: color var(--transition-fast);
	}

	.clear-btn:hover {
		color: var(--text);
	}

	.editor-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
		margin-top: var(--space-4);
		padding-top: var(--space-4);
		border-top: 1px solid var(--border);
	}

	.templates-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
		gap: var(--space-4);
	}

	.template-card {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: var(--space-4);
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		transition: border-color var(--transition-fast);
	}

	.template-card:hover {
		border-color: var(--accent-soft);
	}

	.card-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.card-icon {
		width: 40px;
		height: 40px;
		background: var(--panel-strong);
		border-radius: var(--radius-md);
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--accent);
		flex-shrink: 0;
	}

	.card-title-section {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex: 1;
		min-width: 0;
	}

	.card-title-section h4 {
		font-size: var(--text-md);
		font-weight: 600;
		color: var(--text);
		margin: 0;
	}

	.repo-count {
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.card-description {
		font-size: var(--text-base);
		color: var(--muted);
		line-height: 1.5;
		margin: 0;
	}

	.card-actions {
		display: flex;
		gap: var(--space-1);
		opacity: 0;
		transition: opacity var(--transition-fast);
		margin-left: auto;
	}

	.template-card:hover .card-actions {
		opacity: 1;
	}

	.action-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: none;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		border-radius: var(--radius-sm);
		transition: all var(--transition-fast);
	}

	.action-btn:hover {
		background: var(--panel-strong);
		color: var(--text);
	}

	.action-btn.danger:hover {
		background: var(--danger-subtle);
		color: var(--danger);
	}

	.action-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-3);
		padding: 64px var(--space-4);
		background: var(--panel);
		border: 1px dashed var(--border);
		border-radius: var(--radius-lg);
		text-align: center;
	}

	:global(.empty-state .empty-icon) {
		color: var(--muted);
		opacity: 0.5;
	}

	.empty-state h4 {
		font-size: var(--text-lg);
		font-weight: 600;
		color: var(--text);
		margin: 0;
	}

	.empty-state p {
		font-size: var(--text-md);
		color: var(--muted);
		margin: 0;
	}
</style>
