import type {
	PullRequestSummary,
	RepoDiffFileSummary,
	RepoDiffSummary,
	Workspace,
} from '../../types';
import { logTerminalDebug } from '../../api/terminal-layout';

type RepoViewSelection = {
	workspaceId: string | null;
	repoId: string | null;
	path: string | null;
};

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
	loadBranchDiff: (workspaceId: string, repoId: string, pr: PullRequestSummary) => Promise<void>,
	loadAllPrReviewComments: (workspaceId: string, repoId: string) => Promise<void>,
): void {
	const repo = workspace?.repos.find((entry) => entry.id === repoId);
	const trackedPr = repo?.trackedPullRequest;
	if (!trackedPr || trackedPr.state.toLowerCase() !== 'open') return;
	void loadBranchDiff(workspaceId, repoId, trackedPr);
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

export function logUnifiedRepoEditorEvent(
	workspaceId: string | null,
	repoId: string | null,
	path: string | null,
	event: string,
	details: Record<string, unknown>,
): void {
	if (!workspaceId) return;
	void logTerminalDebug(
		workspaceId,
		'__repo_editor__',
		event,
		JSON.stringify({ repoId, path, ...details }),
	);
}

export function logRepoFileSaveStarted(selection: RepoViewSelection, sizeBytes: number): void {
	logUnifiedRepoEditorEvent(
		selection.workspaceId,
		selection.repoId,
		selection.path,
		'editor_save_start',
		{
			sizeBytes,
		},
	);
}

export function logRepoFileSaveSucceeded(selection: RepoViewSelection, sizeBytes: number): void {
	logUnifiedRepoEditorEvent(
		selection.workspaceId,
		selection.repoId,
		selection.path,
		'editor_save_success',
		{ sizeBytes },
	);
}

export function logRepoFileSaveFailed(selection: RepoViewSelection, message: string): void {
	logUnifiedRepoEditorEvent(
		selection.workspaceId,
		selection.repoId,
		selection.path,
		'editor_save_error',
		{
			message,
		},
	);
}

export function logRepoFileRefreshRequested(
	selection: RepoViewSelection,
	details: {
		blockedByEdit: boolean;
		editMode: boolean;
		hasEditedContent: boolean;
		refreshVersion: number;
	},
): void {
	logUnifiedRepoEditorEvent(
		selection.workspaceId,
		selection.repoId,
		selection.path,
		'repo_file_refresh_requested',
		details,
	);
}

export function logRepoFileRefreshStarted(
	selection: RepoViewSelection,
	refreshVersion: number,
): void {
	logUnifiedRepoEditorEvent(
		selection.workspaceId,
		selection.repoId,
		selection.path,
		'repo_file_refresh_started',
		{ refreshVersion },
	);
}

export function logRepoFileSelected(
	selection: RepoViewSelection,
	details: {
		sameFile: boolean;
		previousRepoId: string | null;
		previousPath: string | null;
		editMode: boolean;
		previewMode: boolean;
	},
): void {
	logUnifiedRepoEditorEvent(
		selection.workspaceId,
		selection.repoId,
		selection.path,
		'repo_file_select',
		details,
	);
}

export function logRepoFileLoadRequest(
	selection: RepoViewSelection,
	details: {
		mode: 'edit' | 'preview' | 'diff' | 'view';
		refreshVersion: number;
		hasDiffFile: boolean;
		diffSignature: string;
	},
): void {
	logUnifiedRepoEditorEvent(
		selection.workspaceId,
		selection.repoId,
		selection.path,
		'repo_file_load_request',
		details,
	);
}

export function logRepoDiffSummarySelected(
	selection: RepoViewSelection,
	event: 'repo_diff_local_summary_selected' | 'repo_diff_branch_summary_selected',
	fileCount: number,
): void {
	logUnifiedRepoEditorEvent(selection.workspaceId, selection.repoId, selection.path, event, {
		fileCount,
	});
}

export const ignoreError = (): void => {};
