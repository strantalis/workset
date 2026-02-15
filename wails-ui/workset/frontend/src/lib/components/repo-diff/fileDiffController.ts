import type { FileDiffMetadata, ParsedPatch } from '@pierre/diffs';
import type { RepoDiffFileSummary, RepoFileDiff } from '../../types';

export type SummarySource = 'pr' | 'local';
export type BranchDiffRefs = { base: string; head: string };

type PatchParserModule = {
	parsePatchFiles: (patch: string) => ParsedPatch[];
};

type RepoDiffFileControllerOptions = {
	workspaceId: () => string;
	repoId: () => string;
	selectedSource: () => SummarySource;
	useBranchDiff: () => BranchDiffRefs | null;
	ensureBranchRefsLoaded: () => Promise<void>;
	setSelected: (value: RepoDiffFileSummary | null) => void;
	setSelectedSource: (value: SummarySource) => void;
	setSelectedDiff: (value: FileDiffMetadata | null) => void;
	setFileMeta: (value: RepoFileDiff | null) => void;
	setFileLoading: (value: boolean) => void;
	setFileError: (value: string | null) => void;
	ensureRenderer: () => Promise<void>;
	getDiffModule: () => PatchParserModule | null;
	getRendererError: () => string | null;
	fetchRepoFileDiff: (
		workspaceId: string,
		repoId: string,
		path: string,
		prevPath: string,
		status: string,
	) => Promise<RepoFileDiff>;
	fetchBranchFileDiff: (
		workspaceId: string,
		repoId: string,
		base: string,
		head: string,
		path: string,
		prevPath: string,
	) => Promise<RepoFileDiff>;
	formatError: (error: unknown, fallback: string) => string;
	requestAnimationFrame: (callback: FrameRequestCallback) => number;
	renderDiff: () => void;
};

export const createRepoDiffFileController = (options: RepoDiffFileControllerOptions) => {
	let fileRequest = 0;
	let renderQueued = false;

	const queueRenderDiff = (): void => {
		if (renderQueued) return;
		renderQueued = true;
		options.requestAnimationFrame(() => {
			renderQueued = false;
			options.renderDiff();
		});
	};

	const loadFileDiff = async (file: RepoDiffFileSummary): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId) return;

		options.setFileLoading(true);
		options.setFileError(null);
		options.setFileMeta(null);
		options.setSelectedDiff(null);
		const requestId = ++fileRequest;

		if (file.binary) {
			options.setFileError('Binary files are not rendered yet.');
			options.setFileLoading(false);
			return;
		}

		try {
			const branchRefs = options.selectedSource() === 'local' ? null : options.useBranchDiff();
			const response = branchRefs
				? await options.fetchBranchFileDiff(
						options.workspaceId(),
						currentRepoId,
						branchRefs.base,
						branchRefs.head,
						file.path,
						file.prevPath ?? '',
					)
				: await options.fetchRepoFileDiff(
						options.workspaceId(),
						currentRepoId,
						file.path,
						file.prevPath ?? '',
						file.status,
					);
			if (requestId !== fileRequest) return;

			options.setFileMeta(response);
			if (response.truncated) {
				const kb = Math.max(1, Math.round(response.totalBytes / 1024));
				options.setFileError(`Diff too large (${response.totalLines} lines, ${kb} KB).`);
				return;
			}
			if (!response.patch.trim()) {
				options.setFileError('No diff available for this file.');
				return;
			}

			await options.ensureRenderer();
			const diffModule = options.getDiffModule();
			if (!diffModule) {
				options.setFileError(options.getRendererError() ?? 'Diff renderer unavailable.');
				return;
			}

			const parsed = diffModule.parsePatchFiles(response.patch);
			const fileDiff = parsed[0]?.files?.[0] ?? null;
			if (!fileDiff) {
				options.setFileError('Unable to parse diff content.');
				return;
			}
			options.setSelectedDiff(fileDiff);
		} catch (error) {
			if (requestId !== fileRequest) return;
			options.setFileError(options.formatError(error, 'Failed to load file diff.'));
		} finally {
			if (requestId === fileRequest) {
				options.setFileLoading(false);
			}
		}
	};

	const selectFile = async (file: RepoDiffFileSummary, source: SummarySource = 'pr'): Promise<void> => {
		if (source === 'pr') {
			const branchRefs = options.useBranchDiff();
			if (!branchRefs) {
				await options.ensureBranchRefsLoaded();
			}
		}

		options.setSelected(file);
		options.setSelectedSource(source);
		void loadFileDiff(file);
	};

	return {
		queueRenderDiff,
		loadFileDiff,
		selectFile,
	};
};
