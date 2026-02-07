import type {
	CheckAnnotation,
	PullRequestCheck,
	PullRequestStatusResult,
	RemoteInfo,
	RepoDiffFileSummary,
	RepoDiffSummary,
} from '../../types';
import type { SummarySource } from './fileDiffController';

export type CheckStats = {
	total: number;
	passed: number;
	failed: number;
	pending: number;
};

export type FilteredAnnotationsResult = {
	annotations: CheckAnnotation[];
	filteredCount: number;
};

type CheckSidebarControllerOptions = {
	getExpandedCheck: () => string | null;
	setExpandedCheck: (value: string | null) => void;
	getCheckAnnotations: () => Record<string, CheckAnnotation[]>;
	setCheckAnnotations: (value: Record<string, CheckAnnotation[]>) => void;
	getCheckAnnotationsLoading: () => Record<string, boolean>;
	setCheckAnnotationsLoading: (value: Record<string, boolean>) => void;
	getPrStatus: () => PullRequestStatusResult | null;
	getRemotes: () => RemoteInfo[];
	getPrBaseRemote: () => string;
	getSummary: () => RepoDiffSummary | null;
	fetchCheckAnnotations: (
		owner: string,
		repo: string,
		checkRunId: number,
	) => Promise<CheckAnnotation[]>;
	selectFile: (file: RepoDiffFileSummary, source?: SummarySource) => void;
	setPendingScrollLine: (line: number) => void;
	logError?: (...args: unknown[]) => void;
};

const resolveOwnerRepo = (
	prStatus: PullRequestStatusResult | null,
	remotes: RemoteInfo[],
	prBaseRemote: string,
): { owner?: string; repo?: string } => {
	if (prStatus?.pullRequest?.baseRepo) {
		const parts = prStatus.pullRequest.baseRepo.split('/');
		if (parts.length === 2) {
			return { owner: parts[0], repo: parts[1] };
		}
	}

	const fallbackRemote = remotes.find((remote) => remote.name === prBaseRemote) ?? remotes[0];
	if (!fallbackRemote) {
		return {};
	}

	return {
		owner: fallbackRemote.owner,
		repo: fallbackRemote.repo,
	};
};

export const getCheckStats = (checks: PullRequestCheck[]): CheckStats => {
	const passed = checks.filter((check) => check.conclusion === 'success').length;
	const failed = checks.filter((check) => check.conclusion === 'failure').length;
	const pending = checks.filter(
		(check) => !check.conclusion || check.status === 'in_progress' || check.status === 'queued',
	).length;
	return {
		total: checks.length,
		passed,
		failed,
		pending,
	};
};

export const createCheckSidebarController = (options: CheckSidebarControllerOptions) => {
	const setAnnotationsForCheck = (checkName: string, annotations: CheckAnnotation[]): void => {
		options.setCheckAnnotations({
			...options.getCheckAnnotations(),
			[checkName]: annotations,
		});
	};

	const setAnnotationsLoading = (checkName: string, loading: boolean): void => {
		options.setCheckAnnotationsLoading({
			...options.getCheckAnnotationsLoading(),
			[checkName]: loading,
		});
	};

	const formatDuration = (milliseconds: number): string => {
		if (milliseconds < 1000) return `${milliseconds}ms`;
		if (milliseconds < 60000) return `${Math.round(milliseconds / 1000)}s`;
		const minutes = Math.floor(milliseconds / 60000);
		const seconds = Math.round((milliseconds % 60000) / 1000);
		return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
	};

	const getCheckStatusClass = (conclusion: string | undefined, status: string): string => {
		if (conclusion === 'success') return 'check-success';
		if (conclusion === 'failure') return 'check-failure';
		if (conclusion === 'skipped' || conclusion === 'cancelled' || conclusion === 'neutral')
			return 'check-neutral';
		if (status === 'in_progress' || status === 'queued') return 'check-pending';
		return 'check-neutral';
	};

	const loadCheckAnnotations = async (
		checkName: string,
		checkRunId: number | undefined,
	): Promise<void> => {
		if (!checkRunId) return;
		if (options.getCheckAnnotationsLoading()[checkName]) return;

		const { owner, repo } = resolveOwnerRepo(
			options.getPrStatus(),
			options.getRemotes(),
			options.getPrBaseRemote(),
		);

		if (!owner || !repo) {
			options.logError?.('Cannot load annotations: missing owner/repo', {
				checkName,
				checkRunId,
				baseRepo: options.getPrStatus()?.pullRequest?.baseRepo,
				remotes: options.getRemotes(),
			});
			setAnnotationsForCheck(checkName, []);
			return;
		}

		setAnnotationsLoading(checkName, true);
		try {
			const result = await options.fetchCheckAnnotations(owner, repo, checkRunId);
			setAnnotationsForCheck(checkName, result);
		} catch (error) {
			options.logError?.('Failed to load annotations:', error);
			setAnnotationsForCheck(checkName, []);
		} finally {
			setAnnotationsLoading(checkName, false);
		}
	};

	const toggleCheckExpansion = (check: PullRequestCheck): void => {
		if (options.getExpandedCheck() === check.name) {
			options.setExpandedCheck(null);
			return;
		}

		options.setExpandedCheck(check.name);
		if (check.conclusion === 'failure' && check.checkRunId) {
			void loadCheckAnnotations(check.name, check.checkRunId);
		}
	};

	const navigateToAnnotationFile = (path: string, line: number): void => {
		const file = options.getSummary()?.files.find((item) => item.path === path);
		if (!file) return;

		options.setPendingScrollLine(line);
		options.selectFile(file, 'pr');
	};

	const getFilteredAnnotations = (checkName: string): FilteredAnnotationsResult => {
		const allAnnotations = options.getCheckAnnotations()[checkName] ?? [];
		const filesInDiff = new Set(options.getSummary()?.files.map((file) => file.path) ?? []);
		const filtered = allAnnotations.filter((annotation) => filesInDiff.has(annotation.path));
		return {
			annotations: filtered,
			filteredCount: allAnnotations.length - filtered.length,
		};
	};

	return {
		formatDuration,
		getCheckStatusClass,
		loadCheckAnnotations,
		toggleCheckExpansion,
		navigateToAnnotationFile,
		getFilteredAnnotations,
	};
};
