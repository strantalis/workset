<script lang="ts">
	import type { RepoDiffSummary, Workspace } from '../../types';
	import { refreshWorkspacesStatus, applyTrackedPullRequest } from '../../state';
	import LocalMergeDrawer from './LocalMergeDrawer.svelte';
	import PrLifecycleDrawer from './PrLifecycleDrawer.svelte';

	interface Props {
		workspace: Workspace;
		selectedRepo: Workspace['repos'][number];
		drawerMode: 'none' | 'local-merge' | 'pr';
		prDiffSummary?: RepoDiffSummary;
		prFileCommentCounts: Map<string, number>;
		closeDrawer: () => void;
	}

	const {
		workspace,
		selectedRepo,
		drawerMode,
		prDiffSummary,
		prFileCommentCounts,
		closeDrawer,
	}: Props = $props();

	const unresolvedThreads = $derived.by(() => {
		let total = 0;
		for (const count of prFileCommentCounts.values()) total += count;
		return total;
	});
</script>

<LocalMergeDrawer
	open={drawerMode === 'local-merge'}
	workspaceId={workspace.id}
	repoId={selectedRepo.id}
	repoName={selectedRepo.name}
	branch={selectedRepo.currentBranch || 'main'}
	baseBranch={selectedRepo.defaultBranch || 'main'}
	onClose={closeDrawer}
	onMerged={() => {
		void refreshWorkspacesStatus(true);
	}}
/>

<PrLifecycleDrawer
	open={drawerMode === 'pr'}
	workspaceId={workspace.id}
	repoId={selectedRepo.id}
	repoName={selectedRepo.name}
	branch={selectedRepo.currentBranch || 'main'}
	baseBranch={selectedRepo.defaultBranch || 'main'}
	trackedPr={selectedRepo.trackedPullRequest ?? null}
	diffStats={prDiffSummary
		? {
				filesChanged: prDiffSummary.files.length,
				additions: prDiffSummary.totalAdded,
				deletions: prDiffSummary.totalRemoved,
			}
		: null}
	{unresolvedThreads}
	onClose={closeDrawer}
	onStatusChanged={() => {}}
	onTrackedPrChanged={(created) => {
		applyTrackedPullRequest(workspace.id, selectedRepo.id, created);
	}}
	onDismissTrackedPr={() => {
		applyTrackedPullRequest(workspace.id, selectedRepo.id, null);
	}}
/>
