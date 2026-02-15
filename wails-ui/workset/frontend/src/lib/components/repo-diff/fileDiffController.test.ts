import type { FileDiffMetadata, ParsedPatch } from '@pierre/diffs';
import { describe, expect, it, vi } from 'vitest';
import type { RepoDiffFileSummary, RepoFileDiff } from '../../types';
import {
	createRepoDiffFileController,
	type BranchDiffRefs,
	type SummarySource,
} from './fileDiffController';

type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
	reject: (reason?: unknown) => void;
};

const createDeferred = <T>(): Deferred<T> => {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
};

const buildResponse = (patch: string): RepoFileDiff => ({
	patch,
	truncated: false,
	totalBytes: patch.length,
	totalLines: 1,
});

const baseFile: RepoDiffFileSummary = {
	path: 'src/main.ts',
	added: 2,
	removed: 1,
	status: 'modified',
};

const createSetup = (
	branchRefs: BranchDiffRefs | null = { base: 'origin/main', head: 'feature' },
) => {
	const state: {
		selected: RepoDiffFileSummary | null;
		selectedSource: SummarySource;
		selectedDiff: FileDiffMetadata | null;
		fileMeta: RepoFileDiff | null;
		fileLoading: boolean;
		fileError: string | null;
	} = {
		selected: null,
		selectedSource: 'pr',
		selectedDiff: null,
		fileMeta: null,
		fileLoading: false,
		fileError: null,
	};

	const frames: FrameRequestCallback[] = [];
	const parsePatchFiles = vi.fn((patch: string) => {
		const diff = { patch } as unknown as FileDiffMetadata;
		return [{ files: [diff] } as unknown as ParsedPatch];
	});
	const fetchRepoFileDiff = vi.fn(async () => buildResponse('repo-patch'));
	const fetchBranchFileDiff = vi.fn(async () => buildResponse('branch-patch'));
	const ensureRenderer = vi.fn(async () => undefined);
	const renderDiff = vi.fn();
	const ensureBranchRefsLoaded = vi.fn(async () => {});

	const controller = createRepoDiffFileController({
		workspaceId: () => 'ws-1',
		repoId: () => 'repo-1',
		selectedSource: () => state.selectedSource,
		useBranchDiff: () => branchRefs,
		ensureBranchRefsLoaded,
		setSelected: (value) => {
			state.selected = value;
		},
		setSelectedSource: (value) => {
			state.selectedSource = value;
		},
		setSelectedDiff: (value) => {
			state.selectedDiff = value;
		},
		setFileMeta: (value) => {
			state.fileMeta = value;
		},
		setFileLoading: (value) => {
			state.fileLoading = value;
		},
		setFileError: (value) => {
			state.fileError = value;
		},
		ensureRenderer,
		getDiffModule: () => ({ parsePatchFiles }),
		getRendererError: () => null,
		fetchRepoFileDiff,
		fetchBranchFileDiff,
		formatError: (error, fallback) => (error instanceof Error ? error.message : fallback),
		requestAnimationFrame: (callback) => {
			frames.push(callback);
			return frames.length;
		},
		renderDiff,
	});

	return {
		state,
		frames,
		parsePatchFiles,
		fetchRepoFileDiff,
		fetchBranchFileDiff,
		ensureRenderer,
		ensureBranchRefsLoaded,
		renderDiff,
		controller,
	};
};

const createSetupWithCustomBranches = (
	getBranchRefs: () => BranchDiffRefs | null,
	ensureBranchRefsLoaded: () => Promise<void>,
) => {
	const state: {
		selected: RepoDiffFileSummary | null;
		selectedSource: SummarySource;
		selectedDiff: FileDiffMetadata | null;
		fileMeta: RepoFileDiff | null;
		fileLoading: boolean;
		fileError: string | null;
	} = {
		selected: null,
		selectedSource: 'pr',
		selectedDiff: null,
		fileMeta: null,
		fileLoading: false,
		fileError: null,
	};

	const frames: FrameRequestCallback[] = [];
	const parsePatchFiles = vi.fn((patch: string) => {
		const diff = { patch } as unknown as FileDiffMetadata;
		return [{ files: [diff] } as unknown as ParsedPatch];
	});
	const fetchRepoFileDiff = vi.fn(async () => buildResponse('repo-patch'));
	const fetchBranchFileDiff = vi.fn(async () => buildResponse('branch-patch'));
	const ensureRenderer = vi.fn(async () => undefined);
	const renderDiff = vi.fn();

	const controller = createRepoDiffFileController({
		workspaceId: () => 'ws-1',
		repoId: () => 'repo-1',
		selectedSource: () => state.selectedSource,
		useBranchDiff: getBranchRefs,
		ensureBranchRefsLoaded,
		setSelected: (value) => {
			state.selected = value;
		},
		setSelectedSource: (value) => {
			state.selectedSource = value;
		},
		setSelectedDiff: (value) => {
			state.selectedDiff = value;
		},
		setFileMeta: (value) => {
			state.fileMeta = value;
		},
		setFileLoading: (value) => {
			state.fileLoading = value;
		},
		setFileError: (value) => {
			state.fileError = value;
		},
		ensureRenderer,
		getDiffModule: () => ({ parsePatchFiles }),
		getRendererError: () => null,
		fetchRepoFileDiff,
		fetchBranchFileDiff,
		formatError: (error, fallback) => (error instanceof Error ? error.message : fallback),
		requestAnimationFrame: (callback) => {
			frames.push(callback);
			return frames.length;
		},
		renderDiff,
	});

	return {
		state,
		frames,
		parsePatchFiles,
		fetchRepoFileDiff,
		fetchBranchFileDiff,
		ensureRenderer,
		ensureBranchRefsLoaded,
		renderDiff,
		controller,
	};
};

