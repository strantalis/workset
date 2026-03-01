<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { fly, fade, scale } from 'svelte/transition';
	import {
		ArrowRight,
		Check,
		ChevronLeft,
		Database,
		GitBranch,
		LayoutTemplate,
		Loader2,
		AlignLeft,
		Zap,
		AlertTriangle,
	} from '@lucide/svelte';
	import { searchGitHubRepositories } from '../../api/github';
	import { deriveRepoName, looksLikeUrl } from '../../names';
	import { languageColors } from '../../view-models/onboardingViewModel';
	import type { GitHubRepoSearchItem, HookExecution } from '../../types';
	import { subscribeHookProgressEvent } from '../../hookEventService';
	import { formatWorkspaceActionError } from '../../services/workspaceActionErrors';
	import {
		applyHookProgress,
		appendHookRuns,
		beginHookTracking,
		clearHookTracking,
		shouldTrackHookEvent,
		type WorkspaceActionPendingHook,
	} from '../../services/workspaceActionHooks';
	import {
		runWorkspaceActionPendingHook,
		trustWorkspaceActionPendingHook,
	} from '../../services/workspaceActionModalActions';
	import {
		computeRepoPositions,
		getStepStatus,
		resolveHookPreviewSource,
		type OnboardingDraft,
		type OnboardingStartResult,
		type RegisteredRepo,
		type RepoTemplate,
		type ReviewRepoEntry,
	} from './OnboardingView.utils';

	interface Props {
		busy?: boolean;
		catalogLoading?: boolean;
		errorMessage?: string | null;
		defaultWorkspaceName?: string;
		existingWorkspaceNames?: string[];
		repoRegistry?: RegisteredRepo[];
		onStart?: (draft: OnboardingDraft) => Promise<OnboardingStartResult | void>;
		onPreviewHooks?: (source: string) => Promise<string[]>;
		onComplete?: (workspaceName: string) => void | Promise<void>;
		onCancel?: () => void;
	}

	const {
		busy = false,
		catalogLoading = false,
		errorMessage = null,
		defaultWorkspaceName = '',
		existingWorkspaceNames = [],
		repoRegistry = [],
		onStart,
		onPreviewHooks,
		onComplete,
		onCancel,
	}: Props = $props();

	let step = $state(1);

	let formName = $state('');
	let formDescription = $state('');
	let threadName = $state('');
	let threadNameTouched = $state(false);
	let featureBranch = $state('');
	let featureBranchTouched = $state(false);
	let reviewDetailsExpanded = $state(false);
	let sourceInput = $state('');
	let selectedAliases = $state<Set<string>>(new Set());
	let directRepos = $state<Array<{ url: string; register: boolean }>>([]);
	let remoteSuggestions = $state<GitHubRepoSearchItem[]>([]);
	let searchLoading = $state(false);
	let searchError: string | null = $state(null);
	let lastSearchedQuery = $state('');
	let sourceSearchDebounce: ReturnType<typeof setTimeout> | null = null;
	let sourceSearchSequence = 0;
	let nameTouched = $state(false);
	let runError = $state<string | null>(null);
	let hookPreviewLoading = $state(false);
	let hookPreviewError = $state<string | null>(null);
	let hookPreviewBySource = $state<Record<string, string[]>>({});
	let hookPreviewSequence = 0;

	let isInitializing = $state(false);
	let initializeStarted = $state(false);
	let initializedWorkspaceName = $state<string | null>(null);
	let hookWarnings = $state<string[]>([]);
	let pendingHooks = $state<WorkspaceActionPendingHook[]>([]);
	let hookRuns = $state<HookExecution[]>([]);
	let activeHookOperation = $state<string | null>(null);
	let activeHookWorkspace = $state<string | null>(null);
	let hookEventUnsubscribe: (() => void) | null = null;

	const duplicateWorkspaceMessage = (name: string): string =>
		`A workset named "${name}" already exists.`;

	const normalizedWorkspaceNames = $derived.by(() =>
		existingWorkspaceNames.map((name) => name.trim()).filter((name) => name.length > 0),
	);
	const trimmedWorkspaceName = $derived(formName.trim());
	const trimmedThreadName = $derived(threadName.trim());
	const isDuplicateWorkspaceName = $derived(
		trimmedWorkspaceName.length > 0 && normalizedWorkspaceNames.includes(trimmedWorkspaceName),
	);
	const workspaceNameValidationError = $derived.by(() =>
		isDuplicateWorkspaceName ? duplicateWorkspaceMessage(trimmedWorkspaceName) : null,
	);

	const hookPreviewEnabled = $derived(!!onPreviewHooks);

	const isLikelyLocalPath = (value: string): boolean => {
		const trimmed = value.trim();
		return (
			trimmed.startsWith('/') ||
			trimmed.startsWith('./') ||
			trimmed.startsWith('../') ||
			trimmed.startsWith('~') ||
			/^[a-zA-Z]:[\\/]/.test(trimmed) ||
			trimmed.includes('\\')
		);
	};
	const sourceQuery = $derived(sourceInput.trim());
	const canAddSource = $derived(looksLikeUrl(sourceQuery) || isLikelyLocalPath(sourceQuery));
	const showSearchStartHint = $derived(sourceQuery.length === 0);
	const shouldSearchRemote = (value: string): boolean => {
		const trimmed = value.trim();
		return trimmed.length >= 2 && !looksLikeUrl(trimmed) && !isLikelyLocalPath(trimmed);
	};
	const showSearchMinCharsHint = $derived(
		sourceQuery.length > 0 &&
			sourceQuery.length < 2 &&
			!looksLikeUrl(sourceQuery) &&
			!isLikelyLocalPath(sourceQuery),
	);
	const showNoSearchResults = $derived(
		!searchLoading &&
			searchError === null &&
			!showSearchStartHint &&
			!showSearchMinCharsHint &&
			remoteSuggestions.length === 0 &&
			lastSearchedQuery !== '' &&
			sourceQuery === lastSearchedQuery,
	);
	const isDirectRepoSelected = (url: string): boolean =>
		directRepos.some((entry) => entry.url === url);
	const isCatalogRepoSelectedByUrl = (url: string): boolean =>
		selectedCatalogRepos.some((entry) => entry.remoteUrl === url);
	const filteredRemoteSuggestions = $derived.by<GitHubRepoSearchItem[]>(() =>
		remoteSuggestions.filter((item) => {
			const source = item.sshUrl || item.cloneUrl;
			if (!source) return false;
			return !isDirectRepoSelected(source) && !isCatalogRepoSelectedByUrl(source);
		}),
	);

	const selectedCatalogRepos = $derived.by<RegisteredRepo[]>(() =>
		repoRegistry.filter((repo) => selectedAliases.has(repo.aliasName)),
	);
	const selectedRepoCount = $derived(selectedAliases.size + directRepos.length);
	const selectedRepoItems = $derived.by<
		Array<{ key: string; label: string; value: string; kind: 'catalog' | 'direct' }>
	>(() => [
		...selectedCatalogRepos.map((repo) => ({
			key: `catalog:${repo.aliasName}`,
			label: repo.name,
			value: repo.aliasName,
			kind: 'catalog' as const,
		})),
		...directRepos.map((repo) => ({
			key: `direct:${repo.url}`,
			label: deriveRepoName(repo.url) || repo.url,
			value: repo.url,
			kind: 'direct' as const,
		})),
	]);
	const nextStepLabel = $derived(
		selectedRepoCount > 0 ? `Next Step (${selectedRepoCount} repos)` : 'Next Step',
	);

	const reviewRepos = $derived.by<RepoTemplate[]>(() => {
		const fromCatalog = selectedCatalogRepos.map((repo) => ({
			name: repo.name,
			remoteUrl: repo.remoteUrl,
			hooks: [],
			aliasName: repo.aliasName,
			sourceType: 'alias' as const,
		}));
		const fromDirect = directRepos.map((repo) => ({
			name: deriveRepoName(repo.url) || repo.url,
			remoteUrl: repo.url,
			hooks: [],
			sourceType: 'direct' as const,
		}));
		return [...fromCatalog, ...fromDirect];
	});

	const reviewRepoEntries = $derived.by<ReviewRepoEntry[]>(() => {
		return reviewRepos.map((repo, index) => {
			const source = resolveHookPreviewSource(repo);
			return {
				key: `${repo.name}-${index}`,
				repo,
				source,
				hooks: hookPreviewBySource[source] ?? [],
			};
		});
	});
	const hasPreviewedHooks = $derived.by(() =>
		reviewRepoEntries.some((entry) => entry.hooks.length > 0),
	);

	const canProceedStep2 = $derived.by(() => {
		return selectedAliases.size > 0 || directRepos.length > 0;
	});

	const filteredRegistry = $derived(
		repoRegistry.filter((r) => {
			const query = sourceQuery.toLowerCase();
			if (!query) return true;
			if (looksLikeUrl(sourceQuery) || isLikelyLocalPath(sourceQuery)) return false;
			return (
				r.name.toLowerCase().includes(query) ||
				r.tags.some((t) => t.includes(query)) ||
				r.remoteUrl.toLowerCase().includes(query)
			);
		}),
	);

	const hasPendingHooksToResolve = $derived(
		pendingHooks.some((pending) => pending.trusted !== true),
	);
	const canOpenWorkset = $derived(
		initializedWorkspaceName !== null && !isInitializing && !hasPendingHooksToResolve,
	);

	const topoRepos = $derived.by<RepoTemplate[]>(() => reviewRepos);

	const repoPositions = $derived.by(() => computeRepoPositions(topoRepos));

	const toSlug = (value: string): string =>
		value
			.toLowerCase()
			.trim()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '');

	$effect(() => {
		if (!nameTouched) {
			formName = defaultWorkspaceName;
		}
	});

	$effect(() => {
		if (threadNameTouched) return;
		threadName = trimmedWorkspaceName;
	});

	$effect(() => {
		if (featureBranchTouched) return;
		const threadSlug = toSlug(trimmedThreadName || trimmedWorkspaceName);
		featureBranch = threadSlug ? `feature/${threadSlug}` : '';
	});

	const nextStep = () => {
		step += 1;
	};
	const prevStep = () => {
		step -= 1;
	};

	const clearHookPreviewState = (): void => {
		hookPreviewSequence += 1;
		hookPreviewBySource = {};
		hookPreviewError = null;
		hookPreviewLoading = false;
	};

	const clearSourceTimers = (): void => {
		if (sourceSearchDebounce) {
			clearTimeout(sourceSearchDebounce);
			sourceSearchDebounce = null;
		}
	};

	const resetRemoteSuggestions = (): void => {
		clearSourceTimers();
		sourceSearchSequence += 1;
		remoteSuggestions = [];
		searchLoading = false;
		searchError = null;
		lastSearchedQuery = '';
	};

	const showRemoteSearchHints = (query: string): void => {
		sourceSearchSequence += 1;
		remoteSuggestions = [];
		searchLoading = false;
		searchError = null;
		lastSearchedQuery = query;
	};

	const toSearchErrorMessage = (err: unknown): string => {
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
	};

	const runRemoteSearch = async (query: string): Promise<void> => {
		const requestSequence = ++sourceSearchSequence;
		searchLoading = true;
		searchError = null;
		lastSearchedQuery = query;
		try {
			const results = await searchGitHubRepositories(query, 8);
			if (requestSequence !== sourceSearchSequence) return;
			remoteSuggestions = results;
		} catch (err) {
			if (requestSequence !== sourceSearchSequence) return;
			remoteSuggestions = [];
			searchError = toSearchErrorMessage(err);
		} finally {
			if (requestSequence === sourceSearchSequence) {
				searchLoading = false;
			}
		}
	};

	const queueRemoteSearch = (value: string): void => {
		const query = value.trim();
		if (sourceSearchDebounce) {
			clearTimeout(sourceSearchDebounce);
			sourceSearchDebounce = null;
		}
		if (query.length === 0) {
			showRemoteSearchHints('');
			return;
		}
		if (!shouldSearchRemote(query)) {
			if (looksLikeUrl(query) || isLikelyLocalPath(query)) {
				resetRemoteSuggestions();
				return;
			}
			showRemoteSearchHints(query);
			return;
		}
		sourceSearchDebounce = setTimeout(() => {
			void runRemoteSearch(query);
		}, 250);
	};

	const handleSourceInput = (value: string): void => {
		sourceInput = value;
		queueRemoteSearch(value);
	};

	const handleAddRemoteSuggestion = (suggestion: GitHubRepoSearchItem): void => {
		const source = (suggestion.sshUrl || suggestion.cloneUrl || '').trim();
		if (!source || isDirectRepoSelected(source) || isCatalogRepoSelectedByUrl(source)) return;
		directRepos = [...directRepos, { url: source, register: true }];
		clearHookPreviewState();
		sourceInput = '';
		resetRemoteSuggestions();
	};

	const handleSourceKeydown = (event: KeyboardEvent): void => {
		if (event.key === 'Enter' && canAddSource) {
			event.preventDefault();
			handleAddDirectRepo();
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			resetRemoteSuggestions();
		}
	};

	const handleAddDirectRepo = (): void => {
		const next = sourceQuery;
		if (!canAddSource || next.length === 0) return;
		if (!directRepos.some((entry) => entry.url === next)) {
			directRepos = [...directRepos, { url: next, register: true }];
			clearHookPreviewState();
		}
		sourceInput = '';
		resetRemoteSuggestions();
	};

	const handleRemoveDirectRepo = (url: string): void => {
		directRepos = directRepos.filter((entry) => entry.url !== url);
		clearHookPreviewState();
	};

	const handleToggleRegistryRepo = (repo: RegisteredRepo): void => {
		const next = new Set(selectedAliases);
		if (next.has(repo.aliasName)) {
			next.delete(repo.aliasName);
		} else {
			next.add(repo.aliasName);
		}
		selectedAliases = next;
		clearHookPreviewState();
	};

	const handleRemoveCatalogAlias = (aliasName: string): void => {
		const next = new Set(selectedAliases);
		if (!next.delete(aliasName)) return;
		selectedAliases = next;
		clearHookPreviewState();
	};

	const previewHooksForReview = async (): Promise<void> => {
		if (!onPreviewHooks) return;
		const repos = reviewRepos;
		if (repos.length === 0) {
			clearHookPreviewState();
			return;
		}

		const sequence = ++hookPreviewSequence;
		hookPreviewError = null;
		hookPreviewLoading = true;
		const previewed: Record<string, string[]> = {};
		const failedRepos: string[] = [];

		await Promise.all(
			repos.map(async (repo) => {
				const source = resolveHookPreviewSource(repo);
				if (!source) return;
				try {
					const hooks = await onPreviewHooks(source);
					const normalized = hooks.map((hook) => hook.trim()).filter((hook) => hook.length > 0);
					if (normalized.length > 0) {
						previewed[source] = normalized;
					}
				} catch {
					failedRepos.push(repo.name || source);
				}
			}),
		);

		if (sequence !== hookPreviewSequence) return;
		hookPreviewBySource = previewed;
		if (failedRepos.length > 0) {
			const message =
				failedRepos.length === 1
					? `Unable to preview hooks for ${failedRepos[0]}. Initialization will still work.`
					: `Unable to preview hooks for ${failedRepos.length} repositories. Initialization will still work.`;
			hookPreviewError = message;
		}
		hookPreviewLoading = false;
	};

	const handleGoToReview = () => {
		nextStep();
		void previewHooksForReview();
	};

	const handleRunPendingHook = async (pending: WorkspaceActionPendingHook): Promise<void> => {
		await runWorkspaceActionPendingHook({
			pending,
			pendingHooks,
			hookRuns,
			workspaceReferences: [initializedWorkspaceName, activeHookWorkspace, trimmedWorkspaceName],
			activeHookOperation,
			getPendingHooks: () => pendingHooks,
			getHookRuns: () => hookRuns,
			setPendingHooks: (next) => (pendingHooks = next),
			setHookRuns: (next) => (hookRuns = next),
		});
	};

	const handleTrustPendingHook = async (pending: WorkspaceActionPendingHook): Promise<void> => {
		await trustWorkspaceActionPendingHook({
			pending,
			pendingHooks,
			hookRuns,
			workspaceReferences: [initializedWorkspaceName, activeHookWorkspace, trimmedWorkspaceName],
			activeHookOperation,
			getPendingHooks: () => pendingHooks,
			getHookRuns: () => hookRuns,
			setPendingHooks: (next) => (pendingHooks = next),
			setHookRuns: (next) => (hookRuns = next),
		});
	};

	const handleComplete = async (): Promise<void> => {
		if (!initializedWorkspaceName) return;
		await onComplete?.(initializedWorkspaceName);
	};

	const handleInitialize = async () => {
		if (canOpenWorkset) {
			await handleComplete();
			return;
		}
		if (initializeStarted) return;
		if (!trimmedWorkspaceName) {
			runError = 'Workset name is required.';
			return;
		}
		if (isDuplicateWorkspaceName) {
			runError = duplicateWorkspaceMessage(trimmedWorkspaceName);
			return;
		}
		if (!trimmedThreadName) {
			runError = 'First thread name is required.';
			return;
		}

		runError = null;
		isInitializing = true;
		initializeStarted = true;
		hookWarnings = [];
		pendingHooks = [];
		hookRuns = [];
		initializedWorkspaceName = null;

		const repos = reviewRepos;
		const selectedAliasNames = Array.from(selectedAliases);
		const primarySource = '';
		const directRepoEntries = directRepos.map((repo) => ({ ...repo }));

		try {
			({ activeHookOperation, activeHookWorkspace, hookRuns, pendingHooks } = beginHookTracking(
				'workspace.create',
				trimmedThreadName,
			));

			const result = await onStart?.({
				worksetName: trimmedWorkspaceName,
				threadName: trimmedThreadName,
				featureBranch: featureBranch.trim(),
				description: formDescription.trim(),
				repos: repos.map((r) => ({ ...r, hooks: [...(r.hooks ?? [])] })),
				selectedAliases: selectedAliasNames,
				primarySource,
				directRepos: directRepoEntries,
			});

			initializedWorkspaceName = result?.workspaceName ?? trimmedThreadName;
			hookWarnings = result?.warnings ?? [];
			hookRuns = appendHookRuns(hookRuns, result?.hookRuns ?? []);
			pendingHooks = (result?.pendingHooks ?? []).map((pending) => ({ ...pending }));
		} catch (error) {
			runError = formatWorkspaceActionError(error, 'Failed to initialize workset.');
			initializeStarted = false;
			isInitializing = false;
			({ activeHookOperation, activeHookWorkspace } = clearHookTracking());
			return;
		}
		isInitializing = false;
		({ activeHookOperation, activeHookWorkspace } = clearHookTracking());
	};

	const stepStatus = (s: number): 'active' | 'completed' | 'pending' => getStepStatus(step, s);

	const hookRunDotClass = (status: HookExecution['status']): string | null => {
		if (status === 'ok') return 'ws-dot-clean';
		if (status === 'failed') return 'ws-dot-error';
		if (status === 'running') return 'ws-dot-ahead';
		return null;
	};

	onMount(() => {
		hookEventUnsubscribe = subscribeHookProgressEvent((payload) => {
			if (
				!shouldTrackHookEvent(payload, {
					activeHookOperation,
					activeHookWorkspace,
					loading: isInitializing,
				})
			) {
				return;
			}
			hookRuns = applyHookProgress(hookRuns, payload);
		});
	});

	onDestroy(() => {
		clearSourceTimers();
		hookEventUnsubscribe?.();
		hookEventUnsubscribe = null;
	});
