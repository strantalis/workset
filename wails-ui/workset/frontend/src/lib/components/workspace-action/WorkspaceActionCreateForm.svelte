<script lang="ts">
	import type { Alias, GroupSummary } from '../../types';
	import type {
		WorkspaceActionDirectRepo,
		WorkspaceActionPreviewItem,
	} from '../../services/workspaceActionContextService';
	import WorkspaceActionCreateDirectTab from './WorkspaceActionCreateDirectTab.svelte';
	import WorkspaceActionCreateGroupsTab from './WorkspaceActionCreateGroupsTab.svelte';
	import WorkspaceActionCreateReposTab from './WorkspaceActionCreateReposTab.svelte';
	import WorkspaceActionCreateSummaryPanel from './WorkspaceActionCreateSummaryPanel.svelte';
	import WorkspaceActionSelectionTabs, {
		type WorkspaceActionSelectionTab as CreateTab,
	} from './WorkspaceActionSelectionTabs.svelte';

	interface Props {
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
		onTabChange: (tab: CreateTab) => void;
		onPrimaryInput: (value: string) => void;
		onSearchQueryInput: (value: string) => void;
		onAddDirectRepo: () => void;
		onBrowsePrimary: () => void;
		onToggleDirectRepoRegister: (url: string) => void;
		onRemoveDirectRepo: (url: string) => void;
		onToggleAlias: (name: string) => void;
		onToggleGroup: (name: string) => void;
		onToggleGroupExpand: (name: string) => void;
		onRemoveAlias: (name: string) => void;
		onRemoveGroup: (name: string) => void;
		onCustomizeNameInput: (value: string) => void;
		onSelectAlternative: (name: string) => void;
		onSubmit: () => void;
	}

	const {
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
		onTabChange,
		onPrimaryInput,
		onSearchQueryInput,
		onAddDirectRepo,
		onBrowsePrimary,
		onToggleDirectRepoRegister,
		onRemoveDirectRepo,
		onToggleAlias,
		onToggleGroup,
		onToggleGroupExpand,
		onRemoveAlias,
		onRemoveGroup,
		onCustomizeNameInput,
		onSelectAlternative,
		onSubmit,
	}: Props = $props();
</script>

<div class="form create-two-column">
	<div class="column-left">
		<WorkspaceActionSelectionTabs
			{activeTab}
			aliasCount={aliasItems.length}
			groupCount={groupItems.length}
			{onTabChange}
		/>

		<div class="selection-area">
			{#if activeTab === 'direct'}
				<WorkspaceActionCreateDirectTab
					{primaryInput}
					{directRepos}
					{deriveRepoName}
					{isRepoSource}
					{onPrimaryInput}
					{onAddDirectRepo}
					{onBrowsePrimary}
					{onToggleDirectRepoRegister}
					{onRemoveDirectRepo}
				/>
			{:else if activeTab === 'repos'}
				<WorkspaceActionCreateReposTab
					{searchQuery}
					{filteredAliases}
					{selectedAliases}
					{getAliasSource}
					{onSearchQueryInput}
					{onToggleAlias}
				/>
			{:else if activeTab === 'groups'}
				<WorkspaceActionCreateGroupsTab
					{searchQuery}
					{filteredGroups}
					{selectedGroups}
					{expandedGroups}
					{groupDetails}
					{onSearchQueryInput}
					{onToggleGroup}
					{onToggleGroupExpand}
				/>
			{/if}
		</div>

		{#if aliasItems.length === 0 && groupItems.length === 0}
			<div class="hint">No registered repos or groups configured. Add them in Settings.</div>
		{/if}
	</div>

	<div class="column-right">
		<WorkspaceActionCreateSummaryPanel
			{selectedItems}
			{totalRepos}
			{customizeName}
			{generatedName}
			{alternatives}
			{finalName}
			{loading}
			{onCustomizeNameInput}
			{onSelectAlternative}
			{onSubmit}
			{onRemoveDirectRepo}
			{onRemoveAlias}
			{onRemoveGroup}
		/>
	</div>
</div>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.hint {
		font-size: 12px;
		color: var(--muted);
	}

	.form.create-two-column {
		display: grid;
		grid-template-columns: 1fr 280px;
		gap: 16px;
		max-height: 80vh;
		min-height: 500px;
		overflow: hidden;
	}

	.column-left {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: hidden;
	}

	.column-right {
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow: hidden;
	}

	.selection-area {
		display: flex;
		flex-direction: column;
		gap: 8px;
		flex: 1;
		min-height: 0;
	}
</style>
