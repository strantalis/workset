import { fireEvent, render, screen } from '@testing-library/svelte';
import type {
	PullRequestCheck,
	PullRequestStatusResult,
	RepoDiffFileSummary,
	RepoDiffSummary,
} from '../../types';
import { describe, expect, it, vi } from 'vitest';
import RepoDiffFileListSidebar from './RepoDiffFileListSidebar.svelte';

const prFile: RepoDiffFileSummary = {
	path: 'src/pr-file.ts',
	added: 12,
	removed: 3,
	status: 'modified',
};

const localFile: RepoDiffFileSummary = {
	path: 'src/local-file.ts',
	added: 2,
	removed: 1,
	status: 'modified',
};

const summary: RepoDiffSummary = {
	files: [prFile],
	totalAdded: 12,
	totalRemoved: 3,
};

const localSummary: RepoDiffSummary = {
	files: [localFile],
	totalAdded: 2,
	totalRemoved: 1,
};

const failedCheck: PullRequestCheck = {
	name: 'lint',
	status: 'completed',
	conclusion: 'failure',
	checkRunId: 100,
	detailsUrl: 'https://github.com/acme/workset/actions/runs/100',
};

const prStatus: PullRequestStatusResult = {
	pullRequest: {
		repo: 'acme/workset',
		number: 42,
		url: 'https://github.com/acme/workset/pull/42',
		title: 'Refactor repo diff sidebar',
		state: 'open',
		draft: false,
		baseRepo: 'acme/workset',
		baseBranch: 'main',
		headRepo: 'acme/workset',
		headBranch: 'feature/sidebar',
	},
	checks: [failedCheck],
};

describe('RepoDiffFileListSidebar', () => {
	it('renders PR and local file sections and preserves select callbacks', async () => {
		const selectFile = vi.fn();

		render(RepoDiffFileListSidebar, {
			props: {
				summary,
				localSummary,
				selected: null,
				selectedSource: 'pr',
				shouldSplitLocalPendingSection: true,
				effectiveMode: 'status',
				prStatus,
				checkStats: { total: 1, passed: 0, failed: 1, pending: 0 },
				expandedCheck: null,
				checkAnnotationsLoading: {},
				formatDuration: (ms: number) => `${ms}ms`,
				getCheckStatusClass: () => 'check-failure',
				toggleCheckExpansion: vi.fn(),
				navigateToAnnotationFile: vi.fn(),
				getFilteredAnnotations: () => ({ annotations: [], filteredCount: 0 }),
				reviewCountForFile: (path: string) => (path === prFile.path ? 2 : 0),
				selectFile,
				onOpenDetailsUrl: vi.fn(),
			},
		});

		expect(screen.getByText('PR files')).toBeInTheDocument();
		expect(screen.getByText('Local pending changes')).toBeInTheDocument();
		expect(screen.getByText('💬 2')).toBeInTheDocument();

		const prFileButton = screen.getByTitle(prFile.path).closest('button');
		const localFileButton = screen.getByTitle(localFile.path).closest('button');
		expect(prFileButton).toBeTruthy();
		expect(localFileButton).toBeTruthy();
		if (prFileButton) {
			await fireEvent.click(prFileButton);
		}
		if (localFileButton) {
			await fireEvent.click(localFileButton);
		}

		expect(selectFile).toHaveBeenCalledWith(prFile, 'pr');
		expect(selectFile).toHaveBeenCalledWith(localFile, 'local');
	});

	it('switches to checks tab and forwards details URL handler', async () => {
		const onOpenDetailsUrl = vi.fn();

		render(RepoDiffFileListSidebar, {
			props: {
				summary,
				localSummary: null,
				selected: null,
				selectedSource: 'pr',
				shouldSplitLocalPendingSection: false,
				effectiveMode: 'status',
				prStatus,
				checkStats: { total: 1, passed: 0, failed: 1, pending: 0 },
				expandedCheck: null,
				checkAnnotationsLoading: {},
				formatDuration: (ms: number) => `${ms}ms`,
				getCheckStatusClass: () => 'check-failure',
				toggleCheckExpansion: vi.fn(),
				navigateToAnnotationFile: vi.fn(),
				getFilteredAnnotations: () => ({ annotations: [], filteredCount: 0 }),
				reviewCountForFile: () => 0,
				selectFile: vi.fn(),
				onOpenDetailsUrl,
			},
		});

		await fireEvent.click(screen.getByRole('button', { name: /checks/i }));
		expect(screen.getByText('lint')).toBeInTheDocument();

		await fireEvent.click(screen.getByTitle('View on GitHub'));
		expect(onOpenDetailsUrl).toHaveBeenCalledWith(failedCheck.detailsUrl);
	});
});
