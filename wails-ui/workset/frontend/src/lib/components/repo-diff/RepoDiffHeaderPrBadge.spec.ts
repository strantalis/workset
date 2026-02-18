import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import type { PullRequestStatusResult } from '../../types';
import RepoDiffHeaderPrBadge from './RepoDiffHeaderPrBadge.svelte';

const prStatus: PullRequestStatusResult = {
	pullRequest: {
		repo: 'acme/workset',
		number: 42,
		url: 'https://github.com/acme/workset/pull/42',
		title: 'Improve PR header',
		state: 'open',
		draft: false,
		baseRepo: 'acme/workset',
		baseBranch: 'main',
		headRepo: 'acme/workset',
		headBranch: 'feature/header',
	},
	checks: [],
};

describe('RepoDiffHeaderPrBadge', () => {
	it('renders loading text when status is loading without PR data', () => {
		render(RepoDiffHeaderPrBadge, {
			props: {
				effectiveMode: 'status',
				prStatus: null,
				checkStats: { total: 0, passed: 0, failed: 0, pending: 0 },
				prStatusLoading: true,
				prReviewsLoading: false,
				onOpenPrUrl: vi.fn(),
			},
		});

		expect(screen.getByText('PR...')).toBeInTheDocument();
	});

	it('renders PR badge and routes click/open callback', async () => {
		const onOpenPrUrl = vi.fn();
		render(RepoDiffHeaderPrBadge, {
			props: {
				effectiveMode: 'status',
				prStatus,
				checkStats: { total: 3, passed: 3, failed: 0, pending: 0 },
				prStatusLoading: false,
				prReviewsLoading: true,
				onOpenPrUrl,
			},
		});

		const badge = screen.getByRole('button', { name: /pr #42/i });
		expect(screen.getByText('open')).toBeInTheDocument();
		expect(screen.getByText('3')).toBeInTheDocument();
		expect(document.querySelector('.pr-badge-sync')).toBeInTheDocument();

		await fireEvent.click(badge);
		expect(onOpenPrUrl).toHaveBeenCalledWith('https://github.com/acme/workset/pull/42');
	});

	it('resolves check summary state precedence', async () => {
		const { rerender } = render(RepoDiffHeaderPrBadge, {
			props: {
				effectiveMode: 'status',
				prStatus,
				checkStats: { total: 0, passed: 0, failed: 0, pending: 0 },
				prStatusLoading: false,
				prReviewsLoading: false,
				onOpenPrUrl: vi.fn(),
			},
		});

		expect(screen.getByText('No checks')).toBeInTheDocument();

		await rerender({
			effectiveMode: 'status',
			prStatus,
			checkStats: { total: 2, passed: 0, failed: 1, pending: 1 },
			prStatusLoading: false,
			prReviewsLoading: false,
			onOpenPrUrl: vi.fn(),
		});
		expect(screen.getByText('1')).toBeInTheDocument();
		expect(document.querySelector('.pr-badge-checks.failed')).toBeInTheDocument();

		await rerender({
			effectiveMode: 'status',
			prStatus,
			checkStats: { total: 2, passed: 0, failed: 0, pending: 2 },
			prStatusLoading: false,
			prReviewsLoading: false,
			onOpenPrUrl: vi.fn(),
		});
		expect(screen.getByText('2')).toBeInTheDocument();
		expect(document.querySelector('.pr-badge-checks.pending')).toBeInTheDocument();

		await rerender({
			effectiveMode: 'status',
			prStatus,
			checkStats: { total: 4, passed: 4, failed: 0, pending: 0 },
			prStatusLoading: false,
			prReviewsLoading: false,
			onOpenPrUrl: vi.fn(),
		});
		expect(screen.getByText('4')).toBeInTheDocument();
		expect(document.querySelector('.pr-badge-checks.passed')).toBeInTheDocument();
	});
});
