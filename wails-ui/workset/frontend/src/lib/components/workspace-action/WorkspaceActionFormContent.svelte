<script lang="ts">
	import { onMount } from 'svelte';
	import type { Alias, GroupSummary, Repo } from '../../types';
	import type {
		ExistingRepoContext,
		WorkspaceActionAddRepoSelectedItem,
		WorkspaceActionDirectRepo,
		WorkspaceActionPreviewItem,
	} from '../../services/workspaceActionContextService';
	import Button from '../ui/Button.svelte';
	import WorkspaceActionAddRepoForm from './WorkspaceActionAddRepoForm.svelte';
	import WorkspaceActionCreateForm from './WorkspaceActionCreateForm.svelte';
	import WorkspaceActionRemoveRepoForm from './WorkspaceActionRemoveRepoForm.svelte';
	import WorkspaceActionRemoveWorkspaceForm from './WorkspaceActionRemoveWorkspaceForm.svelte';

	type Mode =
		| 'create'
		| 'rename'
		| 'add-repo'
		| 'archive'
		| 'remove-workspace'
		| 'remove-repo'
		| null;
	type CreateTab = 'direct' | 'repos' | 'groups';

	interface Props {
		mode: Mode;
		loading: boolean;

		activeTab: CreateTab;
		aliasItems: Alias[];
		groupItems: GroupSummary[];
		searchQuery: string;
		primaryInput: string;
		directRepos: WorkspaceActionDirectRepo[];
		filteredAliases: Alias[];
		filteredGroups: GroupSummary[];
		selectedAliases: Set<string>;
		selectedGroups: Set<string>;
		expandedGroups: Set<string>;
		groupDetails: Map<string, string[]>;
		selectedItems: WorkspaceActionPreviewItem[];
		totalRepos: number;
		customizeName: string;
		generatedName: string | null;
		alternatives: string[];
		finalName: string;
		getAliasSource: (alias: Alias) => string;
		deriveRepoName: (source: string) => string | null;
		isRepoSource: (source: string) => boolean;
		onCreateTabChange: (tab: CreateTab) => void;
		onCreatePrimaryInput: (value: string) => void;
		onCreateSearchQueryInput: (value: string) => void;
		onCreateAddDirectRepo: () => void;
		onCreateBrowsePrimary: () => void;
		onCreateToggleDirectRepoRegister: (url: string) => void;
		onCreateRemoveDirectRepo: (url: string) => void;
		onCreateToggleAlias: (name: string) => void;
		onCreateToggleGroup: (name: string) => void;
		onCreateToggleGroupExpand: (name: string) => void;
		onCreateRemoveAlias: (name: string) => void;
		onCreateRemoveGroup: (name: string) => void;
		onCreateCustomizeNameInput: (value: string) => void;
		onCreateSelectAlternative: (name: string) => void;
		onCreateSubmit: () => void;

		renameName: string;
		onRenameNameInput: (value: string) => void;
		onRenameSubmit: () => void;

		addSource: string;
		existingRepos: ExistingRepoContext[];
		addRepoSelectedItems: WorkspaceActionAddRepoSelectedItem[];
		addRepoTotalItems: number;
		onAddTabChange: (tab: CreateTab) => void;
		onAddSearchQueryInput: (value: string) => void;
		onAddSourceInput: (value: string) => void;
		onAddBrowse: () => void;
		onAddToggleAlias: (name: string) => void;
		onAddToggleGroup: (name: string) => void;
		onAddToggleGroupExpand: (name: string) => void;
		onAddRemoveAlias: (name: string) => void;
		onAddRemoveGroup: (name: string) => void;
		onAddSubmit: () => void;

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

		activeTab,
		aliasItems,
		groupItems,
		searchQuery,
		primaryInput,
		directRepos,
		filteredAliases,
		filteredGroups,
		selectedAliases,
		selectedGroups,
		expandedGroups,
		groupDetails,
		selectedItems,
		totalRepos,
		customizeName,
		generatedName,
		alternatives,
		finalName,
		getAliasSource,
		deriveRepoName,
		isRepoSource,
		onCreateTabChange,
		onCreatePrimaryInput,
		onCreateSearchQueryInput,
		onCreateAddDirectRepo,
		onCreateBrowsePrimary,
		onCreateToggleDirectRepoRegister,
		onCreateRemoveDirectRepo,
		onCreateToggleAlias,
		onCreateToggleGroup,
		onCreateToggleGroupExpand,
		onCreateRemoveAlias,
		onCreateRemoveGroup,
		onCreateCustomizeNameInput,
		onCreateSelectAlternative,
		onCreateSubmit,

		renameName,
		onRenameNameInput,
		onRenameSubmit,

		addSource,
		existingRepos,
		addRepoSelectedItems,
		addRepoTotalItems,
		onAddTabChange,
		onAddSearchQueryInput,
		onAddSourceInput,
		onAddBrowse,
		onAddToggleAlias,
		onAddToggleGroup,
		onAddToggleGroupExpand,
		onAddRemoveAlias,
		onAddRemoveGroup,
		onAddSubmit,

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

{#if mode === 'create'}
	<WorkspaceActionCreateForm
		{loading}
		{activeTab}
		{aliasItems}
		{groupItems}
		{searchQuery}
		{primaryInput}
		{directRepos}
		{filteredAliases}
		{filteredGroups}
		{selectedAliases}
		{selectedGroups}
		{expandedGroups}
		{groupDetails}
		{selectedItems}
		{totalRepos}
		{customizeName}
		{generatedName}
		{alternatives}
		{finalName}
		{getAliasSource}
		{deriveRepoName}
		{isRepoSource}
		onTabChange={onCreateTabChange}
		onPrimaryInput={onCreatePrimaryInput}
		onSearchQueryInput={onCreateSearchQueryInput}
		onAddDirectRepo={onCreateAddDirectRepo}
		onBrowsePrimary={onCreateBrowsePrimary}
		onToggleDirectRepoRegister={onCreateToggleDirectRepoRegister}
		onRemoveDirectRepo={onCreateRemoveDirectRepo}
		onToggleAlias={onCreateToggleAlias}
		onToggleGroup={onCreateToggleGroup}
		onToggleGroupExpand={onCreateToggleGroupExpand}
		onRemoveAlias={onCreateRemoveAlias}
		onRemoveGroup={onCreateRemoveGroup}
		onCustomizeNameInput={onCreateCustomizeNameInput}
		onSelectAlternative={onCreateSelectAlternative}
		onSubmit={onCreateSubmit}
	/>
{:else if mode === 'rename'}
	<div class="form">
		<label class="field">
			<span>New name</span>
			<input
				bind:this={textInput}
				value={renameName}
				oninput={(event) => onRenameNameInput((event.currentTarget as HTMLInputElement).value)}
				placeholder="acme"
				autocapitalize="off"
				autocorrect="off"
				spellcheck="false"
			/>
		</label>
		<div class="hint">Renaming updates config and workset.yaml. Files stay in place.</div>
		<Button variant="primary" onclick={onRenameSubmit} disabled={loading} class="action-btn">
			{loading ? 'Renaming…' : 'Rename'}
		</Button>
	</div>
{:else if mode === 'add-repo'}
	<WorkspaceActionAddRepoForm
		{loading}
		{activeTab}
		{aliasItems}
		{groupItems}
		{searchQuery}
		{addSource}
		{filteredAliases}
		{filteredGroups}
		{selectedAliases}
		{selectedGroups}
		{expandedGroups}
		{groupDetails}
		{existingRepos}
		{addRepoSelectedItems}
		{addRepoTotalItems}
		{getAliasSource}
		onTabChange={onAddTabChange}
		onSearchQueryInput={onAddSearchQueryInput}
		{onAddSourceInput}
		onBrowse={onAddBrowse}
		onToggleAlias={onAddToggleAlias}
		onToggleGroup={onAddToggleGroup}
		onToggleGroupExpand={onAddToggleGroupExpand}
		onRemoveAlias={onAddRemoveAlias}
		onRemoveGroup={onAddRemoveGroup}
		onSubmit={onAddSubmit}
	/>
{:else if mode === 'archive'}
	<div class="form">
		<div class="hint">Archiving hides the workspace but keeps files on disk.</div>
		<label class="field">
			<span>Reason (optional)</span>
			<input
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
{:else if mode === 'remove-workspace'}
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
	.form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.field input {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--text);
		padding: 8px 10px;
		font-size: var(--text-md);
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.field input:focus {
		background: rgba(255, 255, 255, 0.04);
	}

	:global(.action-btn) {
		width: 100%;
		margin-top: 8px;
	}

	.hint {
		font-size: var(--text-sm);
		color: var(--muted);
	}
</style>
