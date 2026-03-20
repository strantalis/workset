import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';

vi.mock('../api/github', () => ({
	searchGitHubRepositories: vi.fn(),
}));

import { searchGitHubRepositories } from '../api/github';
import { createRepoSearch } from './createRepoSearch.svelte';

const mockSearch = vi.mocked(searchGitHubRepositories);

const fakeResult = (name: string) => ({
	name,
	fullName: `org/${name}`,
	owner: 'org',
	cloneUrl: `https://github.com/org/${name}.git`,
	sshUrl: `git@github.com:org/${name}.git`,
	description: '',
	language: 'TypeScript',
	defaultBranch: 'main',
	private: false,
	fork: false,
	archived: false,
	stargazers: 0,
	updatedAt: '2026-01-01T00:00:00Z',
	host: 'github.com',
});

describe('createRepoSearch', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		mockSearch.mockReset();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('starts in idle state', () => {
		const search = createRepoSearch();
		expect(search.results).toEqual([]);
		expect(search.loading).toBe(false);
		expect(search.error).toBe(null);
		expect(search.suggestionsOpen).toBe(false);
		expect(search.activeSuggestionIndex).toBe(-1);
		search.destroy();
	});

	it('shows search start hint when query is empty', () => {
		const search = createRepoSearch();
		expect(search.showSearchStartHint).toBe(true);
		search.destroy();
	});

	it('debounces search calls', async () => {
		mockSearch.mockResolvedValue([fakeResult('repo-a')]);
		const search = createRepoSearch({ debounceMs: 100 });
		search.handleFocus('');
		search.queue('re');
		search.queue('rep');
		search.queue('repo');

		// Should not have called yet
		expect(mockSearch).not.toHaveBeenCalled();

		await vi.advanceTimersByTimeAsync(100);
		expect(mockSearch).toHaveBeenCalledTimes(1);
		expect(mockSearch).toHaveBeenCalledWith('repo', 8);
		search.destroy();
	});

	it('cancels stale requests via sequence', async () => {
		let resolveFirst: (value: unknown[]) => void;
		const firstCall = new Promise<unknown[]>((r) => {
			resolveFirst = r;
		});
		mockSearch
			.mockReturnValueOnce(firstCall as Promise<never>)
			.mockResolvedValueOnce([fakeResult('second')]);

		const search = createRepoSearch({ debounceMs: 0 });
		search.handleFocus('');

		// First search
		search.queue('first-query');
		await vi.advanceTimersByTimeAsync(0);

		// Second search before first resolves
		search.queue('second-query');
		await vi.advanceTimersByTimeAsync(0);

		// Resolve first (should be ignored due to stale sequence)
		resolveFirst!([fakeResult('first')]);
		await vi.advanceTimersByTimeAsync(0);

		expect(search.results.map((r) => r.name)).toEqual(['second']);
		search.destroy();
	});

	it('does not search for URLs', () => {
		const search = createRepoSearch();
		search.queue('https://github.com/org/repo');
		expect(mockSearch).not.toHaveBeenCalled();
		expect(search.suggestionsOpen).toBe(false);
		search.destroy();
	});

	it('does not search for local paths', () => {
		const search = createRepoSearch();
		search.queue('/Users/sean/projects');
		expect(mockSearch).not.toHaveBeenCalled();
		expect(search.suggestionsOpen).toBe(false);
		search.destroy();
	});

	it('shows min chars hint for single-character queries', () => {
		const search = createRepoSearch();
		search.handleFocus('');
		search.queue('r');
		expect(search.showSearchMinCharsHint).toBe(true);
		expect(mockSearch).not.toHaveBeenCalled();
		search.destroy();
	});

	it('respects isActive gate', () => {
		const active = false;
		const search = createRepoSearch({ isActive: () => active });
		search.handleFocus('');
		search.queue('repo');
		expect(mockSearch).not.toHaveBeenCalled();
		search.destroy();
	});

	it('resets all state on reset()', async () => {
		mockSearch.mockResolvedValue([fakeResult('repo-a')]);
		const search = createRepoSearch({ debounceMs: 0 });
		search.handleFocus('');
		search.queue('repo');
		await vi.advanceTimersByTimeAsync(0);

		expect(search.results.length).toBe(1);
		search.reset();

		expect(search.results).toEqual([]);
		expect(search.loading).toBe(false);
		expect(search.error).toBe(null);
		expect(search.suggestionsOpen).toBe(false);
		search.destroy();
	});

	it('maps auth errors to user-friendly message', async () => {
		mockSearch.mockRejectedValue(new Error('GitHub auth required'));
		const search = createRepoSearch({ debounceMs: 0 });
		search.handleFocus('');
		search.queue('repo');
		await vi.advanceTimersByTimeAsync(0);

		expect(search.error).toBe('Connect GitHub in Settings -> GitHub authentication to search.');
		search.destroy();
	});

	it('handles blur with delayed close', async () => {
		mockSearch.mockResolvedValue([fakeResult('repo-a')]);
		const search = createRepoSearch({ debounceMs: 0 });
		search.handleFocus('');
		search.queue('repo');
		await vi.advanceTimersByTimeAsync(0);

		expect(search.suggestionsOpen).toBe(true);
		search.handleBlur();

		// Not closed yet
		expect(search.suggestionsOpen).toBe(true);

		// Closed after 120ms delay
		await vi.advanceTimersByTimeAsync(120);
		expect(search.suggestionsOpen).toBe(false);
		search.destroy();
	});

	it('cancels blur close on re-focus', async () => {
		mockSearch.mockResolvedValue([fakeResult('repo-a')]);
		const search = createRepoSearch({ debounceMs: 0 });
		search.handleFocus('');
		search.queue('repo');
		await vi.advanceTimersByTimeAsync(0);

		search.handleBlur();
		await vi.advanceTimersByTimeAsync(50);

		// Re-focus before close timer fires
		search.handleFocus('repo');
		await vi.advanceTimersByTimeAsync(120);

		// Should still be open
		expect(search.suggestionsOpen).toBe(true);
		search.destroy();
	});

	describe('keyboard navigation', () => {
		it('navigates suggestions with arrow keys', async () => {
			mockSearch.mockResolvedValue([fakeResult('a'), fakeResult('b'), fakeResult('c')]);
			const search = createRepoSearch({ debounceMs: 0 });
			search.handleFocus('');
			search.queue('repo');
			await vi.advanceTimersByTimeAsync(0);

			expect(search.activeSuggestionIndex).toBe(0);

			const down = new KeyboardEvent('keydown', { key: 'ArrowDown' });
			search.handleKeydown(down, () => {});
			expect(search.activeSuggestionIndex).toBe(1);

			search.handleKeydown(down, () => {});
			expect(search.activeSuggestionIndex).toBe(2);

			// Wraps around
			search.handleKeydown(down, () => {});
			expect(search.activeSuggestionIndex).toBe(0);

			const up = new KeyboardEvent('keydown', { key: 'ArrowUp' });
			search.handleKeydown(up, () => {});
			expect(search.activeSuggestionIndex).toBe(2);

			search.destroy();
		});

		it('selects suggestion on Enter', async () => {
			const items = [fakeResult('selected-repo')];
			mockSearch.mockResolvedValue(items);
			const search = createRepoSearch({ debounceMs: 0 });
			search.handleFocus('');
			search.queue('repo');
			await vi.advanceTimersByTimeAsync(0);

			const onSelect = vi.fn();
			const enter = new KeyboardEvent('keydown', { key: 'Enter' });
			search.handleKeydown(enter, onSelect);

			expect(onSelect).toHaveBeenCalledWith(items[0]);
			expect(search.results).toEqual([]);
			search.destroy();
		});

		it('resets on Escape', async () => {
			mockSearch.mockResolvedValue([fakeResult('a')]);
			const search = createRepoSearch({ debounceMs: 0 });
			search.handleFocus('');
			search.queue('repo');
			await vi.advanceTimersByTimeAsync(0);

			const escape = new KeyboardEvent('keydown', { key: 'Escape' });
			search.handleKeydown(escape, () => {});

			expect(search.suggestionsOpen).toBe(false);
			expect(search.results).toEqual([]);
			search.destroy();
		});
	});

	it('cleans up timers on destroy', () => {
		const search = createRepoSearch();
		search.handleFocus('');
		search.queue('repo');

		// Should not throw
		search.destroy();
		vi.advanceTimersByTime(500);
		expect(mockSearch).not.toHaveBeenCalled();
	});
});
