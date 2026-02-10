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
	import {
		languageColors,
		type RepoTemplate,
		type WorksetTemplate,
		type RegisteredRepo,
	} from '../../view-models/onboardingViewModel';
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

	export type OnboardingDraft = {
		workspaceName: string;
		description: string;
		repos: RepoTemplate[];
		selectedGroups: string[];
		selectedAliases: string[];
		primarySource: string;
		directRepos: Array<{ url: string; register: boolean }>;
	};

	export type OnboardingStartResult = {
		workspaceName: string;
		warnings: string[];
		pendingHooks: WorkspaceActionPendingHook[];
		hookRuns: HookExecution[];
	};

	interface Props {
		busy?: boolean;
		catalogLoading?: boolean;
		errorMessage?: string | null;
		defaultWorkspaceName?: string;
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
		templates = [],
		repoRegistry = [],
		onStart,
		onPreviewHooks,
		onComplete,
		onCancel,
	}: Props = $props();

	/* ── Step state ────────────────────────────────── */
	let step = $state(1);

	/* ── Form state ────────────────────────────────── */
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

	/* ── Initialization state ──────────────────────── */
	let isInitializing = $state(false);
	let initializeStarted = $state(false);
	let initializedWorkspaceName = $state<string | null>(null);
	let hookWarnings = $state<string[]>([]);
	let pendingHooks = $state<WorkspaceActionPendingHook[]>([]);
	let hookRuns = $state<HookExecution[]>([]);
	let activeHookOperation = $state<string | null>(null);
	let activeHookWorkspace = $state<string | null>(null);
	let hookEventUnsubscribe: (() => void) | null = null;

	/* ── Derived ───────────────────────────────────── */
	const selectedTemplate = $derived(templates.find((t) => t.id === templateId) ?? null);
	const hookPreviewEnabled = $derived(!!onPreviewHooks);

	const reviewRepos = $derived.by<RepoTemplate[]>(() => {
		if (mode === 'single-repo') return [singleRepo];
		return selectedTemplate?.repos ?? [];
	});

	type ReviewRepoEntry = {
		key: string;
		repo: RepoTemplate;
		source: string;
		hooks: string[];
	};
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

	/* ── Topology geometry ─────────────────────────── */
	const topoRepos = $derived.by<RepoTemplate[]>(() => {
		if (mode === 'single-repo' && singleRepo.name) return [singleRepo];
		return selectedTemplate?.repos ?? [];
	});

	const repoPositions = $derived.by(() => {
		if (topoRepos.length === 0) return [];
		const total = topoRepos.length;
		const radius = total === 1 ? 120 : 140;
		return topoRepos.map((repo, i) => {
			const angle = total === 1 ? 0 : (i / total) * 2 * Math.PI - Math.PI / 2;
			return {
				x: Math.cos(angle) * radius,
				y: Math.sin(angle) * radius,
				name: repo.name,
			};
		});
	});

	/* ── Effects ───────────────────────────────────── */
	$effect(() => {
		if (!nameTouched) {
			formName = defaultWorkspaceName;
		}
	});

	$effect(() => {
		if (templateId || templates.length === 0) return;
		templateId = templates[0].id;
	});

	/* ── Actions ───────────────────────────────────── */
	const nextStep = () => {
		step += 1;
	};
	const prevStep = () => {
		step -= 1;
	};

	const resolveHookPreviewSource = (repo: RepoTemplate): string => {
		const aliasName = (repo.aliasName ?? '').trim();
		if (repo.sourceType === 'alias' && aliasName) return aliasName;
		const remote = (repo.remoteUrl ?? '').trim();
		if (remote) return remote;
		if (aliasName) return aliasName;
		return repo.name.trim();
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
			workspaceReferences: [initializedWorkspaceName, activeHookWorkspace, formName.trim()],
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
			workspaceReferences: [initializedWorkspaceName, activeHookWorkspace, formName.trim()],
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
				formName.trim(),
			));

			const result = await onStart?.({
				workspaceName: formName.trim(),
				description: formDescription.trim(),
				repos: repos.map((r) => ({ ...r, hooks: [...(r.hooks ?? [])] })),
				selectedGroups,
				selectedAliases,
				primarySource,
				directRepos,
			});

			initializedWorkspaceName = result?.workspaceName ?? formName.trim();
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

	const stepStatus = (s: number): 'active' | 'completed' | 'pending' => {
		if (s === step) return 'active';
		if (s < step) return 'completed';
		return 'pending';
	};

	const templateIcon = (template: WorksetTemplate): typeof Code2 => {
		if (template.repos.length >= 6) return Server;
		if (template.repos.length >= 3) return Workflow;
		return Code2;
	};

	const templateIconColor = (template: WorksetTemplate): string => {
		if (template.repos.length >= 6) return '#2D8CFF';
		if (template.repos.length >= 3) return '#86C442';
		return '#F28C28';
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
		hookEventUnsubscribe?.();
		hookEventUnsubscribe = null;
	});
