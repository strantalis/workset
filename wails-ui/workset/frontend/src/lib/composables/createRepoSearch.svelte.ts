import { searchGitHubRepositories } from '../api/github';
import { isLikelyLocalPath, looksLikeUrl } from '../names';
import type { GitHubRepoSearchItem } from '../types';

export type RepoSearchOptions = {
	/** Max results per search (default 8). */
	maxResults?: number;
	/** Debounce delay in ms (default 250). */
	debounceMs?: number;
	/** Optional gate: search is only active when this returns true. */
	isActive?: () => boolean;
};

export type RepoSearchInstance = {
	readonly results: GitHubRepoSearchItem[];
	readonly loading: boolean;
	readonly error: string | null;
	readonly suggestionsOpen: boolean;
	readonly activeSuggestionIndex: number;
	readonly lastSearchedQuery: string;
	readonly focused: boolean;
	readonly showSearchStartHint: boolean;
	readonly showSearchMinCharsHint: boolean;
	readonly showNoSearchResults: boolean;
	queue: (query: string) => void;
	reset: () => void;
	handleFocus: (currentQuery: string) => void;
	handleBlur: () => void;
	handleKeydown: (event: KeyboardEvent, onSelect: (item: GitHubRepoSearchItem) => void) => void;
	destroy: () => void;
};

function toSearchErrorMessage(err: unknown): string {
	const message = err instanceof Error ? err.message : 'Failed to search repositories.';
	const normalized = message.toLowerCase();
	if (
		normalized.includes('auth required') ||
		normalized.includes('not authenticated') ||
		normalized.includes('authentication') ||
		normalized.includes('authenticate') ||
		normalized.includes('github auth')
	) {
		return 'Connect GitHub in Settings -> GitHub authentication to search.';
	}
	return message;
}

function shouldSearchRemote(value: string): boolean {
	const trimmed = value.trim();
	return trimmed.length >= 2 && !looksLikeUrl(trimmed) && !isLikelyLocalPath(trimmed);
}

export function createRepoSearch(options: RepoSearchOptions = {}): RepoSearchInstance {
	const maxResults = options.maxResults ?? 8;
	const debounceMs = options.debounceMs ?? 250;
	const isActive = options.isActive ?? (() => true);

	let results = $state<GitHubRepoSearchItem[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let suggestionsOpen = $state(false);
	let activeSuggestionIndex = $state(-1);
	let lastSearchedQuery = $state('');
	let focused = $state(false);

	let debounceTimer: ReturnType<typeof setTimeout> | null = null;
	let closeTimer: ReturnType<typeof setTimeout> | null = null;
	let sequence = 0;

	const showSearchStartHint = $derived(lastSearchedQuery === '' && !loading);
	const showSearchMinCharsHint = $derived(
		lastSearchedQuery.length > 0 &&
			lastSearchedQuery.length < 2 &&
			!looksLikeUrl(lastSearchedQuery) &&
			!isLikelyLocalPath(lastSearchedQuery),
	);
	const showNoSearchResults = $derived(
		!loading &&
			error === null &&
			!showSearchStartHint &&
			!showSearchMinCharsHint &&
			results.length === 0 &&
			lastSearchedQuery !== '' &&
			lastSearchedQuery.length >= 2,
	);

	const clearTimers = (): void => {
		if (debounceTimer) {
			clearTimeout(debounceTimer);
			debounceTimer = null;
		}
		if (closeTimer) {
			clearTimeout(closeTimer);
			closeTimer = null;
		}
	};

	const reset = (): void => {
		clearTimers();
		sequence += 1;
		results = [];
		loading = false;
		error = null;
		suggestionsOpen = false;
		activeSuggestionIndex = -1;
		lastSearchedQuery = '';
	};

	const showHints = (query: string): void => {
		sequence += 1;
		results = [];
		loading = false;
		error = null;
		suggestionsOpen = focused;
		activeSuggestionIndex = -1;
		lastSearchedQuery = query;
	};

	const runSearch = async (query: string): Promise<void> => {
		const requestSequence = ++sequence;
		loading = true;
		error = null;
		suggestionsOpen = focused;
		lastSearchedQuery = query;
		try {
			const items = await searchGitHubRepositories(query, maxResults);
			if (requestSequence !== sequence) return;
			results = items;
			activeSuggestionIndex = items.length > 0 ? 0 : -1;
		} catch (err) {
			if (requestSequence !== sequence) return;
			results = [];
			activeSuggestionIndex = -1;
			error = toSearchErrorMessage(err);
		} finally {
			if (requestSequence === sequence) {
				loading = false;
			}
		}
	};

	const queue = (query: string): void => {
		const trimmed = query.trim();
		if (!isActive()) {
			reset();
			return;
		}
		if (debounceTimer) {
			clearTimeout(debounceTimer);
			debounceTimer = null;
		}
		if (trimmed.length === 0) {
			showHints('');
			return;
		}
		if (!shouldSearchRemote(trimmed)) {
			if (looksLikeUrl(trimmed) || isLikelyLocalPath(trimmed)) {
				reset();
				return;
			}
			showHints(trimmed);
			return;
		}
		debounceTimer = setTimeout(() => {
			void runSearch(trimmed);
		}, debounceMs);
	};

	const handleFocus = (currentQuery: string): void => {
		focused = true;
		if (closeTimer) {
			clearTimeout(closeTimer);
			closeTimer = null;
		}
		queue(currentQuery);
	};

	const handleBlur = (): void => {
		focused = false;
		if (closeTimer) {
			clearTimeout(closeTimer);
		}
		closeTimer = setTimeout(() => {
			suggestionsOpen = false;
		}, 120);
	};

	const handleKeydown = (
		event: KeyboardEvent,
		onSelect: (item: GitHubRepoSearchItem) => void,
	): void => {
		if (!suggestionsOpen || loading || error) {
			if (event.key === 'Escape') reset();
			return;
		}
		if (results.length === 0) return;
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			activeSuggestionIndex = (activeSuggestionIndex + 1) % results.length;
			return;
		}
		if (event.key === 'ArrowUp') {
			event.preventDefault();
			activeSuggestionIndex = (activeSuggestionIndex - 1 + results.length) % results.length;
			return;
		}
		if (event.key === 'Enter' && activeSuggestionIndex >= 0) {
			event.preventDefault();
			onSelect(results[activeSuggestionIndex]);
			reset();
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			reset();
		}
	};

	const destroy = (): void => {
		clearTimers();
	};

	return {
		get results() {
			return results;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		get suggestionsOpen() {
			return suggestionsOpen;
		},
		get activeSuggestionIndex() {
			return activeSuggestionIndex;
		},
		get lastSearchedQuery() {
			return lastSearchedQuery;
		},
		get focused() {
			return focused;
		},
		get showSearchStartHint() {
			return showSearchStartHint;
		},
		get showSearchMinCharsHint() {
			return showSearchMinCharsHint;
		},
		get showNoSearchResults() {
			return showNoSearchResults;
		},
		queue,
		reset,
		handleFocus,
		handleBlur,
		handleKeydown,
		destroy,
	};
}
