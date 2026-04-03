<script lang="ts">
	import { Search } from '@lucide/svelte';
	import Icon from '@iconify/svelte';
	import { searchWorkspaceRepoFiles } from '../../api/repo-files';
	import { getRepoFileIcon } from '../repo-files/fileIcons';
	import type { RepoFileSearchResult } from '../../types';

	interface Props {
		open: boolean;
		workspaceId: string;
		onClose: () => void;
		onSelectFile: (repoId: string, path: string) => void;
	}

	const { open, workspaceId, onClose, onSelectFile }: Props = $props();

	let query = $state('');
	let selectedIndex = $state(0);
	let inputRef = $state<HTMLInputElement | null>(null);
	let results = $state<RepoFileSearchResult[]>([]);
	let loading = $state(false);
	let previousFocus: HTMLElement | null = null;
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;

	const resetState = (): void => {
		query = '';
		selectedIndex = 0;
		results = [];
		loading = false;
		if (debounceTimer) {
			clearTimeout(debounceTimer);
			debounceTimer = null;
		}
	};

	const doSearch = async (q: string): Promise<void> => {
		if (!workspaceId || q.trim().length === 0) {
			results = [];
			loading = false;
			return;
		}
		loading = true;
		try {
			const fetched = await searchWorkspaceRepoFiles(workspaceId, q.trim(), 50);
			// Only update if query hasn't changed during the async call
			if (query.trim() === q.trim()) {
				results = fetched;
				selectedIndex = 0;
			}
		} catch {
			results = [];
		} finally {
			loading = false;
		}
	};

	const handleInput = (e: Event): void => {
		const value = (e.currentTarget as HTMLInputElement).value;
		query = value;
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			void doSearch(value);
		}, 100);
	};

	const selectResult = (result: RepoFileSearchResult): void => {
		onSelectFile(result.repoId, result.path);
		onClose();
		resetState();
	};

	const onKeydown = (event: KeyboardEvent): void => {
		if (!open) return;
		if (event.key === 'Escape') {
			event.preventDefault();
			onClose();
			resetState();
			return;
		}
		if (event.key === 'Tab') {
			event.preventDefault();
			inputRef?.focus();
			return;
		}
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			selectedIndex = Math.min(selectedIndex + 1, Math.max(results.length - 1, 0));
			return;
		}
		if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedIndex = Math.max(selectedIndex - 1, 0);
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			const current = results[selectedIndex];
			if (current) {
				selectResult(current);
			}
		}
	};

	$effect(() => {
		if (open) {
			previousFocus = document.activeElement as HTMLElement | null;
			selectedIndex = 0;
			requestAnimationFrame(() => {
				inputRef?.focus();
			});
		} else if (previousFocus) {
			previousFocus.focus();
			previousFocus = null;
		}
	});
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<div class="palette-overlay" role="presentation">
		<button type="button" class="overlay-dismiss" aria-label="Close file search" onclick={onClose}
		></button>
		<div class="palette" role="dialog" aria-modal="true" aria-label="File search" tabindex="-1">
			<div class="search-row">
				<span class="search-icon"><Search size={16} /></span>
				<input
					class="ws-field-input ws-field-input--ghost"
					bind:this={inputRef}
					type="text"
					placeholder="Search files by name..."
					value={query}
					oninput={handleInput}
				/>
				<kbd class="kbd ui-kbd">esc</kbd>
			</div>
			<div class="result-list">
				{#if results.length === 0 && query.trim().length > 0 && !loading}
					<div class="empty ws-empty-state">
						<p class="ws-empty-state-copy">No matching files</p>
					</div>
				{:else if results.length === 0 && query.trim().length === 0}
					<div class="empty ws-empty-state">
						<p class="ws-empty-state-copy">Type to search files...</p>
					</div>
				{:else}
					{#each results as result, i (result.repoId + ':' + result.path)}
						<button
							type="button"
							class:selected={selectedIndex === i}
							onmouseenter={() => (selectedIndex = i)}
							onclick={() => selectResult(result)}
						>
							<span class="file-icon">
								<Icon icon={getRepoFileIcon(result.path)} width="14" />
							</span>
							<span class="text">
								<span class="label">{result.path.split('/').pop()}</span>
								<span class="description">{result.path}</span>
							</span>
							<span class="repo-name">{result.repoName}</span>
						</button>
					{/each}
				{/if}
			</div>
			<div class="footer">
				<span><kbd class="kbd ui-kbd">↑↓</kbd> navigate</span>
				<span><kbd class="kbd ui-kbd">↵</kbd> open</span>
				<span><kbd class="kbd ui-kbd">esc</kbd> close</span>
			</div>
		</div>
	</div>
{/if}

<style>
	.palette-overlay {
		position: fixed;
		inset: 0;
		z-index: 400;
		display: grid;
		place-items: start center;
		padding-top: 80px;
		background: rgba(3, 6, 10, 0.58);
		backdrop-filter: blur(6px);
		-webkit-backdrop-filter: blur(6px);
	}

	.overlay-dismiss {
		position: absolute;
		inset: 0;
		border: none;
		background: transparent;
		padding: 0;
	}

	.palette {
		position: relative;
		z-index: 1;
		width: min(620px, calc(100vw - 24px));
		border-radius: 14px;
		border: 1px solid var(--glass-border);
		background: var(--glass-bg-strong);
		backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		-webkit-backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		overflow: hidden;
		box-shadow: var(--glass-shadow), var(--inset-highlight);
	}

	.search-row {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px;
		border-bottom: 1px solid var(--border);
	}

	.search-icon {
		color: var(--muted);
	}

	.search-row input {
		flex: 1;
		font-size: var(--text-md);
	}

	.result-list {
		max-height: min(48vh, 420px);
		overflow: auto;
		padding: 8px;
		display: grid;
		gap: 2px;
	}

	.result-list button {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		text-align: left;
		padding: 8px;
		border-radius: 10px;
		border: 1px solid transparent;
		background: transparent;
		color: inherit;
		cursor: pointer;
	}

	.result-list button:hover {
		background: var(--hover-bg-solid);
		border-color: var(--border);
	}

	.result-list button.selected {
		background: var(--active-accent-bg);
		border-color: var(--active-accent-border);
	}

	.file-icon {
		display: inline-grid;
		place-items: center;
		flex-shrink: 0;
		color: var(--muted);
	}

	.text {
		display: inline-grid;
		gap: 1px;
		flex: 1;
		min-width: 0;
	}

	.label {
		font-size: var(--text-base);
		color: var(--text);
	}

	.description {
		font-size: var(--text-xs);
		color: var(--muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.repo-name {
		font-size: var(--text-xs);
		color: var(--subtle);
		flex-shrink: 0;
		font-family: var(--font-mono);
	}

	.empty {
		padding: 20px;
	}

	.footer {
		display: flex;
		gap: 16px;
		padding: 8px 12px;
		border-top: 1px solid var(--border);
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.kbd {
		justify-content: center;
		min-width: 20px;
		height: 18px;
		padding: 0 4px;
	}
</style>
