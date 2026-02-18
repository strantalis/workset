<script lang="ts">
	import { onMount, tick } from 'svelte';
	import {
		activeWorkspaceId,
		clearRepo,
		clearWorkspace,
		loadWorkspaces,
		selectWorkspace,
		workspaces,
	} from '../state';
	import {
		addRepo,
		archiveWorkspace,
		createWorkspace,
		removeRepo,
		removeWorkspace,
		renameWorkspace,
		unarchiveWorkspace,
	} from '../api/workspaces';
	import type { Repo, Workspace } from '../types';
	import WorkspaceManagerCreateWorkspaceSection from './workspace-manager/WorkspaceManagerCreateWorkspaceSection.svelte';
	import WorkspaceManagerRepoListSection from './workspace-manager/WorkspaceManagerRepoListSection.svelte';
	import WorkspaceManagerWorkspaceList from './workspace-manager/WorkspaceManagerWorkspaceList.svelte';

	interface Props {
		onClose: () => void;
		initialWorkspaceId?: string | null;
		initialRepoName?: string | null;
		initialSection?: 'create' | 'rename' | 'repo' | null;
	}

	const {
		onClose,
		initialWorkspaceId = null,
		initialRepoName = null,
		initialSection = null,
	}: Props = $props();

	let selectedWorkspaceId: string | null = $state(null);
	let showArchived = $state(false);

	let createName = $state('');
	let createPath = $state('');
	let createError: string | null = $state(null);
	let createSuccess: string | null = $state(null);
	let creating = $state(false);
	let workspaceError: string | null = $state(null);
	let createInput: HTMLInputElement | null = $state(null);

	let addSource = $state('');
	let addName = $state('');
	let addRepoDir = $state('');
	let addError: string | null = $state(null);
	let addSuccess: string | null = $state(null);
	let addWarnings: string[] = $state([]);
	let adding = $state(false);
	let addSourceInput: HTMLInputElement | null = $state(null);

	let selectedRepoName: string | null = $state(null);

	let renameName = $state('');
	let renameError: string | null = $state(null);
	let renameSuccess: string | null = $state(null);
	let renaming = $state(false);
	let lastSelectedId: string | null = $state(null);
	let lastSelectedRepoName: string | null = $state(null);
	let renameInput: HTMLInputElement | null = $state(null);

	let confirmWorkspaceRemove: string | null = $state(null);
	let confirmRepoRemove: { workspaceId: string; repoName: string } | null = $state(null);
	let removeRepoDeleteWorktree = $state(false);
	let removeRepoDeleteLocal = $state(false);
	let working = false;

	const selectManagerWorkspace = (id: string): void => {
		selectedWorkspaceId = id;
		confirmWorkspaceRemove = null;
		confirmRepoRemove = null;
		removeRepoDeleteWorktree = false;
		removeRepoDeleteLocal = false;
		addError = null;
		addSuccess = null;
		addWarnings = [];
		workspaceError = null;
	};

	const managerWorkspaces = $derived($workspaces);
	const activeWorkspaces = $derived(managerWorkspaces.filter((workspace) => !workspace.archived));
	const archivedWorkspaces = $derived(managerWorkspaces.filter((workspace) => workspace.archived));
	const selectedWorkspace = $derived(
		managerWorkspaces.find((workspace) => workspace.id === selectedWorkspaceId) ?? null,
	);
	$effect(() => {
		if (!selectedWorkspaceId && managerWorkspaces.length > 0) {
			selectedWorkspaceId = $activeWorkspaceId ?? managerWorkspaces[0]?.id ?? null;
		}
	});
	$effect(() => {
		if (selectedWorkspace && selectedWorkspace.id !== lastSelectedId) {
			renameName = selectedWorkspace.name;
			renameError = null;
			renameSuccess = null;
			lastSelectedId = selectedWorkspace.id;
			selectedRepoName = selectedWorkspace.repos[0]?.name ?? null;
			lastSelectedRepoName = null;
		}
	});
	$effect(() => {
		if (selectedWorkspace && selectedRepoName) {
			const exists = selectedWorkspace.repos.some((repo) => repo.name === selectedRepoName);
			if (!exists) {
				selectedRepoName = selectedWorkspace.repos[0]?.name ?? null;
				lastSelectedRepoName = null;
			}
		}
	});
	const selectedRepo = $derived(
		selectedWorkspace?.repos.find((repo) => repo.name === selectedRepoName) ?? null,
	);
	$effect(() => {
		if (selectedRepo && selectedRepo.name !== lastSelectedRepoName) {
			lastSelectedRepoName = selectedRepo.name;
		}
	});

	const formatError = (err: unknown, fallback: string): string => {
		if (err instanceof Error) {
			return err.message;
		}
		if (typeof err === 'string') {
			return err;
		}
		if (err && typeof err === 'object' && 'message' in err) {
			const message = (err as { message?: string }).message;
			if (typeof message === 'string') {
				return message;
			}
		}
		return fallback;
	};

	const handleCreate = async (): Promise<void> => {
		if (creating) return;
		createError = null;
		createSuccess = null;
		const name = createName.trim();
		if (!name) {
			createError = 'Workspace name is required.';
			return;
		}
		creating = true;
		try {
			const result = await createWorkspace(name, createPath.trim());
			createName = '';
			createPath = '';
			createSuccess = `Created ${result.workspace.name}.`;
			await loadWorkspaces(true);
			selectWorkspace(result.workspace.name);
			selectedWorkspaceId = result.workspace.name;
		} catch (err) {
			createError = formatError(err, 'Failed to create workspace.');
		} finally {
			creating = false;
		}
	};

	const handleArchive = async (workspace: Workspace): Promise<void> => {
		if (working) return;
		workspaceError = null;
		working = true;
		try {
			await archiveWorkspace(workspace.id, '');
			await loadWorkspaces(true);
			if ($activeWorkspaceId === workspace.id) {
				clearWorkspace();
			}
			if (selectedWorkspaceId === workspace.id) {
				selectedWorkspaceId = null;
			}
		} catch (err) {
			workspaceError = formatError(err, 'Failed to archive workspace.');
		} finally {
			working = false;
		}
	};

	const handleUnarchive = async (workspace: Workspace): Promise<void> => {
		if (working) return;
		workspaceError = null;
		working = true;
		try {
			await unarchiveWorkspace(workspace.id);
			await loadWorkspaces(true);
			selectedWorkspaceId = workspace.id;
		} catch (err) {
			workspaceError = formatError(err, 'Failed to unarchive workspace.');
		} finally {
			working = false;
		}
	};

	const handleRemoveWorkspace = async (workspace: Workspace): Promise<void> => {
		if (working) return;
		workspaceError = null;
		working = true;
		try {
			await removeWorkspace(workspace.id);
			await loadWorkspaces(true);
			if ($activeWorkspaceId === workspace.id) {
				clearWorkspace();
			}
			if (selectedWorkspaceId === workspace.id) {
				selectedWorkspaceId = null;
			}
		} catch (err) {
			workspaceError = formatError(err, 'Failed to remove workspace.');
		} finally {
			confirmWorkspaceRemove = null;
			working = false;
		}
	};

	const handleAddRepo = async (): Promise<void> => {
		if (adding || !selectedWorkspace) return;
		addError = null;
		addSuccess = null;
		addWarnings = [];
		const source = addSource.trim();
		if (!source) {
			addError = 'Repo source is required.';
			return;
		}
		adding = true;
		try {
			const result = await addRepo(selectedWorkspace.id, source, addName.trim(), addRepoDir.trim());
			const warnings = result.warnings ?? [];
			addSource = '';
			addName = '';
			addRepoDir = '';
			if (warnings.length > 0) {
				addWarnings = Array.from(new Set(warnings));
				addSuccess = `Repo added with ${addWarnings.length} warning${addWarnings.length === 1 ? '' : 's'}.`;
			} else {
				addSuccess = 'Repo added.';
			}
			await loadWorkspaces(true);
		} catch (err) {
			addError = formatError(err, 'Failed to add repo.');
		} finally {
			adding = false;
		}
	};

	const handleRemoveRepo = async (workspace: Workspace, repo: Repo): Promise<void> => {
		if (working) return;
		working = true;
		try {
			await removeRepo(workspace.id, repo.name, removeRepoDeleteWorktree, removeRepoDeleteLocal);
			await loadWorkspaces(true);
			if ($activeWorkspaceId === workspace.id) {
				clearRepo();
			}
		} catch (err) {
			addError = formatError(err, 'Failed to remove repo.');
		} finally {
			confirmRepoRemove = null;
			removeRepoDeleteWorktree = false;
			removeRepoDeleteLocal = false;
			working = false;
		}
	};

	const handleRename = async (): Promise<void> => {
		if (renaming || !selectedWorkspace) return;
		renameError = null;
		renameSuccess = null;
		const nextName = renameName.trim();
		if (!nextName) {
			renameError = 'New name is required.';
			return;
		}
		if (nextName === selectedWorkspace.name) {
			renameSuccess = 'Name is unchanged.';
			return;
		}
		renaming = true;
		try {
			const currentId = selectedWorkspace.id;
			await renameWorkspace(currentId, nextName);
			await loadWorkspaces(true);
			if ($activeWorkspaceId === currentId) {
				selectWorkspace(nextName);
			}
			selectedWorkspaceId = nextName;
			renameSuccess = `Renamed to ${nextName}.`;
		} catch (err) {
			renameError = formatError(err, 'Failed to rename workspace.');
		} finally {
			renaming = false;
		}
	};

	onMount(() => {
		void loadWorkspaces(true);
		if (!selectedWorkspaceId) {
			selectedWorkspaceId = initialWorkspaceId ?? $activeWorkspaceId ?? $workspaces[0]?.id ?? null;
		}
		if (initialRepoName) {
			selectedRepoName = initialRepoName;
		}
		void tick().then(() => {
			if (initialSection === 'create') {
				createInput?.focus();
			} else if (initialSection === 'rename') {
				renameInput?.focus();
			} else if (initialSection === 'repo') {
				addSourceInput?.focus();
			}
		});
	});
