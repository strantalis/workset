import type { RepoDiffFileSummary, RepoDiffSummary, Workspace } from '../../types';

export function handleEditorSaveKeydown(
	event: KeyboardEvent,
	editMode: boolean,
	onSave: () => void,
): void {
	if ((event.metaKey || event.ctrlKey) && event.key === 's' && editMode) {
		event.preventDefault();
		onSave();
	}
}

export function findChangedFilePath(
	files: RepoDiffFileSummary[],
	selectedIndex: number,
	delta: number,
): string | null {
	if (files.length === 0 || selectedIndex < 0) return null;
	const nextIndex = Math.max(0, Math.min(files.length - 1, selectedIndex + delta));
	if (nextIndex === selectedIndex) return null;
	return files[nextIndex]?.path ?? null;
}

export function maybeLoadBranchDataForRepo(
	workspace: Workspace | null,
	workspaceId: string,
	repoId: string,
	loadBranchDiff: (
		workspaceId: string,
		repoId: string,
		baseBranch: string,
		headBranch: string,
	) => Promise<void>,
	loadAllPrReviewComments: (workspaceId: string, repoId: string) => Promise<void>,
): void {
	const repo = workspace?.repos.find((entry) => entry.id === repoId);
	const trackedPr = repo?.trackedPullRequest;
	if (!trackedPr || trackedPr.state.toLowerCase() !== 'open') return;
	void loadBranchDiff(workspaceId, repoId, trackedPr.baseBranch, trackedPr.headBranch);
	void loadAllPrReviewComments(workspaceId, repoId);
}

export function isRepoFileChanged(
	changedFileSet: Set<string>,
	repoId: string,
	path: string,
): boolean {
	return changedFileSet.has(`${repoId}:${path}`);
}

export function getRepoFileDiffInfo(
	repoDiffMap: Map<string, RepoDiffSummary>,
	repoId: string,
	path: string,
): RepoDiffFileSummary | undefined {
	return repoDiffMap.get(repoId)?.files.find((file) => file.path === path);
}

export const ignoreError = (): void => {};