describe('fileDiffController', () => {
	it('fetches branch diff for PR files and repo diff for local files', async () => {
		const setup = createSetup();

		setup.state.selectedSource = 'pr';
		await setup.controller.loadFileDiff(baseFile);
		expect(setup.fetchBranchFileDiff).toHaveBeenCalledWith(
			'ws-1',
			'repo-1',
			'origin/main',
			'feature',
			baseFile.path,
			'',
		);
		expect(setup.fetchRepoFileDiff).not.toHaveBeenCalled();

		setup.fetchBranchFileDiff.mockClear();
		setup.fetchRepoFileDiff.mockClear();

		setup.state.selectedSource = 'local';
		await setup.controller.loadFileDiff(baseFile);
		expect(setup.fetchRepoFileDiff).toHaveBeenCalledWith(
			'ws-1',
			'repo-1',
			baseFile.path,
			'',
			baseFile.status,
		);
		expect(setup.fetchBranchFileDiff).not.toHaveBeenCalled();
	});

	it('ignores stale diff responses when a newer file request is active', async () => {
		const setup = createSetup(null);
		const first = createDeferred<RepoFileDiff>();
		const second = createDeferred<RepoFileDiff>();

		setup.fetchRepoFileDiff
			.mockImplementationOnce(async () => first.promise)
			.mockImplementationOnce(async () => second.promise);

		setup.state.selectedSource = 'local';

		const firstLoad = setup.controller.loadFileDiff({ ...baseFile, path: 'src/first.ts' });
		const secondLoad = setup.controller.loadFileDiff({ ...baseFile, path: 'src/second.ts' });

		first.resolve(buildResponse('first-patch'));
		await firstLoad;

		expect(setup.state.fileMeta).toBeNull();
		expect(setup.state.selectedDiff).toBeNull();
		expect(setup.parsePatchFiles).not.toHaveBeenCalled();

		second.resolve(buildResponse('second-patch'));
		await secondLoad;

		expect(setup.state.fileMeta?.patch).toBe('second-patch');
		expect((setup.state.selectedDiff as unknown as { patch: string }).patch).toBe('second-patch');
		expect(setup.parsePatchFiles).toHaveBeenCalledTimes(1);
		expect(setup.parsePatchFiles).toHaveBeenCalledWith('second-patch');
		expect(setup.state.fileLoading).toBe(false);
	});

	it('deduplicates render queue callbacks per animation frame', () => {
		const setup = createSetup();

		setup.controller.queueRenderDiff();
		setup.controller.queueRenderDiff();

		expect(setup.frames).toHaveLength(1);
		expect(setup.renderDiff).not.toHaveBeenCalled();

		setup.frames[0](0);
		expect(setup.renderDiff).toHaveBeenCalledTimes(1);

		setup.controller.queueRenderDiff();
		expect(setup.frames).toHaveLength(2);
	});

	it('calls ensureBranchRefsLoaded when selecting PR file without branch refs', async () => {
		const setup = createSetup(null);

		await setup.controller.selectFile(baseFile, 'pr');

		expect(setup.ensureBranchRefsLoaded).toHaveBeenCalled();
		expect(setup.state.selectedSource).toBe('pr');
		expect(setup.state.selected).toEqual(baseFile);
	});

	it('loads repo diff when branch refs remain missing for PR selection', async () => {
		const setup = createSetup(null);

		await setup.controller.selectFile(baseFile, 'pr');

		expect(setup.fetchRepoFileDiff).toHaveBeenCalledWith(
			'ws-1',
			'repo-1',
			baseFile.path,
			'',
			baseFile.status,
		);
		expect(setup.fetchBranchFileDiff).not.toHaveBeenCalled();
	});

	it('selects file after ensureBranchRefsLoaded makes branch refs available', async () => {
		let branchRefValue: BranchDiffRefs | null = null;
		const getBranchRefs = () => branchRefValue;
		const ensureBranchRefsLoaded = vi.fn(async () => {
			branchRefValue = { base: 'origin/main', head: 'feature' };
		});
		const setup = createSetupWithCustomBranches(getBranchRefs, ensureBranchRefsLoaded);

		await setup.controller.selectFile(baseFile, 'pr');

		expect(setup.ensureBranchRefsLoaded).toHaveBeenCalled();
		expect(setup.state.selected).toEqual(baseFile);
		expect(setup.state.selectedSource).toBe('pr');
	});

	it('does not call ensureBranchRefsLoaded when selecting local file', async () => {
		const setup = createSetup(null);

		await setup.controller.selectFile(baseFile, 'local');

		expect(setup.ensureBranchRefsLoaded).not.toHaveBeenCalled();
		expect(setup.state.selectedSource).toBe('local');
	});

	it('does not call ensureBranchRefsLoaded when selecting PR file with branch refs available', async () => {
		const setup = createSetup({ base: 'origin/main', head: 'feature' });

		await setup.controller.selectFile(baseFile, 'pr');

		expect(setup.ensureBranchRefsLoaded).not.toHaveBeenCalled();
		expect(setup.state.selectedSource).toBe('pr');
	});
});
