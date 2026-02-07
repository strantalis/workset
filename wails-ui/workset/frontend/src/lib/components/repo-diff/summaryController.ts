import type { FileDiffMetadata } from '@pierre/diffs';
import type { RepoDiffFileSummary, RepoDiffSummary, RepoFileDiff } from '../../types';

type SummarySource = 'pr' | 'local';
type BranchDiffRefs = { base: string; head: string };

type SummaryControllerOptions = {
	workspaceId: () => string;
	repoId: () => string;
	repoStatusKnown: () => boolean;
	repoMissing: () => boolean;
	localHasUncommitted: () => boolean;
	selected: () => RepoDiffFileSummary | null;
	selectedSource: () => SummarySource;
	summary: () => RepoDiffSummary | null;
	setSummary: (value: RepoDiffSummary | null) => void;
	setSummaryLoading: (value: boolean) => void;
	setSummaryError: (value: string | null) => void;
	setLocalSummary: (value: RepoDiffSummary | null) => void;
	setSelected: (value: RepoDiffFileSummary | null) => void;
	setSelectedDiff: (value: FileDiffMetadata | null) => void;
	setFileMeta: (value: RepoFileDiff | null) => void;
	setFileError: (value: string | null) => void;
	selectFile: (file: RepoDiffFileSummary, source?: SummarySource) => void;
	useBranchDiff: () => BranchDiffRefs | null;
	fetchRepoDiffSummary: (workspaceId: string, repoId: string) => Promise<RepoDiffSummary>;
	fetchBranchDiffSummary: (
		workspaceId: string,
		repoId: string,
		base: string,
		head: string,
	) => Promise<RepoDiffSummary>;
	applyRepoDiffSummary: (workspaceId: string, repoId: string, summary: RepoDiffSummary) => void;
	formatError: (error: unknown, fallback: string) => string;
};

const findSummaryMatch = (
	data: RepoDiffSummary,
	current: RepoDiffFileSummary | null,
): RepoDiffFileSummary | null => {
	if (!current) return null;
	return (
		data.files.find((file) => file.path === current.path && file.prevPath === current.prevPath) ??
		null
	);
};

export const createSummaryController = (options: SummaryControllerOptions) => {
	let summaryRequest = 0;

	const applySummaryUpdate = (data: RepoDiffSummary, source: SummarySource): void => {
		if (source === 'pr') {
			options.setSummary(data);
			options.setSummaryLoading(false);
			options.setSummaryError(null);
		} else {
			options.setLocalSummary(data);
		}

		if (options.selectedSource() !== source) {
			const currentSummary = options.summary();
			if (
				source === 'local' &&
				(!currentSummary || currentSummary.files.length === 0) &&
				!options.selected() &&
				data.files.length > 0
			) {
				options.selectFile(data.files[0], 'local');
			}
			return;
		}

		if (data.files.length === 0) {
			options.setSelected(null);
			options.setSelectedDiff(null);
			options.setFileMeta(null);
			options.setFileError(null);
			return;
		}

		const match = findSummaryMatch(data, options.selected());
		if (match) {
			options.selectFile(match, source);
			return;
		}
		options.selectFile(data.files[0], source);
	};

	const loadLocalSummary = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId) {
			options.setLocalSummary(null);
			return;
		}

		if (!options.localHasUncommitted()) {
			options.setLocalSummary(null);
			return;
		}

		try {
			const data = await options.fetchRepoDiffSummary(options.workspaceId(), currentRepoId);
			applySummaryUpdate(data, 'local');
			options.applyRepoDiffSummary(options.workspaceId(), currentRepoId, data);
		} catch {
			options.setLocalSummary(null);
		}
	};

	const loadSummary = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId) {
			options.setSummary(null);
			options.setSummaryLoading(false);
			return;
		}

		options.setSummaryLoading(true);
		options.setSummaryError(null);
		options.setSummary(null);
		options.setSelected(null);
		options.setSelectedDiff(null);
		options.setFileMeta(null);
		options.setFileError(null);

		if (options.repoStatusKnown() !== false && options.repoMissing()) {
			options.setSummaryError('Repo is missing on disk. Restore it to view the diff.');
			options.setSummaryLoading(false);
			return;
		}

		const requestId = ++summaryRequest;
		try {
			const branchRefs = options.useBranchDiff();
			const data = branchRefs
				? await options.fetchBranchDiffSummary(
						options.workspaceId(),
						currentRepoId,
						branchRefs.base,
						branchRefs.head,
					)
				: await options.fetchRepoDiffSummary(options.workspaceId(), currentRepoId);
			if (requestId !== summaryRequest) return;
			options.setSummary(data);
			if (data.files.length > 0) {
				options.selectFile(data.files[0]);
			}
		} catch (error) {
			if (requestId !== summaryRequest) return;
			options.setSummaryError(options.formatError(error, 'Failed to load diff summary.'));
		} finally {
			if (requestId === summaryRequest) {
				options.setSummaryLoading(false);
			}
		}
	};

	return {
		applySummaryUpdate,
		loadLocalSummary,
		loadSummary,
	};
};
