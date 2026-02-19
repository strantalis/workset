<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { fly, fade, scale } from 'svelte/transition';
	import {
		ArrowRight,
		Check,
		ChevronLeft,
		Code2,
		Database,
		GitBranch,
		LayoutTemplate,
		Loader2,
		Plus,
		Search,
		Server,
		Workflow,
		AlignLeft,
		Zap,
		AlertTriangle,
	} from '@lucide/svelte';
	import { languageColors } from '../../view-models/onboardingViewModel';
	import type { HookExecution } from '../../types';
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
		getTemplateVisual,
		resolveHookPreviewSource,
		type OnboardingDraft,
		type OnboardingStartResult,
		type RegisteredRepo,
		type RepoTemplate,
		type ReviewRepoEntry,
		type WorksetTemplate,
	} from './OnboardingView.utils';

	interface Props {
		busy?: boolean;
		catalogLoading?: boolean;
		errorMessage?: string | null;
		defaultWorkspaceName?: string;
		existingWorkspaceNames?: string[];
		templates?: WorksetTemplate[];
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
		templates = [],
		repoRegistry = [],
		onStart,
		onPreviewHooks,
		onComplete,
		onCancel,
	}: Props = $props();

	let step = $state(1);

	let formName = $state('');
	let formDescription = $state('');
	let mode = $state<'template' | 'single-repo' | null>(null);
	let templateId = $state('');
	let singleRepoSource = $state<'registry' | 'new' | null>(null);
	let registrySearch = $state('');
	let selectedRegistryRepo = $state<RegisteredRepo | null>(null);
	let singleRepo = $state<RepoTemplate>({
		name: '',
		remoteUrl: '',
		hooks: [],
		sourceType: 'direct',
	});
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
	const isDuplicateWorkspaceName = $derived(
		trimmedWorkspaceName.length > 0 && normalizedWorkspaceNames.includes(trimmedWorkspaceName),
	);
	const workspaceNameValidationError = $derived.by(() =>
		isDuplicateWorkspaceName ? duplicateWorkspaceMessage(trimmedWorkspaceName) : null,
	);

	const selectedTemplate = $derived(templates.find((t) => t.id === templateId) ?? null);
	const hookPreviewEnabled = $derived(!!onPreviewHooks);

	const reviewRepos = $derived.by<RepoTemplate[]>(() => {
		if (mode === 'single-repo') return [singleRepo];
		return selectedTemplate?.repos ?? [];
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
		if (mode === 'template') return !!templateId;
		if (mode === 'single-repo') {
			if (singleRepoSource === 'registry') return !!selectedRegistryRepo;
			if (singleRepoSource === 'new')
				return !!singleRepo.name.trim() && !!(singleRepo.remoteUrl ?? '').trim();
		}
		return false;
	});

	const filteredRegistry = $derived(
		repoRegistry.filter(
			(r) =>
				r.name.toLowerCase().includes(registrySearch.toLowerCase()) ||
				r.tags.some((t) => t.includes(registrySearch.toLowerCase())),
		),
	);

	const hasPendingHooksToResolve = $derived(
		pendingHooks.some((pending) => pending.trusted !== true),
	);
	const canOpenWorkset = $derived(
		initializedWorkspaceName !== null && !isInitializing && !hasPendingHooksToResolve,
	);

	const topoRepos = $derived.by<RepoTemplate[]>(() => {
		if (mode === 'single-repo' && singleRepo.name) return [singleRepo];
		return selectedTemplate?.repos ?? [];
	});

	const repoPositions = $derived.by(() => computeRepoPositions(topoRepos));
	const templateIconByName = {
		code2: Code2,
		workflow: Workflow,
		server: Server,
	} as const;

	$effect(() => {
		if (!nameTouched) {
			formName = defaultWorkspaceName;
		}
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

	const previewHooksForReview = async (): Promise<void> => {
		if (!onPreviewHooks) return;
		const repos = mode === 'single-repo' ? [singleRepo] : (selectedTemplate?.repos ?? []);
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

	const handleSelectMode = (m: 'template' | 'single-repo') => {
		mode = m;
		runError = null;
		clearHookPreviewState();
		if (m === 'single-repo') {
			templateId = '';
			singleRepoSource = null;
			selectedRegistryRepo = null;
		} else {
			templateId = templates[0]?.id ?? '';
			singleRepo = { name: '', remoteUrl: '', hooks: [], sourceType: 'direct' };
			singleRepoSource = null;
			selectedRegistryRepo = null;
		}
	};

	const handleSelectFromRegistry = (repo: RegisteredRepo) => {
		clearHookPreviewState();
		selectedRegistryRepo = repo;
		singleRepo = {
			name: repo.name,
			remoteUrl: repo.remoteUrl,
			hooks: [],
			aliasName: repo.aliasName,
			sourceType: 'alias',
		};
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

		runError = null;
		isInitializing = true;
		initializeStarted = true;
		hookWarnings = [];
		pendingHooks = [];
		hookRuns = [];
		initializedWorkspaceName = null;

		const repos = mode === 'single-repo' ? [singleRepo] : (selectedTemplate?.repos ?? []);
		const selectedGroups =
			mode === 'template' && selectedTemplate ? [selectedTemplate.groupName] : [];
		const selectedAliases =
			mode === 'single-repo' && singleRepoSource === 'registry' && selectedRegistryRepo
				? [selectedRegistryRepo.aliasName]
				: [];
		const primarySource =
			mode === 'single-repo' && singleRepoSource === 'new'
				? (singleRepo.remoteUrl ?? '').trim()
				: '';
		const directRepos =
			primarySource.length > 0
				? [
						{
							url: primarySource,
							register: true,
						},
					]
				: [];

		try {
			({ activeHookOperation, activeHookWorkspace, hookRuns, pendingHooks } = beginHookTracking(
				'workspace.create',
				trimmedWorkspaceName,
			));

			const result = await onStart?.({
				workspaceName: trimmedWorkspaceName,
				description: formDescription.trim(),
				repos: repos.map((r) => ({ ...r, hooks: [...(r.hooks ?? [])] })),
				selectedGroups,
				selectedAliases,
				primarySource,
				directRepos,
			});

			initializedWorkspaceName = result?.workspaceName ?? trimmedWorkspaceName;
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
		hookEventUnsubscribe?.();
		hookEventUnsubscribe = null;
	});
</script>

<div class="onboarding-shell">
	<div class="onboarding-inner">
		<div class="form-side">
			<div class="form-header">
				<h1>Create Workset</h1>
				<p>Set up a new development environment in seconds.</p>
				{#if onCancel}
					<button type="button" class="header-cancel" onclick={onCancel} disabled={busy}>
						Cancel
					</button>
				{/if}
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
						<span class="step-label" class:active={stepStatus(2) === 'active'}>Configure Repos</span
						>
					</div>

					{#if step === 2}
						<div class="step-content" in:fly={{ x: -20, duration: 200 }}>
							<div class="mode-row">
								<button
									type="button"
									class="mode-card"
									class:active={mode === 'single-repo'}
									onclick={() => handleSelectMode('single-repo')}
								>
									<div class="mode-icon green"><GitBranch size={18} /></div>
									<div>
										<div class="mode-title">Single Repo</div>
										<div class="mode-desc">One repository</div>
									</div>
								</button>
								<button
									type="button"
									class="mode-card"
									class:active={mode === 'template'}
									onclick={() => handleSelectMode('template')}
								>
									<div class="mode-icon blue"><Workflow size={18} /></div>
									<div>
										<div class="mode-title">From Template</div>
										<div class="mode-desc">Multi-repo bundle</div>
									</div>
								</button>
							</div>

							{#if mode === 'template'}
								<div class="template-list" in:fly={{ y: 10, duration: 180 }}>
									{#if catalogLoading}
										<div class="registry-empty">Loading templates from settings…</div>
									{:else if templates.length === 0}
										<div class="registry-empty">No templates found. Add templates in Settings.</div>
									{:else}
										{#each templates as t (t.id)}
											{@const visual = getTemplateVisual(t)}
											{@const Icon = templateIconByName[visual.icon]}
											<button
												type="button"
												class="template-row"
												class:active={templateId === t.id}
												onclick={() => (templateId = t.id)}
											>
												<div class="template-icon" style="color: {visual.color}">
													<Icon size={24} />
												</div>
												<div class="template-info">
													<div class="template-name">{t.name}</div>
													<div class="template-desc">{t.description}</div>
												</div>
												{#if templateId === t.id}
													<Check size={18} class="template-check" />
												{/if}
											</button>
										{/each}
									{/if}
								</div>
							{/if}

							{#if mode === 'single-repo'}
								<div class="single-repo-content" in:fly={{ y: 10, duration: 180 }}>
									<div class="source-toggle">
										<button
											type="button"
											class="source-btn"
											class:active-green={singleRepoSource === 'registry'}
											onclick={() => {
												clearHookPreviewState();
												singleRepoSource = 'registry';
												singleRepo = {
													name: '',
													remoteUrl: '',
													hooks: [],
													sourceType: 'direct',
												};
												selectedRegistryRepo = null;
											}}
										>
											<Search size={12} /> From Repo Catalog
										</button>
										<button
											type="button"
											class="source-btn"
											class:active-orange={singleRepoSource === 'new'}
											onclick={() => {
												clearHookPreviewState();
												singleRepoSource = 'new';
												singleRepo = {
													name: '',
													remoteUrl: '',
													hooks: [],
													sourceType: 'direct',
												};
												selectedRegistryRepo = null;
											}}
										>
											<Plus size={12} /> New Repo
										</button>
									</div>

									{#if singleRepoSource === 'registry'}
										<div in:fly={{ y: 6, duration: 150 }}>
											<div class="registry-search">
												<Search size={13} />
												<input
													type="text"
													bind:value={registrySearch}
													placeholder="Search registered repos..."
												/>
											</div>
											<div class="registry-list">
												{#each filteredRegistry as repo (repo.id)}
													{@const isSelected = selectedRegistryRepo?.id === repo.id}
													{@const langColor = languageColors[repo.language] ?? '#A3B5C9'}
													<button
														type="button"
														class="registry-item"
														class:selected={isSelected}
														onclick={() => handleSelectFromRegistry(repo)}
													>
														<div class="registry-check" class:checked={isSelected}>
															{#if isSelected}<Check size={10} />{/if}
														</div>
														<div class="registry-info">
															<div class="registry-name-row">
																<span class="registry-name">{repo.name}</span>
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
												{#if filteredRegistry.length === 0}
													<div class="registry-empty">
														{#if catalogLoading}
															Loading repos from catalog…
														{:else}
															No matching repos. Try
															<button
																type="button"
																class="link-btn"
																onclick={() => (singleRepoSource = 'new')}>adding a new one</button
															>.
														{/if}
													</div>
												{/if}
											</div>

											{#if selectedRegistryRepo}
												<div class="selected-summary" in:fly={{ y: 4, duration: 150 }}>
													<GitBranch size={14} />
													<div class="selected-summary-info">
														<div class="selected-summary-name">{selectedRegistryRepo.name}</div>
														<div class="selected-summary-url">{selectedRegistryRepo.remoteUrl}</div>
													</div>
													<span class="selected-summary-branch"
														>{selectedRegistryRepo.defaultBranch}</span
													>
												</div>
											{/if}
										</div>
									{/if}

									{#if singleRepoSource === 'new'}
										<div in:fly={{ y: 6, duration: 150 }}>
											<div class="new-repo-notice">
												<Plus size={11} />
												This repo will be added to your catalog
											</div>
											<label class="field">
												<span class="field-label-sm">Repository Name</span>
												<input
													type="text"
													bind:value={singleRepo.name}
													oninput={() => {
														clearHookPreviewState();
														singleRepo = { ...singleRepo };
													}}
													placeholder="e.g., my-api-service"
													class="mono"
												/>
											</label>
											<label class="field" style="margin-top: 12px">
												<span class="field-label-sm">Remote URL</span>
												<input
													type="text"
													value={singleRepo.remoteUrl ?? ''}
													oninput={(e) => {
														clearHookPreviewState();
														singleRepo = {
															...singleRepo,
															remoteUrl: (e.currentTarget as HTMLInputElement).value,
														};
													}}
													placeholder="git@github.com:org/repo.git"
													class="mono"
												/>
											</label>
										</div>
									{/if}

									{#if singleRepoSource && singleRepo.name}
										<div class="hooks-discovery-note" in:fly={{ y: 6, duration: 150 }}>
											Lifecycle hooks are previewed from repository config before initialization.
											You can still trust or run them during initialization.
										</div>
									{/if}
								</div>
							{/if}

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
									Next Step
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
							>Review &amp; Initialize</span
						>
					</div>

					{#if step === 3}
						<div class="step-content" in:fly={{ x: -20, duration: 200 }}>
							<div class="review-card">
								<div class="review-meta">Creating workset:</div>
								<div class="review-name">{formName}</div>
								{#if formDescription}
									<div class="review-desc-row">
										<AlignLeft size={11} />
										<p>{formDescription}</p>
									</div>
								{/if}
								<div class="review-mode-badge">
									{#if mode === 'single-repo'}
										<GitBranch size={11} /> Single Repository
									{:else}
										<Workflow size={11} /> {selectedTemplate?.name} Template
									{/if}
								</div>

								<div class="review-repos-label">
									{mode === 'single-repo' ? 'Repository:' : 'Repositories:'}
								</div>
								<ul class="review-repo-list">
									{#each reviewRepoEntries as entry (entry.key)}
										{@const repo = entry.repo}
										<li>
											<div class="review-repo-header">
												<GitBranch size={14} class="review-repo-icon" />
												<span>{repo.name}</span>
												{#if mode === 'single-repo' && repo.remoteUrl}
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
														class="hook-run-status"
														class:ok={run.status === 'ok'}
														class:failed={run.status === 'failed'}
														class:running-status={run.status === 'running'}
														class:skipped={run.status === 'skipped'}
													>
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
													<div class="pending-hook-actions">
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
														<div class="pending-hook-error">{pending.runError}</div>
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
								disabled={busy || isInitializing || (initializeStarted && !canOpenWorkset)}
							>
								{#if canOpenWorkset}
									Open Workset <ArrowRight size={16} />
								{:else if isInitializing || busy}
									Initializing Environment...
								{:else if initializeStarted}
									Resolve Hook Trust To Continue
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

			<h3 class="topo-title">Workset Topology</h3>

			<div class="topo-area">
				<svg class="topo-svg" viewBox="-200 -200 400 400">
					{#each repoPositions as pos, i (pos.name + '-line')}
						<line
							x1="0"
							y1="0"
							x2={pos.x}
							y2={pos.y}
							class="topo-svg-line"
							class:green={mode === 'single-repo'}
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
						class:green={mode === 'single-repo'}
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
					<span>{mode === 'single-repo' ? 'Single Repository' : 'Managed Repo Store'}</span>
				</div>
			</div>
		</div>
	</div>
</div>

<style src="./OnboardingView.css"></style>