</script>

<div class="panel" role="dialog" aria-modal="true" aria-label="Workspace management">
	<header class="header">
		<div>
			<div class="title">Workspaces</div>
			<div class="subtitle">Create and manage workspace registrations and repos.</div>
		</div>
		<button class="ghost" type="button" onclick={onClose}>Close</button>
	</header>

	<WorkspaceManagerCreateWorkspaceSection
		{createName}
		{createPath}
		{createError}
		{createSuccess}
		{creating}
		onCreateNameChange={(value) => (createName = value)}
		onCreatePathChange={(value) => (createPath = value)}
		onCreateInputChange={(input) => (createInput = input)}
		onCreate={() => void handleCreate()}
	/>

	<section class="list">
		<div class="list-grid">
			<WorkspaceManagerWorkspaceList
				{showArchived}
				{workspaceError}
				{activeWorkspaces}
				{archivedWorkspaces}
				{selectedWorkspaceId}
				{confirmWorkspaceRemove}
				onShowArchivedChange={(value) => (showArchived = value)}
				onSelectWorkspace={selectManagerWorkspace}
				onOpenWorkspace={selectWorkspace}
				onArchiveWorkspace={(workspace) => void handleArchive(workspace)}
				onUnarchiveWorkspace={(workspace) => void handleUnarchive(workspace)}
				onConfirmRemoveWorkspace={(workspaceId) => (confirmWorkspaceRemove = workspaceId)}
				onRemoveWorkspace={(workspace) => void handleRemoveWorkspace(workspace)}
			/>

			<div class="details-column">
				{#if selectedWorkspace}
					<div class="details-card">
						<div class="details-header">
							<div>
								<div class="details-title">{selectedWorkspace.name}</div>
								<div class="details-sub">{selectedWorkspace.path}</div>
							</div>
							<div class="pill">{selectedWorkspace.repos.length} repos</div>
						</div>

						<div class="rename">
							<div class="section-title">Rename workspace</div>
							<div class="form-grid">
								<label class="field span-2">
									<span>New name</span>
									<input
										placeholder="acme"
										bind:this={renameInput}
										bind:value={renameName}
										autocapitalize="off"
										autocorrect="off"
										spellcheck="false"
										onkeydown={(event) => {
											if (event.key === 'Enter') void handleRename();
										}}
									/>
								</label>
							</div>
							<div class="inline-actions">
								<button class="primary" type="button" onclick={handleRename} disabled={renaming}>
									{renaming ? 'Renaming…' : 'Rename'}
								</button>
								{#if renameError}
									<div class="note error">{renameError}</div>
								{:else if renameSuccess}
									<div class="note success">{renameSuccess}</div>
								{/if}
							</div>
							<div class="hint">Renaming updates config and workset.yaml. Files stay in place.</div>
						</div>

						<div class="repo-add">
							<div class="section-title">Add repo</div>
							<div class="form-grid">
								<label class="field span-2">
									<span>Source</span>
									<input
										placeholder="registered repo, URL, or local path"
										bind:this={addSourceInput}
										bind:value={addSource}
										autocapitalize="off"
										autocorrect="off"
										spellcheck="false"
										onkeydown={(event) => {
											if (event.key === 'Enter') void handleAddRepo();
										}}
									/>
								</label>
								<label class="field">
									<span>Repo name (optional)</span>
									<input
										placeholder="agent-skills"
										bind:value={addName}
										autocapitalize="off"
										autocorrect="off"
										spellcheck="false"
									/>
								</label>
								<label class="field">
									<span>Repo dir (optional)</span>
									<input
										placeholder="agent-skills"
										bind:value={addRepoDir}
										autocapitalize="off"
										autocorrect="off"
										spellcheck="false"
									/>
								</label>
							</div>
							<div class="inline-actions">
								<button class="primary" type="button" onclick={handleAddRepo} disabled={adding}>
									{adding ? 'Adding…' : 'Add repo'}
								</button>
								{#if addError}
									<div class="note error">{addError}</div>
								{:else}
									{#if addSuccess}
										<div class="note success">{addSuccess}</div>
									{/if}
									{#if addWarnings.length > 0}
										<div class="note warning">
											{#each addWarnings as warning (warning)}
												<div>{warning}</div>
											{/each}
										</div>
									{/if}
								{/if}
							</div>
							<div class="hint">Removes only update the workset config. Files stay on disk.</div>
						</div>

						<WorkspaceManagerRepoListSection
							{selectedWorkspace}
							{selectedRepoName}
							{confirmRepoRemove}
							{removeRepoDeleteWorktree}
							{removeRepoDeleteLocal}
							onSelectRepoName={(repoName) => (selectedRepoName = repoName)}
							onConfirmRepoRemove={(value) => (confirmRepoRemove = value)}
							onRemoveRepoDeleteWorktreeChange={(value) => (removeRepoDeleteWorktree = value)}
							onRemoveRepoDeleteLocalChange={(value) => (removeRepoDeleteLocal = value)}
							onRemoveRepo={(workspace, repo) => void handleRemoveRepo(workspace, repo)}
						/>
					</div>
				{:else}
					<div class="details-card empty">
						<div class="details-title">Pick a workspace to manage repos.</div>
						<div class="details-sub">Select a workspace to view repos and add new ones.</div>
					</div>
				{/if}
			</div>
		</div>
	</section>
</div>

<style>
	.panel {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 20px;
		padding: 24px;
		max-width: 1120px;
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 20px;
		box-shadow: 0 30px 80px rgba(6, 10, 16, 0.6);
	}

	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}

	.title {
		font-size: var(--text-2xl);
		font-weight: 600;
	}

	.subtitle {
		color: var(--muted);
		font-size: var(--text-base);
	}

	.ghost {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		color: var(--text);
		padding: 6px 12px;
		border-radius: 8px;
		cursor: pointer;
	}

	.primary {
		background: var(--accent);
		color: #081018;
		border: none;
		padding: 8px 16px;
		border-radius: 10px;
		font-weight: 600;
		cursor: pointer;
	}

	.list {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 16px;
	}

	.section-title {
		font-size: var(--text-base);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
		font-weight: 600;
	}

	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
		margin-top: 12px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.field input {
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: 10px;
		color: var(--text);
		padding: 8px 10px;
		font-size: var(--text-md);
	}

	.span-2 {
		grid-column: span 2;
	}

	.inline-actions {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-top: 12px;
	}

	.note {
		font-size: var(--text-base);
	}

	.note.error {
		color: var(--danger);
	}

	.note.success {
		color: var(--success);
	}

	.note.warning {
		color: var(--warning);
	}

	.list-grid {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr);
		gap: 16px;
		margin-top: 16px;
	}

	.details-column {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.details-card {
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 16px;
		background: var(--panel-soft);
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.details-card.empty {
		align-items: flex-start;
		justify-content: center;
	}

	.details-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
	}

	.details-title {
		font-size: var(--text-xl);
		font-weight: 600;
	}

	.details-sub {
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.pill {
		background: rgba(255, 255, 255, 0.06);
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 4px 10px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.repo-add {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.hint {
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.empty {
		font-size: var(--text-base);
		color: var(--muted);
		padding: 8px 0;
	}

	@media (max-width: 1000px) {
		.list-grid {
			grid-template-columns: 1fr;
		}
	}

	@media (max-width: 720px) {
		.panel {
			border-radius: 0;
			height: 100%;
			overflow: auto;
		}
	}
</style>
