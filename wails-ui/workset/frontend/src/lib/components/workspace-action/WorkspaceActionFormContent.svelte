<script lang="ts">
	import { onMount } from 'svelte';
	import type { RepoHooksPreviewUnavailableReason } from '../../api/workspaces';
	import type { Alias, HookExecution, Repo } from '../../types';
	import type {
		ExistingRepoContext,
		WorkspaceActionAddRepoSelectedItem,
	} from '../../services/workspaceActionContextService';
	import type { RepoCreationStatus } from '../../services/workspaceActionHooks';
	import Button from '../ui/Button.svelte';
	import WorkspaceActionAddRepoForm from './WorkspaceActionAddRepoForm.svelte';
	import WorkspaceActionCreateForm from './WorkspaceActionCreateForm.svelte';
	import WorkspaceActionRemoveRepoForm from './WorkspaceActionRemoveRepoForm.svelte';
	import WorkspaceActionRemoveWorkspaceForm from './WorkspaceActionRemoveWorkspaceForm.svelte';

	type Mode =
		| 'create'
		| 'create-thread'
		| 'rename'
		| 'add-repo'
		| 'archive'
		| 'remove-thread'
		| 'remove-repo'
		| null;
	type ThreadHookPreviewRow = {
		repoName: string;
		hooks: string[];
		hasSource: boolean;
		previewUnavailableReason: RepoHooksPreviewUnavailableReason | null;
	};

	interface Props {
		mode: Mode;
		loading: boolean;

		aliasItems: Alias[];
		searchQuery: string;
		filteredAliases: Alias[];
		selectedAliases: Set<string>;
		getAliasSource: (alias: Alias) => string;

		renameName: string;
		onRenameNameInput: (value: string) => void;
		onRenameSubmit: () => void;

		addSource: string;
		existingRepos: ExistingRepoContext[];
		addRepoSelectedItems: WorkspaceActionAddRepoSelectedItem[];
		addRepoTotalItems: number;
		onAddSearchQueryInput: (value: string) => void;
		onAddSourceInput: (value: string) => void;
		onAddBrowse: () => void;
		onAddToggleAlias: (name: string) => void;
		onAddRemoveAlias: (name: string) => void;
		onAddSubmit: () => void;

		createWorkspaceName: string;
		createDescription?: string;
		createNameValidationError?: string | null;
		createWorksetLabel?: string | null;
		createSourceInput: string;
		createDirectRepos: Array<{ url: string; register: boolean }>;
		createThreadHookRows?: ThreadHookPreviewRow[];
		createThreadHooksLoading?: boolean;
		createThreadHooksError?: string | null;
		createCreating?: boolean;
		createRepoProgress?: Record<string, RepoCreationStatus>;
		createHookRuns?: HookExecution[];
		onCreateWorkspaceNameInput: (value: string) => void;
		onCreateDescriptionInput?: (value: string) => void;
		onCreateSearchQueryInput: (value: string) => void;
		onCreateSourceInput: (value: string) => void;
		onCreateAddDirectRepo: () => void;
		onCreateRemoveDirectRepo: (url: string) => void;
		onCreateToggleDirectRepoRegister: (url: string) => void;
		onCreateToggleAlias: (name: string) => void;
		onCreateSubmit: () => void;

		archiveReason: string;
		onArchiveReasonInput: (value: string) => void;
		onArchiveSubmit: () => void;

		removing: boolean;
		removalSuccess: boolean;
		removeDeleteFiles: boolean;
		removeForceDelete: boolean;
		removeConfirmText: string;
		removeConfirmValid: boolean;
		onRemoveWorkspaceDeleteFilesToggle: (checked: boolean) => void;
		onRemoveWorkspaceForceDeleteToggle: (checked: boolean) => void;
		onRemoveWorkspaceConfirmTextInput: (value: string) => void;
		onRemoveWorkspaceSubmit: () => void;

		removeDeleteWorktree: boolean;
		removeRepoConfirmRequired: boolean;
		removeRepoConfirmText: string;
		removeRepoStatusRefreshing: boolean;
		removeRepoStatus: Repo | null;
		removeRepoConfirmValid: boolean;
		onRemoveRepoDeleteWorktreeToggle: (checked: boolean) => void;
		onRemoveRepoConfirmTextInput: (value: string) => void;
		onRemoveRepoSubmit: () => void;
	}

	const {
		mode,
		loading,

		aliasItems,
		searchQuery,
		filteredAliases,
		selectedAliases,
		getAliasSource,

		renameName,
		onRenameNameInput,
		onRenameSubmit,

		addSource,
		existingRepos,
		addRepoSelectedItems,
		addRepoTotalItems,
		onAddSearchQueryInput,
		onAddSourceInput,
		onAddBrowse,
		onAddToggleAlias,
		onAddRemoveAlias,
		onAddSubmit,

		createWorkspaceName,
		createDescription = '',
		createNameValidationError = null,
		createWorksetLabel = null,
		createSourceInput,
		createDirectRepos,
		createThreadHookRows = [],
		createThreadHooksLoading = false,
		createThreadHooksError = null,
		createCreating = false,
		createRepoProgress = {},
		createHookRuns = [],
		onCreateWorkspaceNameInput,
		onCreateDescriptionInput,
		onCreateSearchQueryInput,
		onCreateSourceInput,
		onCreateAddDirectRepo,
		onCreateRemoveDirectRepo,
		onCreateToggleDirectRepoRegister,
		onCreateToggleAlias,
		onCreateSubmit,

		archiveReason,
		onArchiveReasonInput,
		onArchiveSubmit,

		removing,
		removalSuccess,
		removeDeleteFiles,
		removeForceDelete,
		removeConfirmText,
		removeConfirmValid,
		onRemoveWorkspaceDeleteFilesToggle,
		onRemoveWorkspaceForceDeleteToggle,
		onRemoveWorkspaceConfirmTextInput,
		onRemoveWorkspaceSubmit,

		removeDeleteWorktree,
		removeRepoConfirmRequired,
		removeRepoConfirmText,
		removeRepoStatusRefreshing,
		removeRepoStatus,
		removeRepoConfirmValid,
		onRemoveRepoDeleteWorktreeToggle,
		onRemoveRepoConfirmTextInput,
		onRemoveRepoSubmit,
	}: Props = $props();

	let textInput: HTMLInputElement | null = $state(null);

	onMount(() => {
		if (mode === 'rename' || mode === 'archive') {
			textInput?.focus();
		}
	});
