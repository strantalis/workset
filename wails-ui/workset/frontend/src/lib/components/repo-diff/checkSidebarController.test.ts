import { describe, expect, it, vi } from 'vitest';
import type {
	CheckAnnotation,
	PullRequestCheck,
	PullRequestStatusResult,
	RemoteInfo,
	RepoDiffFileSummary,
	RepoDiffSummary,
} from '../../types';
import { createCheckSidebarController, getCheckStats } from './checkSidebarController';

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

const flushPromises = async (): Promise<void> => {
	await Promise.resolve();
	await Promise.resolve();
};

const buildPrStatus = (baseRepo: string): PullRequestStatusResult => ({
	pullRequest: {
		repo: 'acme/workset',
		number: 12,
		url: 'https://github.com/acme/workset/pull/12',
		title: 'Improve checks',
		state: 'open',
		draft: false,
		baseRepo,
		baseBranch: 'main',
		headRepo: 'acme/workset',
		headBranch: 'feature/checks',
	},
	checks: [],
});

const fileA: RepoDiffFileSummary = {
	path: 'src/a.ts',
	added: 1,
	removed: 0,
	status: 'modified',
};

const createSetup = () => {
	const state: {
		expandedCheck: string | null;
		checkAnnotations: Record<string, CheckAnnotation[]>;
		checkAnnotationsLoading: Record<string, boolean>;
		prStatus: PullRequestStatusResult | null;
		remotes: RemoteInfo[];
		prBaseRemote: string;
		summary: RepoDiffSummary | null;
		pendingScrollLine: number | null;
	} = {
		expandedCheck: null,
		checkAnnotations: {},
		checkAnnotationsLoading: {},
		prStatus: buildPrStatus('acme/workset'),
		remotes: [],
		prBaseRemote: '',
		summary: { files: [fileA], totalAdded: 1, totalRemoved: 0 },
		pendingScrollLine: null,
	};

	const fetchCheckAnnotations = vi.fn(async (): Promise<CheckAnnotation[]> => []);
	const selectFile = vi.fn();
	const logError = vi.fn();

	const controller = createCheckSidebarController({
		getExpandedCheck: () => state.expandedCheck,
		setExpandedCheck: (value) => {
			state.expandedCheck = value;
		},
		getCheckAnnotations: () => state.checkAnnotations,
		setCheckAnnotations: (value) => {
			state.checkAnnotations = value;
		},
		getCheckAnnotationsLoading: () => state.checkAnnotationsLoading,
		setCheckAnnotationsLoading: (value) => {
			state.checkAnnotationsLoading = value;
		},
		getPrStatus: () => state.prStatus,
		getRemotes: () => state.remotes,
		getPrBaseRemote: () => state.prBaseRemote,
		getSummary: () => state.summary,
		fetchCheckAnnotations,
		selectFile,
		setPendingScrollLine: (line) => {
			state.pendingScrollLine = line;
		},
		logError,
	});

	return {
		state,
		fetchCheckAnnotations,
		selectFile,
		logError,
		controller,
	};
};

describe('checkSidebarController', () => {
	it('computes check stats for passed/failed/pending checks', () => {
		const checks: PullRequestCheck[] = [
			{ name: 'ci', status: 'completed', conclusion: 'success' },
			{ name: 'lint', status: 'completed', conclusion: 'failure' },
			{ name: 'e2e', status: 'in_progress' },
		];

		expect(getCheckStats(checks)).toEqual({
			total: 3,
			passed: 1,
			failed: 1,
			pending: 1,
		});
	});

	it('expands failed checks and loads annotations once per in-flight request', async () => {
		const setup = createSetup();
		const deferred = createDeferred<CheckAnnotation[]>();
		setup.fetchCheckAnnotations.mockImplementationOnce(async () => deferred.promise);
		const failedCheck: PullRequestCheck = {
			name: 'ci',
			status: 'completed',
			conclusion: 'failure',
			checkRunId: 77,
		};

		setup.controller.toggleCheckExpansion(failedCheck);
		expect(setup.state.expandedCheck).toBe('ci');
		expect(setup.state.checkAnnotationsLoading.ci).toBe(true);

		await setup.controller.loadCheckAnnotations('ci', 77);
		expect(setup.fetchCheckAnnotations).toHaveBeenCalledTimes(1);

		deferred.resolve([
			{
				path: 'src/a.ts',
				startLine: 4,
				endLine: 4,
				level: 'failure',
				message: 'Type error',
			},
		]);
		await flushPromises();

		expect(setup.fetchCheckAnnotations).toHaveBeenCalledWith('acme', 'workset', 77);
		expect(setup.state.checkAnnotationsLoading.ci).toBe(false);
		expect(setup.state.checkAnnotations.ci).toHaveLength(1);
	});

	it('falls back to remotes using selected base remote when PR base repo is unavailable', async () => {
		const setup = createSetup();
		setup.state.prStatus = buildPrStatus('');
		setup.state.remotes = [
			{ name: 'origin', owner: 'local', repo: 'workset' },
			{ name: 'upstream', owner: 'core', repo: 'main-workset' },
		];
		setup.state.prBaseRemote = 'upstream';

		await setup.controller.loadCheckAnnotations('lint', 11);

		expect(setup.fetchCheckAnnotations).toHaveBeenCalledWith('core', 'main-workset', 11);
	});

	it('stores empty annotations and logs when owner/repo cannot be resolved', async () => {
		const setup = createSetup();
		setup.state.prStatus = null;
		setup.state.remotes = [];

		await setup.controller.loadCheckAnnotations('lint', 11);

		expect(setup.fetchCheckAnnotations).not.toHaveBeenCalled();
		expect(setup.state.checkAnnotations.lint).toEqual([]);
		expect(setup.logError).toHaveBeenCalledTimes(1);
	});

	it('filters annotations to files present in the active diff summary', () => {
		const setup = createSetup();
		setup.state.checkAnnotations = {
			ci: [
				{
					path: 'src/a.ts',
					startLine: 1,
					endLine: 1,
					level: 'failure',
					message: 'in diff',
				},
				{
					path: 'src/b.ts',
					startLine: 2,
					endLine: 2,
					level: 'failure',
					message: 'outside diff',
				},
			],
		};

		const result = setup.controller.getFilteredAnnotations('ci');
		expect(result.annotations).toHaveLength(1);
		expect(result.annotations[0].path).toBe('src/a.ts');
		expect(result.filteredCount).toBe(1);
	});

	it('navigates to the annotated file when it exists in summary', () => {
		const setup = createSetup();

		setup.controller.navigateToAnnotationFile('src/a.ts', 19);
		expect(setup.state.pendingScrollLine).toBe(19);
		expect(setup.selectFile).toHaveBeenCalledWith(fileA, 'pr');

		setup.controller.navigateToAnnotationFile('src/missing.ts', 8);
		expect(setup.selectFile).toHaveBeenCalledTimes(1);
	});
});
