import type { RemoteInfo, RepoDiffFileSummary, RepoDiffSummary } from '../../types';
import { describe, expect, it, vi } from 'vitest';
import { createSummarySourceController } from './summarySourceController';

const fileA: RepoDiffFileSummary = {
	path: 'src/a.ts',
	added: 1,
	removed: 0,
	status: 'modified',
};

const summaryA: RepoDiffSummary = {
	files: [fileA],
	totalAdded: 1,
	totalRemoved: 0,
};

const flushPromises = async (): Promise<void> => {
	await Promise.resolve();
	await Promise.resolve();
};

const createSetup = () => {
	const state: {
		workspaceId: string;
		repoId: string;
		repoStatusKnown: boolean;
		repoMissing: boolean;
		localHasUncommitted: boolean;
		remotes: RemoteInfo[];
		pullRequest: {
			baseRepo?: string;
			baseBranch?: string;
			headRepo?: string;
			headBranch?: string;
		} | null;
		selected: RepoDiffFileSummary | null;
		selectedSource: 'pr' | 'local';
		summary: RepoDiffSummary | null;
		summaryLoading: boolean;
		summaryError: string | null;
		localSummary: RepoDiffSummary | null;
	} = {
		workspaceId: 'ws-1',
		repoId: 'repo-1',
		repoStatusKnown: true,
		repoMissing: false,
		localHasUncommitted: false,
		remotes: [{ name: 'origin', owner: 'acme', repo: 'workset' }],
		pullRequest: null,
		selected: null,
		selectedSource: 'pr',
		summary: null,
		summaryLoading: false,
		summaryError: null,
		localSummary: null,
	};

	const fetchRepoDiffSummary = vi.fn(async () => summaryA);
	const fetchBranchDiffSummary = vi.fn(async () => summaryA);
	const applyRepoDiffSummary = vi.fn();
	const formatError = vi.fn((_: unknown, fallback: string) => fallback);

	const controller = createSummarySourceController({
		workspaceId: () => state.workspaceId,
		repoId: () => state.repoId,
		repoStatusKnown: () => state.repoStatusKnown,
		repoMissing: () => state.repoMissing,
		localHasUncommitted: () => state.localHasUncommitted,
		getRemotes: () => state.remotes,
		getPullRequestRefs: () => state.pullRequest,
		selected: () => state.selected,
		selectedSource: () => state.selectedSource,
		summary: () => state.summary,
		setSummary: (value) => {
			state.summary = value;
		},
		setSummaryLoading: (value) => {
			state.summaryLoading = value;
		},
		setSummaryError: (value) => {
			state.summaryError = value;
		},
		setLocalSummary: (value) => {
			state.localSummary = value;
		},
		setSelected: (value) => {
			state.selected = value;
		},
		setSelectedDiff: vi.fn(),
		setFileMeta: vi.fn(),
		setFileError: vi.fn(),
		selectFile: vi.fn((file: RepoDiffFileSummary, source: 'pr' | 'local' = 'pr') => {
			state.selected = file;
			state.selectedSource = source;
		}),
		fetchRepoDiffSummary,
		fetchBranchDiffSummary,
		applyRepoDiffSummary,
		formatError,
	});

	return {
		state,
		controller,
		fetchRepoDiffSummary,
		fetchBranchDiffSummary,
	};
};

describe('summarySourceController', () => {
	it('resolves branch refs from remotes and pull request refs', () => {
		const setup = createSetup();
		setup.state.pullRequest = {
			baseRepo: 'acme/workset',
			baseBranch: 'main',
			headRepo: 'acme/workset',
			headBranch: 'feature/refactor',
		};

		expect(setup.controller.useBranchDiff()).toEqual({
			base: 'origin/main',
			head: 'origin/feature/refactor',
		});
	});

	it('reloads summary only when branch refs become available or change', async () => {
		const setup = createSetup();

		setup.controller.reloadSummaryOnBranchRefChange();
		await flushPromises();
		expect(setup.fetchBranchDiffSummary).not.toHaveBeenCalled();
		expect(setup.fetchRepoDiffSummary).not.toHaveBeenCalled();

		setup.state.pullRequest = {
			baseRepo: 'acme/workset',
			baseBranch: 'main',
			headRepo: 'acme/workset',
			headBranch: 'feature/a',
		};
		setup.controller.reloadSummaryOnBranchRefChange();
		await flushPromises();
		expect(setup.fetchBranchDiffSummary).toHaveBeenCalledTimes(1);
		expect(setup.fetchBranchDiffSummary).toHaveBeenCalledWith(
			'ws-1',
			'repo-1',
			'origin/main',
			'origin/feature/a',
		);

		setup.controller.reloadSummaryOnBranchRefChange();
		await flushPromises();
		expect(setup.fetchBranchDiffSummary).toHaveBeenCalledTimes(1);

		setup.state.pullRequest = {
			baseRepo: 'acme/workset',
			baseBranch: 'main',
			headRepo: 'acme/workset',
			headBranch: 'feature/b',
		};
		setup.controller.reloadSummaryOnBranchRefChange();
		await flushPromises();
		expect(setup.fetchBranchDiffSummary).toHaveBeenCalledTimes(2);
		expect(setup.fetchBranchDiffSummary).toHaveBeenLastCalledWith(
			'ws-1',
			'repo-1',
			'origin/main',
			'origin/feature/b',
		);
	});

	it('delegates summary updates to the underlying summary controller', () => {
		const setup = createSetup();
		setup.controller.applySummaryUpdate(summaryA, 'pr');

		expect(setup.state.summary).toEqual(summaryA);
		expect(setup.state.summaryLoading).toBe(false);
		expect(setup.state.summaryError).toBeNull();
	});
});