</script>

{#if mode === 'create' || mode === 'create-thread'}
	<WorkspaceActionCreateForm
		{loading}
		modeVariant={mode === 'create-thread' ? 'thread' : 'workset'}
		worksetLabel={createWorksetLabel}
		workspaceName={createWorkspaceName}
		description={createDescription}
		nameValidationError={createNameValidationError}
		{searchQuery}
		sourceInput={createSourceInput}
		directRepos={createDirectRepos}
		threadHookRows={createThreadHookRows}
		threadHooksLoading={createThreadHooksLoading}
		threadHooksError={createThreadHooksError}
		creating={createCreating}
		repoProgress={createRepoProgress}
		hookRuns={createHookRuns}
		{filteredAliases}
		{selectedAliases}
		{getAliasSource}
		onWorkspaceNameInput={onCreateWorkspaceNameInput}
		onDescriptionInput={onCreateDescriptionInput}
		onSearchQueryInput={onCreateSearchQueryInput}
		onSourceInput={onCreateSourceInput}
		onAddDirectRepo={onCreateAddDirectRepo}
		onRemoveDirectRepo={onCreateRemoveDirectRepo}
		onToggleDirectRepoRegister={onCreateToggleDirectRepoRegister}
		onToggleAlias={onCreateToggleAlias}
		onSubmit={onCreateSubmit}
	/>
{:else if mode === 'rename'}
	<div class="form ws-form-stack">
		<label class="field ws-field">
			<span>New name</span>
			<input
				class="ws-field-input"
				bind:this={textInput}
				value={renameName}
				oninput={(event) => onRenameNameInput((event.currentTarget as HTMLInputElement).value)}
				placeholder="acme"
				autocapitalize="off"
				autocorrect="off"
				spellcheck="false"
			/>
		</label>
		<div class="hint ws-hint">Renaming updates config and workset.yaml. Files stay in place.</div>
		<Button variant="primary" onclick={onRenameSubmit} disabled={loading} class="action-btn">
			{loading ? 'Renaming…' : 'Rename'}
		</Button>
	</div>
{:else if mode === 'add-repo'}
	<WorkspaceActionAddRepoForm
		{loading}
		{aliasItems}
		{searchQuery}
		{addSource}
		{filteredAliases}
		{selectedAliases}
		{existingRepos}
		{addRepoSelectedItems}
		{addRepoTotalItems}
		{getAliasSource}
		onSearchQueryInput={onAddSearchQueryInput}
		{onAddSourceInput}
		onBrowse={onAddBrowse}
		onToggleAlias={onAddToggleAlias}
		onRemoveAlias={onAddRemoveAlias}
		onSubmit={onAddSubmit}
	/>
{:else if mode === 'archive'}
	<div class="form ws-form-stack">
		<div class="hint ws-hint">Archiving hides the workspace but keeps files on disk.</div>
		<label class="field ws-field">
			<span>Reason (optional)</span>
			<input
				class="ws-field-input"
				bind:this={textInput}
				value={archiveReason}
				oninput={(event) => onArchiveReasonInput((event.currentTarget as HTMLInputElement).value)}
				placeholder="paused"
				autocapitalize="off"
				autocorrect="off"
				spellcheck="false"
			/>
		</label>
		<Button variant="danger" onclick={onArchiveSubmit} disabled={loading} class="action-btn">
			{loading ? 'Archiving…' : 'Archive'}
		</Button>
	</div>
{:else if mode === 'remove-thread'}
	<WorkspaceActionRemoveWorkspaceForm
		{loading}
		{removing}
		{removalSuccess}
		{removeDeleteFiles}
		{removeForceDelete}
		{removeConfirmText}
		{removeConfirmValid}
		onToggleDeleteFiles={onRemoveWorkspaceDeleteFilesToggle}
		onToggleForceDelete={onRemoveWorkspaceForceDeleteToggle}
		onConfirmTextInput={onRemoveWorkspaceConfirmTextInput}
		onSubmit={onRemoveWorkspaceSubmit}
	/>
{:else if mode === 'remove-repo'}
	<WorkspaceActionRemoveRepoForm
		{loading}
		{removing}
		{removalSuccess}
		{removeDeleteWorktree}
		{removeRepoConfirmRequired}
		{removeRepoConfirmText}
		{removeRepoStatusRefreshing}
		{removeRepoStatus}
		{removeRepoConfirmValid}
		onToggleDeleteWorktree={onRemoveRepoDeleteWorktreeToggle}
		onConfirmTextInput={onRemoveRepoConfirmTextInput}
		onSubmit={onRemoveRepoSubmit}
	/>
{/if}

<style>
	:global(.action-btn) {
		width: 100%;
		margin-top: 8px;
	}
</style>
