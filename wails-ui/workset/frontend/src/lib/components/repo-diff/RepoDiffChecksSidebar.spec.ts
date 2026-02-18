import { fireEvent, render, screen } from '@testing-library/svelte';
import type { PullRequestCheck, PullRequestStatusResult } from '../../types';
import { describe, expect, it, vi } from 'vitest';
import RepoDiffChecksSidebar from './RepoDiffChecksSidebar.svelte';

const failedCheck: PullRequestCheck = {
	name: 'lint',
	status: 'completed',
	conclusion: 'failure',
	checkRunId: 11,
	startedAt: '2026-01-01T00:00:00.000Z',
	completedAt: '2026-01-01T00:00:01.000Z',
	detailsUrl: 'https://github.com/acme/workset/actions/runs/11',
};

const passedCheck: PullRequestCheck = {
	name: 'ci',
	status: 'completed',
	conclusion: 'success',
	detailsUrl: 'https://github.com/acme/workset/actions/runs/12',
};

const prStatus: PullRequestStatusResult = {
	pullRequest: {
		repo: 'acme/workset',
		number: 42,
		url: 'https://github.com/acme/workset/pull/42',
		title: 'Improve checks sidebar',
		state: 'open',
		draft: false,
		baseRepo: 'acme/workset',
		baseBranch: 'main',
		headRepo: 'acme/workset',
		headBranch: 'feature/checks',
	},
	checks: [failedCheck, passedCheck],
};

describe('RepoDiffChecksSidebar', () => {
	it('renders checks summary and routes expansion/navigation callbacks', async () => {
		const toggleCheckExpansion = vi.fn();
		const navigateToAnnotationFile = vi.fn();
		const onOpenDetailsUrl = vi.fn();

		render(RepoDiffChecksSidebar, {
			props: {
				prStatus,
				checkStats: { total: 2, passed: 1, failed: 1, pending: 0 },
				expandedCheck: 'lint',
				checkAnnotationsLoading: {},
				formatDuration: (ms: number) => `${ms}ms`,
				getCheckStatusClass: (conclusion: string | undefined) =>
					conclusion === 'failure' ? 'check-failure' : 'check-success',
				toggleCheckExpansion,
				navigateToAnnotationFile,
				getFilteredAnnotations: (checkName: string) =>
					checkName === 'lint'
						? {
								annotations: [
									{
										path: 'src/a.ts',
										startLine: 10,
										endLine: 10,
										level: 'failure',
										message: 'Type mismatch',
									},
								],
								filteredCount: 1,
							}
						: { annotations: [], filteredCount: 0 },
				onOpenDetailsUrl,
			},
		});

		expect(screen.getByText('lint')).toBeInTheDocument();
		expect(screen.getByText('ci')).toBeInTheDocument();
		expect(screen.getByText('src/a.ts:10')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: /lint/i }));
		expect(toggleCheckExpansion).toHaveBeenCalledWith(expect.objectContaining({ name: 'lint' }));

		await fireEvent.click(screen.getByRole('button', { name: /src\/a\.ts:10/i }));
		expect(navigateToAnnotationFile).toHaveBeenCalledWith('src/a.ts', 10);

		toggleCheckExpansion.mockClear();
		await fireEvent.click(screen.getAllByTitle('View on GitHub')[0]);
		expect(onOpenDetailsUrl).toHaveBeenCalledWith(failedCheck.detailsUrl);
		expect(toggleCheckExpansion).not.toHaveBeenCalled();
	});

	it('shows loading state for expanded failed checks', () => {
		render(RepoDiffChecksSidebar, {
			props: {
				prStatus,
				checkStats: { total: 2, passed: 1, failed: 1, pending: 0 },
				expandedCheck: 'lint',
				checkAnnotationsLoading: { lint: true },
				formatDuration: (ms: number) => `${ms}ms`,
				getCheckStatusClass: () => 'check-failure',
				toggleCheckExpansion: vi.fn(),
				navigateToAnnotationFile: vi.fn(),
				getFilteredAnnotations: () => ({ annotations: [], filteredCount: 0 }),
				onOpenDetailsUrl: vi.fn(),
			},
		});

		expect(screen.getByText('Loading annotations...')).toBeInTheDocument();
	});
});
