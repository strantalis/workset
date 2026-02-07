import { resolveBranchRefs, type PullRequestRefs } from '../../diff/branchRefs';
import type { RemoteInfo } from '../../types';
import type { BranchDiffRefs } from './fileDiffController';
import { createSummaryController } from './summaryController';

type SummaryControllerOptions = Parameters<typeof createSummaryController>[0];

type SummarySourceControllerOptions = Omit<SummaryControllerOptions, 'useBranchDiff'> & {
	getRemotes: () => RemoteInfo[];
	getPullRequestRefs: () => PullRequestRefs | null | undefined;
};

export const createSummarySourceController = (options: SummarySourceControllerOptions) => {
	let lastBranchKey: string | null = null;

	const useBranchDiff = (): BranchDiffRefs | null =>
		resolveBranchRefs(options.getRemotes(), options.getPullRequestRefs());

	const summaryController = createSummaryController({
		...options,
		useBranchDiff,
	});

	const reloadSummaryOnBranchRefChange = (): void => {
		const branchRefs = useBranchDiff();
		const newKey = branchRefs ? `${branchRefs.base}..${branchRefs.head}` : null;
		if (newKey !== lastBranchKey && newKey !== null) {
			lastBranchKey = newKey;
			void summaryController.loadSummary();
		}
	};

	return {
		useBranchDiff,
		loadSummary: () => summaryController.loadSummary(),
		loadLocalSummary: () => summaryController.loadLocalSummary(),
		applySummaryUpdate: summaryController.applySummaryUpdate,
		reloadSummaryOnBranchRefChange,
	};
};