</script>

<div class="onboarding-shell">
	<div class="onboarding-inner">
		<!-- ═══ LEFT: Steps & Form ═══ -->
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
				<!-- Progress line -->
				<div class="progress-line"></div>

				<!-- ── Step 1 ── -->
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
									disabled={!formName || busy}
								>
									Continue <ArrowRight size={16} />
								</button>
							</div>
						</div>
					{/if}
				</div>

				<!-- ── Step 2 ── -->
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
							<!-- Mode selector -->
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

							<!-- Template mode -->
							{#if mode === 'template'}
								<div class="template-list" in:fly={{ y: 10, duration: 180 }}>
									{#if catalogLoading}
										<div class="registry-empty">Loading templates from settings…</div>
									{:else if templates.length === 0}
										<div class="registry-empty">No templates found. Add templates in Settings.</div>
									{:else}
										{#each templates as t (t.id)}
											{@const Icon = templateIcon(t)}
											<button
												type="button"
												class="template-row"
												class:active={templateId === t.id}
												onclick={() => (templateId = t.id)}
											>
												<div class="template-icon" style="color: {templateIconColor(t)}">
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

							<!-- Single repo mode -->
							{#if mode === 'single-repo'}
								<div class="single-repo-content" in:fly={{ y: 10, duration: 180 }}>
									<!-- Source toggle -->
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

									<!-- Registry picker -->
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

									<!-- New repo form -->
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

							<!-- Navigation -->
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

				<!-- ── Step 3 ── -->
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
							<!-- Review card -->
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

							<!-- Initialize button -->
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

		<!-- ═══ RIGHT: Topology Visualization ═══ -->
		<div class="topo-side">
			<div class="topo-gradient"></div>

			<h3 class="topo-title">Workset Topology</h3>

			<div class="topo-area">
				<!-- SVG layer for animated connection lines -->
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

				<!-- Central hub -->
				<div class="hub-node" class:dim={!formName}>
					<LayoutTemplate size={24} />
					<span class="hub-label">{formName || '...'}</span>
				</div>

				<!-- Description callout -->
				{#if formDescription}
					<div class="hub-desc-callout" in:fade={{ duration: 150 }}>
						<p>{formDescription}</p>
					</div>
				{/if}

				<!-- Satellite repo nodes -->
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

<style>
	/* ═══ Shell ═══ */
	.onboarding-shell {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		padding: 32px;
		background: color-mix(in srgb, var(--bg) 90%, transparent);
	}

	.onboarding-inner {
		display: flex;
		gap: 48px;
		width: 100%;
		max-width: 960px;
		max-height: 100%;
	}

	/* ═══ Left: Form Side ═══ */
	.form-side {
		flex: 1;
		max-width: 420px;
		overflow: auto;
		padding-right: 8px;
	}

	.form-header {
		margin-bottom: 48px;
	}

	.header-cancel {
		margin-top: 12px;
		padding: 4px 10px;
		border-radius: 999px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--muted);
		font-size: var(--text-sm);
		font-weight: 600;
		cursor: pointer;
	}

	.header-cancel:hover:not(:disabled) {
		border-color: var(--accent);
		color: var(--text);
	}

	.form-header h1 {
		margin: 0;
		font-size: var(--text-3xl);
		font-weight: 700;
		font-family: var(--font-display);
		letter-spacing: -0.02em;
		line-height: 1.15;
	}

	.form-header p {
		margin: 8px 0 0;
		color: var(--muted);
		font-size: var(--text-md);
	}

	/* ═══ Step Rail (vertical) ═══ */
	.step-rail {
		position: relative;
	}

	.progress-line {
		position: absolute;
		left: 15px;
		top: 0;
		bottom: 0;
		width: 2px;
		background: var(--border);
	}

	.step-group {
		position: relative;
		margin-bottom: 24px;
	}

	.step-indicator-row {
		display: flex;
		align-items: center;
		gap: 16px;
		position: relative;
		z-index: 1;
	}

	.step-dot {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		border: 2px solid var(--border);
		background: var(--panel-soft);
		color: var(--muted);
		display: grid;
		place-items: center;
		flex-shrink: 0;
		transition:
			background 0.3s,
			border-color 0.3s,
			color 0.3s,
			box-shadow 0.3s;
	}

	.step-dot.active {
		background: var(--accent);
		border-color: var(--accent);
		color: white;
		box-shadow: 0 0 10px color-mix(in srgb, var(--accent) 50%, transparent);
	}

	.step-dot.completed {
		background: var(--success);
		border-color: var(--success);
		color: white;
	}

	.step-num {
		font-size: var(--text-base);
		font-weight: 700;
		font-family: var(--font-display);
	}

	.step-label {
		font-size: var(--text-md);
		font-weight: 500;
		color: var(--muted);
		transition: color 0.2s;
	}

	.step-label.active {
		color: var(--text);
	}

	/* ═══ Step Content ═══ */
	.step-content {
		display: flex;
		flex-direction: column;
		gap: 16px;
		margin-left: 48px;
		margin-top: 12px;
		margin-bottom: 8px;
	}

	/* ═══ Fields ═══ */
	.field-group {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.field-label {
		font-size: var(--text-md);
		font-weight: 500;
		color: var(--text);
	}

	.field-label-sm {
		font-size: var(--text-sm);
		font-weight: 500;
		color: var(--text);
	}

	.field-hint-inline {
		margin-left: 8px;
		font-size: var(--text-xs);
		color: #8b8aed;
		font-weight: 400;
	}

	.field-hint {
		margin: 4px 0 0;
		font-size: var(--text-xs);
		color: #6b7f95;
	}

	.field input,
	.field textarea,
	.step-content input:not(.registry-search input) {
		width: 100%;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 10px 16px;
		color: var(--text);
		font-family: inherit;
		font-size: var(--text-md);
		transition: border-color 0.15s;
	}

	.mono {
		font-family: var(--font-mono);
		font-size: var(--text-mono-base);
	}

	.field input:focus,
	.field textarea:focus,
	.step-content input:focus {
		outline: none;
		border-color: var(--accent);
	}

	.field textarea {
		resize: none;
		font-size: var(--text-base);
	}

	/* ═══ Primary Button ═══ */
	.btn-primary {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 10px 24px;
		background: var(--accent);
		border: none;
		border-radius: 8px;
		color: white;
		font-weight: 600;
		font-size: var(--text-md);
		font-family: inherit;
		cursor: pointer;
		transition:
			background 0.15s,
			opacity 0.15s;
	}

	.btn-primary:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 88%, white);
	}

	.btn-primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-primary.wide {
		flex: 1;
	}

	/* ═══ Mode Cards ═══ */
	.mode-row {
		display: flex;
		gap: 12px;
	}

	.mode-card {
		flex: 1;
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--panel-strong);
		cursor: pointer;
		text-align: left;
		font-family: inherit;
		transition:
			border-color 0.15s,
			background 0.15s;
	}

	.mode-card:hover {
		border-color: color-mix(in srgb, var(--accent) 50%, var(--border));
	}

	.mode-card.active {
		background: color-mix(in srgb, var(--accent) 10%, var(--panel-strong));
		border-color: var(--accent);
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 50%, transparent);
	}

	.mode-icon {
		width: 32px;
		height: 32px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--bg) 40%, transparent);
		border: 1px solid var(--border);
		display: grid;
		place-items: center;
		flex-shrink: 0;
	}

	.mode-icon.green {
		color: var(--success);
	}

	.mode-icon.blue {
		color: var(--accent);
	}

	.mode-title {
		font-size: var(--text-md);
		font-weight: 500;
		color: var(--text);
	}

	.mode-desc {
		font-size: var(--text-xs);
		color: var(--muted);
	}

	/* ═══ Template List ═══ */
	.template-list {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.template-row {
		display: flex;
		align-items: center;
		gap: 16px;
		padding: 16px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--panel-strong);
		cursor: pointer;
		text-align: left;
		font-family: inherit;
		transition:
			border-color 0.15s,
			background 0.15s,
			transform 0.15s;
	}

	.template-row:hover {
		transform: scale(1.02);
		border-color: color-mix(in srgb, var(--accent) 50%, var(--border));
	}

	.template-row.active {
		background: color-mix(in srgb, var(--accent) 10%, var(--panel-strong));
		border-color: var(--accent);
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 50%, transparent);
	}

	.template-icon {
		width: 40px;
		height: 40px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--bg) 40%, transparent);
		border: 1px solid var(--border);
		display: grid;
		place-items: center;
		flex-shrink: 0;
	}

	.template-info {
		flex: 1;
		min-width: 0;
	}

	.template-name {
		font-size: var(--text-md);
		font-weight: 500;
		color: var(--text);
	}

	.template-desc {
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.template-row :global(.template-check) {
		color: var(--accent);
		margin-left: auto;
		flex-shrink: 0;
	}

	/* ═══ Single Repo ═══ */
	.single-repo-content {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.source-toggle {
		display: flex;
		gap: 8px;
	}

	.source-btn {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		padding: 8px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--panel-strong);
		color: var(--muted);
		font-size: var(--text-sm);
		font-weight: 500;
		font-family: inherit;
		cursor: pointer;
		transition:
			border-color 0.15s,
			background 0.15s,
			color 0.15s;
	}

	.source-btn:hover {
		color: var(--text);
	}

	.source-btn.active-green {
		background: color-mix(in srgb, var(--success) 10%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--success) 50%, var(--border));
		color: var(--success);
	}

	.source-btn.active-orange {
		background: color-mix(in srgb, var(--warning) 10%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--warning) 50%, var(--border));
		color: var(--warning);
	}

	/* ── Registry ── */
	.registry-search {
		position: relative;
		display: flex;
		align-items: center;
		margin-bottom: 12px;
	}

	.registry-search :global(svg) {
		position: absolute;
		left: 12px;
		color: var(--muted);
		pointer-events: none;
	}

	.registry-search input {
		width: 100%;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 8px 16px 8px 36px;
		color: var(--text);
		font-size: var(--text-base);
		font-family: inherit;
	}

	.registry-list {
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: 12px;
		overflow: hidden;
		max-height: 192px;
		overflow-y: auto;
	}

	.registry-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		width: 100%;
		border: none;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
		background: transparent;
		cursor: pointer;
		text-align: left;
		font-family: inherit;
		transition: background 0.1s;
	}

	.registry-item:last-child {
		border-bottom: none;
	}

	.registry-item:hover {
		background: var(--panel-strong);
	}

	.registry-item.selected {
		background: color-mix(in srgb, var(--success) 10%, transparent);
	}

	.registry-check {
		width: 16px;
		height: 16px;
		border: 1px solid var(--border);
		border-radius: 3px;
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: white;
		transition:
			background 0.15s,
			border-color 0.15s;
	}

	.registry-check.checked {
		background: var(--success);
		border-color: var(--success);
	}

	.registry-info {
		flex: 1;
		min-width: 0;
	}

	.registry-name-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.registry-name {
		font-size: var(--text-mono-base);
		font-family: var(--font-mono);
		color: var(--text);
	}

	.lang-badge {
		font-size: var(--text-xs);
		padding: 2px 6px;
		border-radius: 4px;
		font-weight: 500;
		border: 1px solid;
		flex-shrink: 0;
	}

	.registry-url {
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: color-mix(in srgb, var(--muted) 60%, transparent);
		margin-top: 2px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.registry-empty {
		padding: 16px;
		text-align: center;
		font-size: var(--text-sm);
		color: color-mix(in srgb, var(--muted) 50%, transparent);
	}

	.link-btn {
		background: none;
		border: none;
		color: var(--warning);
		text-decoration: underline;
		cursor: pointer;
		font-size: inherit;
		font-family: inherit;
		padding: 0;
	}

	.selected-summary {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--success) 5%, transparent);
		border: 1px solid color-mix(in srgb, var(--success) 20%, var(--border));
		margin-top: 12px;
		color: var(--success);
	}

	.selected-summary-info {
		flex: 1;
		min-width: 0;
	}

	.selected-summary-name {
		font-size: var(--text-mono-base);
		font-family: var(--font-mono);
		color: var(--text);
	}

	.selected-summary-url {
		font-size: var(--text-xs);
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.selected-summary-branch {
		font-size: var(--text-xs);
		color: var(--muted);
		background: var(--panel-strong);
		padding: 2px 6px;
		border-radius: 4px;
		border: 1px solid var(--border);
		flex-shrink: 0;
	}

	/* ── New repo notice ── */
	.new-repo-notice {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 12px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--warning) 5%, transparent);
		border: 1px solid color-mix(in srgb, var(--warning) 20%, var(--border));
		color: var(--warning);
		font-size: var(--text-xs);
		margin-bottom: 12px;
	}

	.hooks-discovery-note {
		padding: 10px 12px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--accent) 10%, var(--panel-strong));
		border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--border));
		font-size: var(--text-sm);
		color: var(--muted);
		line-height: 1.4;
	}

	/* ═══ Step 2 Nav ═══ */
	.step2-nav {
		display: flex;
		gap: 12px;
		padding-top: 8px;
	}

	.back-btn {
		padding: 8px;
		background: none;
		border: none;
		color: var(--muted);
		cursor: pointer;
		transition: color 0.15s;
	}

	.back-btn:hover {
		color: var(--text);
	}

	/* ═══ Review ═══ */
	.review-card {
		background: var(--panel-strong);
		padding: 20px;
		border-radius: 12px;
		border: 1px solid var(--border);
		margin-bottom: 24px;
	}

	.review-meta {
		font-size: var(--text-base);
		color: var(--muted);
		margin-bottom: 4px;
	}

	.review-name {
		font-size: var(--text-2xl);
		font-family: var(--font-mono);
		font-weight: 600;
		color: var(--text);
		margin-bottom: 4px;
	}

	.review-desc-row {
		display: flex;
		align-items: flex-start;
		gap: 6px;
		margin-bottom: 8px;
		color: #8b8aed;
	}

	.review-desc-row p {
		margin: 0;
		font-size: var(--text-sm);
		color: var(--muted);
		line-height: 1.5;
	}

	.review-mode-badge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-sm);
		color: var(--muted);
		margin-bottom: 16px;
	}

	.review-repos-label {
		font-size: var(--text-base);
		color: var(--muted);
		margin-bottom: 8px;
	}

	.review-repo-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.review-repo-header {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-md);
		color: var(--text);
	}

	.review-repo-header :global(.review-repo-icon) {
		color: var(--accent);
		flex-shrink: 0;
	}

	.review-repo-url {
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: var(--muted);
		margin-left: 4px;
		max-width: 200px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.review-hooks-status {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 10px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.review-hooks-warning {
		display: flex;
		align-items: flex-start;
		gap: 6px;
		margin-top: 10px;
		padding: 8px 10px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--warning) 35%, var(--border));
		background: color-mix(in srgb, var(--warning) 12%, transparent);
		color: color-mix(in srgb, var(--warning) 82%, white);
		font-size: var(--text-sm);
		line-height: 1.35;
	}

	.review-hooks-warning :global(svg) {
		flex-shrink: 0;
		margin-top: 1px;
	}

	.review-hooks-label {
		margin-top: 12px;
		margin-bottom: 8px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.review-hooks-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.review-hooks-item {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 8px;
		border-radius: 8px;
		background: var(--panel-soft);
		border: 1px solid var(--border);
	}

	.review-hooks-repo {
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.review-hooks-chip-row {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.review-hooks-chip {
		display: inline-flex;
		align-items: center;
		padding: 2px 8px;
		border-radius: 999px;
		border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--border));
		background: color-mix(in srgb, var(--accent) 10%, var(--panel-strong));
		color: var(--text);
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
	}

	.hook-runtime-card {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin-bottom: 16px;
		padding: 12px;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--panel-soft);
	}

	.hook-runtime-status {
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--muted);
		font-size: var(--text-sm);
	}

	.hook-warning-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.hook-warning-item {
		display: flex;
		gap: 6px;
		align-items: flex-start;
		color: color-mix(in srgb, var(--warning) 80%, white);
		font-size: var(--text-sm);
		line-height: 1.4;
	}

	.hook-warning-item :global(svg) {
		margin-top: 2px;
		flex-shrink: 0;
	}

	.hook-runs-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.hook-run-row {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-sm);
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 6px 8px;
	}

	.hook-run-repo {
		color: var(--text);
		font-weight: 500;
	}

	.hook-run-id {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: var(--muted);
	}

	.hook-run-status {
		margin-left: auto;
		font-size: var(--text-xs);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		padding: 2px 6px;
		border-radius: 999px;
		border: 1px solid var(--border);
		color: var(--muted);
	}

	.hook-run-status.ok {
		background: color-mix(in srgb, var(--success) 18%, transparent);
		color: var(--success);
		border-color: color-mix(in srgb, var(--success) 45%, transparent);
	}

	.hook-run-status.failed {
		background: color-mix(in srgb, var(--danger) 18%, transparent);
		color: var(--danger);
		border-color: color-mix(in srgb, var(--danger) 45%, transparent);
	}

	.hook-run-status.running-status {
		background: color-mix(in srgb, var(--accent) 16%, transparent);
		color: var(--accent);
		border-color: color-mix(in srgb, var(--accent) 35%, transparent);
	}

	.hook-run-status.skipped {
		background: color-mix(in srgb, var(--muted) 18%, transparent);
	}

	.pending-hooks-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.pending-hook-row {
		display: grid;
		gap: 8px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 10px;
	}

	.pending-hook-copy {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.pending-hook-title {
		display: flex;
		align-items: center;
		gap: 6px;
		color: var(--text);
		font-size: var(--text-sm);
		font-weight: 500;
	}

	.pending-hook-title :global(svg) {
		color: var(--warning);
	}

	.pending-hook-body {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: var(--muted);
	}

	.pending-hook-trusted {
		margin-left: auto;
		color: var(--success);
		font-size: var(--text-xs);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.pending-hook-actions {
		display: flex;
		gap: 8px;
	}

	.pending-hook-btn {
		padding: 6px 10px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--accent) 45%, var(--border));
		background: color-mix(in srgb, var(--accent) 18%, var(--panel-strong));
		color: var(--text);
		font-size: var(--text-sm);
		font-family: inherit;
		cursor: pointer;
	}

	.pending-hook-btn.ghost {
		border-color: var(--border);
		background: transparent;
		color: var(--muted);
	}

	.pending-hook-btn:disabled {
		opacity: 0.6;
		cursor: wait;
	}

	.pending-hook-error {
		color: color-mix(in srgb, var(--danger) 85%, white);
		font-size: var(--text-xs);
	}

	.hook-spin {
		color: var(--accent);
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	@keyframes hubPulse {
		0%,
		100% {
			box-shadow: 0 0 30px color-mix(in srgb, var(--accent) 25%, transparent);
		}
		50% {
			box-shadow:
				0 0 40px color-mix(in srgb, var(--accent) 40%, transparent),
				0 0 80px color-mix(in srgb, var(--accent) 10%, transparent);
		}
	}

	.review-no-hooks {
		padding-left: 0;
		font-size: var(--text-sm);
		color: color-mix(in srgb, var(--muted) 50%, transparent);
		font-style: italic;
		margin-top: 8px;
	}

	/* ── Init button ── */
	.init-btn {
		width: 100%;
		padding: 12px;
		border: none;
		border-radius: 8px;
		font-size: var(--text-md);
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		background: var(--success);
		color: white;
		box-shadow: 0 0 20px color-mix(in srgb, var(--success) 20%, transparent);
		transition:
			background 0.15s,
			box-shadow 0.15s;
	}

	.init-btn:hover:not(:disabled) {
		background: color-mix(in srgb, var(--success) 90%, white);
		box-shadow: 0 0 30px color-mix(in srgb, var(--success) 40%, transparent);
	}

	.init-btn:disabled {
		cursor: wait;
	}

	.init-btn.running {
		background: var(--panel-strong);
		color: var(--muted);
		border: 1px solid var(--border);
		box-shadow: none;
	}

	.init-btn.finished {
		background: var(--accent);
		box-shadow: 0 0 20px color-mix(in srgb, var(--accent) 20%, transparent);
	}

	.init-btn.finished:hover {
		background: color-mix(in srgb, var(--accent) 90%, white);
	}

	.back-link {
		width: 100%;
		margin-top: 12px;
		background: none;
		border: none;
		color: var(--muted);
		font-size: var(--text-base);
		font-family: inherit;
		cursor: pointer;
		transition: color 0.15s;
	}

	.back-link:hover {
		color: var(--text);
	}

	.init-error {
		margin-top: 10px;
		padding: 8px 10px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--danger) 45%, transparent);
		background: color-mix(in srgb, var(--danger) 14%, transparent);
		color: color-mix(in srgb, var(--danger) 92%, white);
		font-size: var(--text-sm);
		line-height: 1.4;
	}

	/* ═══ Right: Topology ═══ */
	.topo-side {
		flex: 1;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 32px;
		display: flex;
		flex-direction: column;
		position: relative;
		overflow: hidden;
		min-height: 460px;
	}

	.topo-gradient {
		position: absolute;
		inset: 0;
		background: radial-gradient(
			circle at center,
			color-mix(in srgb, var(--accent) 5%, transparent) 0%,
			transparent 70%
		);
		pointer-events: none;
		animation: gradientPulse 4s ease-in-out infinite;
	}

	@keyframes gradientPulse {
		0%,
		100% {
			opacity: 0.6;
		}
		50% {
			opacity: 1;
		}
	}

	.topo-title {
		margin: 0;
		font-size: var(--text-sm);
		font-weight: 500;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		margin-bottom: 32px;
		position: relative;
		z-index: 1;
	}

	.topo-area {
		flex: 1;
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1;
	}

	/* ── Hub node ── */
	.hub-node {
		width: 96px;
		height: 96px;
		border-radius: 50%;
		background: var(--panel-soft);
		border: 2px solid var(--accent);
		box-shadow: 0 0 30px color-mix(in srgb, var(--accent) 30%, transparent);
		animation: hubPulse 3s ease-in-out infinite;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		color: var(--accent);
		z-index: 20;
		transition:
			transform 0.3s,
			opacity 0.3s;
	}

	.hub-node.dim {
		transform: scale(0.8);
		opacity: 0.5;
	}

	.hub-label {
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: var(--muted);
		text-align: center;
		max-width: 80px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ── Hub description callout ── */
	.hub-desc-callout {
		position: absolute;
		top: calc(50% + 56px);
		left: 50%;
		transform: translateX(-50%);
		max-width: 200px;
		padding: 6px 12px;
		border-radius: 8px;
		background: color-mix(in srgb, #8b8aed 10%, transparent);
		border: 1px solid color-mix(in srgb, #8b8aed 20%, transparent);
		text-align: center;
		z-index: 30;
	}

	.hub-desc-callout p {
		margin: 0;
		font-size: var(--text-xs);
		color: #8b8aed;
		line-height: 1.4;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ── SVG connection lines ── */
	.topo-svg {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		pointer-events: none;
		overflow: visible;
	}

	.topo-svg-line {
		stroke: var(--accent);
		stroke-width: 1.5;
		stroke-dasharray: 6 4;
		stroke-linecap: round;
		opacity: 0.4;
		animation: dashFlow 1.2s linear infinite;
	}

	.topo-svg-line.green {
		stroke: var(--success);
	}

	@keyframes dashFlow {
		to {
			stroke-dashoffset: -20;
		}
	}

	/* ── Repo nodes ── */
	.repo-node {
		position: absolute;
		left: 50%;
		top: 50%;
		width: 56px;
		height: 56px;
		margin-left: -28px;
		margin-top: -28px;
		border-radius: 12px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
		color: var(--muted);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
		transition:
			transform 0.25s ease,
			box-shadow 0.25s ease,
			border-color 0.25s ease;
	}

	.repo-node:hover {
		transform: translate(var(--tx, 0), var(--ty, 0)) scale(1.08);
		box-shadow:
			0 6px 20px rgba(0, 0, 0, 0.4),
			0 0 12px color-mix(in srgb, var(--accent) 15%, transparent);
		border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
	}

	.repo-node.green {
		border-color: color-mix(in srgb, var(--success) 50%, var(--border));
		color: var(--success);
	}

	.repo-node.green:hover {
		border-color: var(--success);
		box-shadow:
			0 6px 20px rgba(0, 0, 0, 0.4),
			0 0 12px color-mix(in srgb, var(--success) 20%, transparent);
	}

	.repo-node-label {
		font-size: var(--text-mono-xs);
		color: var(--muted);
		max-width: 50px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ── Topo footer ── */
	.topo-footer {
		margin-top: 32px;
		text-align: center;
		position: relative;
		z-index: 1;
	}

	.topo-badge {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 6px 12px;
		border-radius: 999px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		color: var(--muted);
		font-size: var(--text-mono-sm);
		font-family: var(--font-mono);
	}

	/* ═══ Responsive ═══ */
	@media (max-width: 900px) {
		.onboarding-inner {
			flex-direction: column;
			max-width: 100%;
		}

		.form-side {
			max-width: 100%;
		}

		.topo-side {
			min-height: 300px;
		}
	}
</style>