</script>

<div class="onboarding-shell">
	<div class="onboarding-inner">
		<div class="form-side">
			<div class="form-header">
				<div class="form-header-top">
					<h1>Create Workset</h1>
					{#if onCancel}
						<button type="button" class="header-cancel" onclick={onCancel} disabled={busy}>
							Cancel
						</button>
					{/if}
				</div>
				<p>Group repos and create your first feature thread.</p>
			</div>

			<div class="step-rail">
				<div class="progress-line"></div>

				<div class="step-group">
					<div class="step-indicator-row">
						<div
							class="step-dot"
							class:active={stepStatus(1) === 'active'}
							class:completed={stepStatus(1) === 'completed'}
						>
							{#if stepStatus(1) === 'completed'}
								<Check size={16} />
							{:else}
								<span class="step-num">1</span>
							{/if}
						</div>
						<span class="step-label" class:active={stepStatus(1) === 'active'}
							>Workset Identity</span
						>
					</div>

					{#if step === 1}
						<div class="step-content" in:fly={{ x: -20, duration: 200 }}>
							<div class="field-group">
								<label class="field">
									<span class="field-label">Workset Name</span>
									<input
										type="text"
										bind:value={formName}
										oninput={() => (nameTouched = true)}
										placeholder="e.g., payment-system-v2"
										autocapitalize="off"
										autocorrect="off"
										spellcheck="false"
									/>
									{#if workspaceNameValidationError}
										<p class="field-error">{workspaceNameValidationError}</p>
									{/if}
								</label>
								<label class="field">
									<span class="field-label">
										Description
										<span class="field-hint-inline">what are you working on?</span>
									</span>
									<textarea
										bind:value={formDescription}
										placeholder="e.g., Migrating auth service to OAuth2 + billing rewrite for Stripe Connect"
										rows="2"
									></textarea>
									<p class="field-hint">
										This shows in the workset switcher so you remember what you were doing.
									</p>
								</label>
								<button
									type="button"
									class="btn-primary"
									onclick={nextStep}
									disabled={!trimmedWorkspaceName || isDuplicateWorkspaceName || busy}
								>
									Continue <ArrowRight size={16} />
								</button>
							</div>
						</div>
					{/if}
				</div>

				<div class="step-group">
					<div class="step-indicator-row">
						<div
							class="step-dot"
							class:active={stepStatus(2) === 'active'}
							class:completed={stepStatus(2) === 'completed'}
						>
							{#if stepStatus(2) === 'completed'}
								<Check size={16} />
							{:else}
								<span class="step-num">2</span>
							{/if}
						</div>
						<span class="step-label" class:active={stepStatus(2) === 'active'}
							>Add Repositories</span
						>
					</div>

					{#if step === 2}
						<div class="step-content" in:fly={{ x: -20, duration: 200 }}>
							<div class="single-repo-content" in:fly={{ y: 10, duration: 180 }}>
								<div class="field">
									<span class="field-label-sm">Repositories</span>
									<div class="repo-input-row">
										<div class="repo-input-shell">
											<input
												type="text"
												value={sourceInput}
												oninput={(event) =>
													handleSourceInput((event.currentTarget as HTMLInputElement).value)}
												onkeydown={handleSourceKeydown}
												placeholder="Search catalog/GitHub, or paste repo URL/path"
												class="mono"
												autocapitalize="off"
												autocorrect="off"
												spellcheck="false"
											/>
										</div>
										<button
											type="button"
											class="repo-add-btn"
											onclick={handleAddDirectRepo}
											disabled={!canAddSource}
										>
											Add
										</button>
									</div>
									{#if showSearchMinCharsHint}
										<div class="repo-search-status">
											Type at least 2 characters to search GitHub.
										</div>
									{:else if searchLoading}
										<div class="repo-search-status">
											<Loader2 size={14} />
											<span>Searching GitHub…</span>
										</div>
									{:else if searchError}
										<div class="repo-search-error">{searchError}</div>
									{:else if showNoSearchResults}
										<div class="repo-search-status">
											No GitHub repositories found for "{sourceQuery}".
										</div>
									{/if}
								</div>

								{#if selectedRepoCount > 0}
									<div class="selected-repos-panel">
										<div class="selected-repos-header">
											<span>Selected Repositories</span>
											<span>{selectedRepoCount}</span>
										</div>
										<div class="selected-repos-list">
											{#each selectedRepoItems as item (item.key)}
												<button
													type="button"
													class="selected-repo-chip"
													onclick={() =>
														item.kind === 'catalog'
															? handleRemoveCatalogAlias(item.value)
															: handleRemoveDirectRepo(item.value)}
												>
													<span>{item.label}</span>
													<span class="selected-repo-remove">x</span>
												</button>
											{/each}
										</div>
									</div>
								{/if}

								<div class="field">
									<span class="field-label-sm">Repository Results</span>
									<div class="registry-list">
										{#each filteredRegistry as repo (repo.id)}
											{@const isSelected = selectedAliases.has(repo.aliasName)}
											{@const langColor = languageColors[repo.language] ?? '#A3B5C9'}
											<button
												type="button"
												class="registry-item"
												class:selected={isSelected}
												onclick={() => handleToggleRegistryRepo(repo)}
											>
												<div class="registry-check" class:checked={isSelected}>
													{#if isSelected}<Check size={10} />{/if}
												</div>
												<div class="registry-info">
													<div class="registry-name-row">
														<span class="registry-name">{repo.name}</span>
														<span class="source-badge source-badge-catalog">Catalog</span>
														<span
															class="lang-badge"
															style="color: {langColor}; background: {langColor}15; border-color: {langColor}30"
														>
															{repo.language}
														</span>
													</div>
													<div class="registry-url">{repo.remoteUrl}</div>
												</div>
											</button>
										{/each}

										{#if filteredRemoteSuggestions.length > 0}
											{#each filteredRemoteSuggestions as suggestion (`${suggestion.owner}/${suggestion.name}`)}
												<button
													type="button"
													class="registry-item github-result"
													onclick={() => handleAddRemoteSuggestion(suggestion)}
												>
													<div class="registry-check github-result-check">
														<ArrowRight size={10} />
													</div>
													<div class="registry-info">
														<div class="registry-name-row">
															<span class="registry-name">{suggestion.owner}/{suggestion.name}</span
															>
															<span class="lang-badge github-source-badge">GitHub</span>
														</div>
														<div class="registry-url">
															{suggestion.sshUrl || suggestion.cloneUrl}
														</div>
													</div>
												</button>
											{/each}
										{/if}

										{#if filteredRegistry.length === 0 && filteredRemoteSuggestions.length === 0}
											<div class="registry-empty">
												{#if catalogLoading}
													Loading repos from catalog…
												{:else if sourceQuery.length > 0}
													No matching repositories.
												{:else}
													No repositories available yet.
												{/if}
											</div>
										{/if}
									</div>
								</div>

								<div class="hooks-discovery-note" in:fly={{ y: 6, duration: 150 }}>
									{#if selectedRepoCount === 0}
										Select at least one repository to continue.
									{:else}
										{selectedRepoCount} repos selected. Lifecycle hooks are previewed before initialization.
									{/if}
								</div>
							</div>

							<div class="step2-nav">
								<button type="button" class="back-btn" onclick={prevStep}>
									<ChevronLeft size={20} />
								</button>
								<button
									type="button"
									class="btn-primary wide"
									onclick={handleGoToReview}
									disabled={!canProceedStep2 || busy}
								>
									{nextStepLabel}
								</button>
							</div>
						</div>
					{/if}
				</div>

				<div class="step-group">
					<div class="step-indicator-row">
						<div
							class="step-dot"
							class:active={stepStatus(3) === 'active'}
							class:completed={stepStatus(3) === 'completed'}
						>
							{#if stepStatus(3) === 'completed'}
								<Check size={16} />
							{:else}
								<span class="step-num">3</span>
							{/if}
						</div>
						<span class="step-label" class:active={stepStatus(3) === 'active'}
							>First Thread &amp; Review</span
						>
					</div>

					{#if step === 3}
						<div class="step-content" in:fly={{ x: -20, duration: 200 }}>
							<div class="first-thread-note">
								Every workset needs at least one thread to initialize.
							</div>
							<label class="field">
								<span class="field-label-sm">Thread Name</span>
								<input
									type="text"
									value={threadName}
									oninput={(event) => {
										threadNameTouched = true;
										threadName = (event.currentTarget as HTMLInputElement).value;
									}}
									placeholder="e.g., OAuth2 Migration"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
							</label>
							<label class="field">
								<span class="field-label-sm">Feature Branch (optional)</span>
								<input
									type="text"
									value={featureBranch}
									oninput={(event) => {
										featureBranchTouched = true;
										featureBranch = (event.currentTarget as HTMLInputElement).value;
									}}
									placeholder="e.g., feature/oauth2-migration"
									class="mono"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
							</label>
							<button
								type="button"
								class="review-toggle"
								aria-expanded={reviewDetailsExpanded}
								onclick={() => (reviewDetailsExpanded = !reviewDetailsExpanded)}
							>
								<span>{reviewDetailsExpanded ? 'Hide review details' : 'Show review details'}</span>
								<span class="review-toggle-meta"
									>{reviewRepoEntries.length} repo{reviewRepoEntries.length === 1 ? '' : 's'}</span
								>
							</button>

							{#if reviewDetailsExpanded}
								<div class="review-card">
									<div class="review-meta">Creating workset:</div>
									<div class="review-name">{formName}</div>
									<div class="review-thread-row">
										<GitBranch size={12} />
										<span class="review-thread-name"
											>{trimmedThreadName || 'Name your first thread'}</span
										>
									</div>
									{#if featureBranch.trim().length > 0}
										<div class="review-thread-branch">{featureBranch.trim()}</div>
									{/if}
									{#if formDescription}
										<div class="review-desc-row">
											<AlignLeft size={11} />
											<p>{formDescription}</p>
										</div>
									{/if}
									<div class="review-mode-badge">
										<GitBranch size={11} /> Repository Setup
									</div>

									<div class="review-repos-label">Repository:</div>
									<ul class="review-repo-list">
										{#each reviewRepoEntries as entry (entry.key)}
											{@const repo = entry.repo}
											<li>
												<div class="review-repo-header">
													<GitBranch size={14} class="review-repo-icon" />
													<span>{repo.name}</span>
													{#if repo.remoteUrl}
														<span class="review-repo-url">({repo.remoteUrl})</span>
													{/if}
												</div>
											</li>
										{/each}
									</ul>
									{#if hookPreviewEnabled}
										{#if hookPreviewLoading}
											<div class="review-hooks-status">
												<span class="hook-spin"><Loader2 size={13} /></span>
												<span>Checking lifecycle hooks in repository config…</span>
											</div>
										{:else if hookPreviewError}
											<div class="review-hooks-warning">
												<AlertTriangle size={12} />
												<span>{hookPreviewError}</span>
											</div>
										{/if}

										{#if hasPreviewedHooks}
											<div class="review-hooks-label">Discovered lifecycle hooks</div>
											<ul class="review-hooks-list">
												{#each reviewRepoEntries as entry (entry.key)}
													{#if entry.hooks.length > 0}
														<li class="review-hooks-item">
															<span class="review-hooks-repo">{entry.repo.name}</span>
															<div class="review-hooks-chip-row">
																{#each entry.hooks as hook (`${entry.key}-${hook}`)}
																	<span class="review-hooks-chip">{hook}</span>
																{/each}
															</div>
														</li>
													{/if}
												{/each}
											</ul>
										{:else if !hookPreviewLoading}
											<div class="review-no-hooks">
												No lifecycle hooks found in repository config.
											</div>
										{/if}
									{:else}
										<div class="review-no-hooks">
											Lifecycle hooks are discovered from repository config when initialization
											starts.
										</div>
									{/if}
								</div>
							{:else}
								<div class="review-collapsed-note">
									Review details are collapsed. Expand to inspect repositories and hook previews.
								</div>
							{/if}

							{#if initializeStarted}
								<div class="hook-runtime-card" in:fade={{ duration: 180 }}>
									{#if isInitializing}
										<div class="hook-runtime-status">
											<span class="hook-spin"><Loader2 size={13} /></span>
											<span>Cloning repositories and discovering lifecycle hooks…</span>
										</div>
									{/if}

									{#if hookWarnings.length > 0}
										<div class="hook-warning-list">
											{#each hookWarnings as warning (warning)}
												<div class="hook-warning-item">
													<AlertTriangle size={12} />
													<span>{warning}</span>
												</div>
											{/each}
										</div>
									{/if}

									{#if hookRuns.length > 0}
										<div class="hook-runs-list">
											{#each hookRuns as run (`${run.repo}:${run.event}:${run.id}`)}
												<div class="hook-run-row">
													<span class="hook-run-repo">{run.repo}</span>
													<span class="hook-run-id">{run.id}</span>
													<span
														class="hook-run-status ws-inline"
														class:ok={run.status === 'ok'}
														class:failed={run.status === 'failed'}
														class:running-status={run.status === 'running'}
														class:skipped={run.status === 'skipped'}
													>
														{#if hookRunDotClass(run.status)}
															<span
																class={`ws-dot ws-dot-sm ${hookRunDotClass(run.status)}`}
																aria-hidden="true"
															></span>
														{/if}
														{run.status}
													</span>
												</div>
											{/each}
										</div>
									{/if}

									{#if pendingHooks.length > 0}
										<div class="pending-hooks-list">
											{#each pendingHooks as pending (`${pending.repo}:${pending.event}`)}
												<div class="pending-hook-row">
													<div class="pending-hook-copy">
														<div class="pending-hook-title">
															<Zap size={12} />
															<span>{pending.repo}</span>
															{#if pending.trusted}
																<span class="pending-hook-trusted">Trusted</span>
															{/if}
														</div>
														<div class="pending-hook-body">
															{pending.hooks.join(', ')}
														</div>
													</div>
													<div class="ws-pending-hook-actions">
														<button
															type="button"
															class="pending-hook-btn"
															disabled={pending.running || pending.trusted}
															onclick={() => void handleRunPendingHook(pending)}
														>
															{pending.running ? 'Running…' : 'Run now'}
														</button>
														<button
															type="button"
															class="pending-hook-btn ghost"
															disabled={pending.trusting || pending.trusted}
															onclick={() => void handleTrustPendingHook(pending)}
														>
															{pending.trusting
																? 'Trusting…'
																: pending.trusted
																	? 'Trusted'
																	: 'Trust'}
														</button>
													</div>
													{#if pending.runError}
														<div class="pending-hook-error ws-pending-hook-error">
															{pending.runError}
														</div>
													{/if}
												</div>
											{/each}
										</div>
									{/if}
								</div>
							{/if}

							<button
								type="button"
								class="init-btn"
								class:finished={canOpenWorkset}
								class:running={isInitializing && !canOpenWorkset}
								onclick={handleInitialize}
								disabled={busy ||
									isInitializing ||
									(initializeStarted && !canOpenWorkset) ||
									(!initializeStarted && trimmedThreadName.length === 0)}
							>
								{#if canOpenWorkset}
									Open Workset <ArrowRight size={16} />
								{:else if isInitializing || busy}
									Initializing Environment...
								{:else if initializeStarted}
									Resolve Hook Trust To Continue
								{:else if trimmedThreadName.length === 0}
									Name your first thread to continue
								{:else}
									Initialize Workset
								{/if}
							</button>

							{#if runError || errorMessage}
								<div class="init-error">{runError ?? errorMessage}</div>
							{/if}

							{#if !isInitializing && !busy}
								<button type="button" class="back-link" onclick={prevStep}>Back</button>
							{/if}
						</div>
					{/if}
				</div>
			</div>
		</div>

		<div class="topo-side">
			<div class="topo-gradient"></div>

			<h3 class="topo-title ws-section-title">Workset Topology</h3>

			<div class="topo-area">
				<svg class="topo-svg" viewBox="-200 -200 400 400">
					{#each repoPositions as pos, i (pos.name + '-line')}
						<line
							x1="0"
							y1="0"
							x2={pos.x}
							y2={pos.y}
							class="topo-svg-line"
							class:green={true}
							style="animation-delay: {i * 150}ms"
						/>
					{/each}
				</svg>

				<div class="hub-node" class:dim={!formName}>
					<LayoutTemplate size={24} />
					<span class="hub-label">{formName || '...'}</span>
				</div>

				{#if formDescription}
					<div class="hub-desc-callout" in:fade={{ duration: 150 }}>
						<p>{formDescription}</p>
					</div>
				{/if}

				{#each repoPositions as pos, i (pos.name)}
					<div
						class="repo-node"
						class:green={true}
						style="transform: translate({pos.x}px, {pos.y}px)"
						in:scale={{ duration: 250, delay: i * 100 }}
					>
						<GitBranch size={16} />
						<span class="repo-node-label">
							{pos.name.length > 8 ? pos.name.slice(0, 7) + '…' : pos.name}
						</span>
					</div>
				{/each}
			</div>

			<div class="topo-footer">
				<div class="topo-badge">
					<Database size={12} />
					<span>Repo Catalog + Direct Sources</span>
				</div>
			</div>
		</div>
	</div>
</div>

<style src="./OnboardingView.css"></style>
