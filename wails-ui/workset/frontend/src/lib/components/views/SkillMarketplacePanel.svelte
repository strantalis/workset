<script lang="ts">
	import {
		ChevronDown,
		ChevronUp,
		Download,
		ExternalLink,
		LoaderCircle,
		LockKeyhole,
		Search,
		Settings,
		ShieldCheck,
		Sparkles,
		Star,
		TrendingUp,
	} from '@lucide/svelte';
	import DOMPurify from 'dompurify';
	import { marked } from 'marked';
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import {
		attachSkillMarketplaceSource,
		getSkill,
		getMarketplaceSkillMetadata,
		getMarketplaceSkillContent,
		installMarketplaceSkill,
		searchMarketplaceSkills,
		type MarketplaceSkill,
		type SkillInfo,
		type SkillScope,
	} from '../../api/skills';

	type ToolOption = {
		id: string;
		label: string;
		globalOnly?: boolean;
	};

	const TOOL_OPTIONS: ToolOption[] = [
		{ id: 'agents', label: 'Agents' },
		{ id: 'claude', label: 'Claude' },
		{ id: 'codex', label: 'Codex' },
		{ id: 'copilot', label: 'Copilot', globalOnly: false },
		{ id: 'cursor', label: 'Cursor' },
		{ id: 'opencode', label: 'OpenCode' },
	];

	const QUICK_SEARCHES = ['frontend', 'testing', 'react', 'postgres', 'review'];
	const DEFAULT_QUERY = 'frontend';
	const SEARCH_DEBOUNCE_MS = 260;

	const {
		workspaceId = null,
		installedSkills = [],
		onInstalled = () => {},
	}: {
		workspaceId?: string | null;
		installedSkills?: SkillInfo[];
		onInstalled?: (payload: { installedSkill: SkillInfo; message: string }) => void | Promise<void>;
	} = $props();

	const stripFrontmatter = (raw: string): string => {
		const trimmed = raw.trimStart();
		if (!trimmed.startsWith('---')) return raw;
		const end = trimmed.indexOf('---', 3);
		if (end === -1) return raw;
		return trimmed.slice(end + 3).trimStart();
	};

	const toErrorMessage = (error: unknown, fallback: string): string =>
		error instanceof Error ? error.message : fallback;

	const slugifyDirName = (skill: MarketplaceSkill): string => {
		const input = (skill.externalId || skill.name || '')
			.toLowerCase()
			.replace(/[^a-z0-9/_ -]+/g, '')
			.trim();
		const lastSegment = input.split('/').filter(Boolean).at(-1) ?? skill.name;
		return lastSegment
			.toLowerCase()
			.replace(/[^a-z0-9_-]+/g, '-')
			.replace(/-{2,}/g, '-')
			.replace(/^-|-$/g, '');
	};

	const formatInstallCount = (count?: number | null): string => {
		if (count == null) return 'Unknown installs';
		return `${Intl.NumberFormat('en-US', { notation: 'compact' }).format(count)} installs`;
	};

	const formatScopeLabel = (scope: SkillScope): string =>
		scope === 'project' ? 'Workset' : 'Global';

	const skillKey = (skill: Pick<MarketplaceSkill, 'provider' | 'externalId'>): string =>
		`${skill.provider}:${skill.externalId}`;

	const getInstalledScopes = (skill: MarketplaceSkill): SkillScope[] => {
		const scopes: SkillScope[] = [];
		for (const installed of installedSkills) {
			const marketplace = installed.marketplace;
			if (!marketplace) continue;
			if (marketplace.provider !== skill.provider) continue;
			if (marketplace.externalId !== skill.externalId) continue;
			if (
				(installed.scope === 'global' || installed.scope === 'project') &&
				!scopes.includes(installed.scope)
			) {
				scopes.push(installed.scope);
			}
		}
		for (const scope of backfilledInstallScopes[skillKey(skill)] ?? []) {
			if (!scopes.includes(scope)) {
				scopes.push(scope);
			}
		}
		return scopes.sort((left, right) => (left === right ? 0 : left === 'project' ? -1 : 1));
	};

	const isInstalled = (skill: MarketplaceSkill): boolean => getInstalledScopes(skill).length > 0;

	const normalizeSkillContent = (input: string): string => input.replace(/\r\n/g, '\n').trim();

	const resolveInstalledTool = (skill: SkillInfo): string => {
		if (skill.tools.includes('agents')) return 'agents';
		if (skill.tools.includes('claude')) return 'claude';
		if (skill.tools.includes('codex')) return 'codex';
		return skill.tools[0] ?? 'agents';
	};

	const getSourceHost = (rawSkillUrl: string): string => {
		try {
			return new URL(rawSkillUrl).host;
		} catch {
			return 'unknown host';
		}
	};

	const getSecuritySignal = (
		skill: MarketplaceSkill,
	): {
		label: string;
		tone: 'good' | 'caution' | 'neutral';
		detail: string;
	} => {
		const audits = skill.auditSummaries ?? [];
		if (audits.length > 0) {
			const normalized = audits.map((audit) => audit.status.toLowerCase());
			const hasHighRisk = normalized.some(
				(status) =>
					status.includes('high') ||
					status.includes('critical') ||
					(status.includes('alert') && !status.includes('0 alert')),
			);
			const hasMediumRisk = normalized.some(
				(status) => status.includes('med') || status.includes('warn'),
			);
			const summary = audits.map((audit) => `${audit.provider}: ${audit.status}`).join(' • ');
			if (hasHighRisk) {
				return {
					label: 'Audit issues detected',
					tone: 'caution',
					detail: summary,
				};
			}
			if (hasMediumRisk) {
				return {
					label: 'Audit review recommended',
					tone: 'neutral',
					detail: summary,
				};
			}
			return {
				label: 'Audits available',
				tone: 'good',
				detail: summary,
			};
		}
		if (skill.verified === true && (skill.trustScore ?? 0) >= 8.5) {
			return {
				label: 'Verified source',
				tone: 'good',
				detail: 'Provider verification and strong trust metadata are available.',
			};
		}
		if (skill.verified === true) {
			return {
				label: 'Provider verified',
				tone: 'good',
				detail:
					'Verification exists, but audit depth is provider-defined rather than enforced by Workset.',
			};
		}
		if (skill.sourceRepo.trim().length > 0) {
			return {
				label: 'Public source',
				tone: 'neutral',
				detail:
					'GitHub source is visible, but no provider audit signal is present. Review before installing.',
			};
		}
		return {
			label: 'No audit signal',
			tone: 'caution',
			detail:
				'No verification or public-source confidence signal is present. Manual review is required.',
		};
	};

	const auditStatusTone = (status: string): 'good' | 'neutral' | 'caution' => {
		const normalized = status.toLowerCase();
		if (
			normalized.includes('high') ||
			normalized.includes('critical') ||
			(normalized.includes('alert') && !normalized.includes('0 alert'))
		) {
			return 'caution';
		}
		if (normalized.includes('med') || normalized.includes('warn')) {
			return 'neutral';
		}
		return 'good';
	};

	const auditLabel = (provider: string): string => {
		switch (provider) {
			case 'Gen Agent Trust Hub':
				return 'Gen';
			default:
				return provider;
		}
	};

	const hasAuditSummaries = (skill: MarketplaceSkill): boolean =>
		(skill.auditSummaries?.length ?? 0) > 0;

	let searchQuery = $state('');
	let loading = $state(false);
	let detailLoading = $state(false);
	let installing = $state(false);
	let searchError = $state<string | null>(null);
	let installError = $state<string | null>(null);

	let results = $state<MarketplaceSkill[]>([]);
	let selectedKey = $state<string | null>(null);
	let selectedSkill = $state<MarketplaceSkill | null>(null);
	let content = $state('');
	let debouncedSearch: ReturnType<typeof setTimeout> | null = null;
	let searchGeneration = 0;

	let installScope = $state<SkillScope>('global');
	let installDirName = $state('');
	const installTools = new SvelteSet<string>(['agents']);
	let installScopeInitialized = false;
	let installConfigOpen = $state(false);
	let backfilledInstallScopes = $state<Record<string, SkillScope[]>>({});

	const selectedResult = $derived.by<MarketplaceSkill | null>(() => {
		if (selectedSkill && skillKey(selectedSkill) === selectedKey) {
			return selectedSkill;
		}
		return results.find((entry) => skillKey(entry) === selectedKey) ?? selectedSkill;
	});

	const availableToolOptions = $derived.by(() =>
		TOOL_OPTIONS.filter((option) => !(installScope === 'global' && option.globalOnly === false)),
	);

	const renderedMarkdown = $derived.by(() => {
		if (!content) return '';
		try {
			const rendered = marked.parse(stripFrontmatter(content), {
				async: false,
				gfm: true,
				breaks: true,
			}) as string;
			return DOMPurify.sanitize(rendered);
		} catch {
			return '<p>Failed to render markdown.</p>';
		}
	});

	const canInstall = $derived(
		selectedSkill !== null &&
			content.trim().length > 0 &&
			installDirName.trim().length > 0 &&
			/^[a-z0-9_-]+$/.test(installDirName.trim()) &&
			installTools.size > 0 &&
			!(installScope === 'project' && !workspaceId),
	);

	$effect(() => {
		if (!installScopeInitialized) {
			installScope = workspaceId ? 'project' : 'global';
			installScopeInitialized = true;
			return;
		}
		if (!workspaceId && installScope === 'project') {
			installScope = 'global';
		}
	});

	$effect(() => {
		if (!availableToolOptions.some((option) => installTools.has(option.id))) {
			installTools.clear();
			installTools.add('agents');
		}
	});

	const runSearch = async (queryOverride?: string): Promise<void> => {
		const nextQuery = (queryOverride ?? searchQuery).trim();
		const generation = ++searchGeneration;
		searchQuery = nextQuery;
		searchError = null;
		installError = null;

		if (!nextQuery) {
			results = [];
			selectedKey = null;
			selectedSkill = null;
			content = '';
			return;
		}

		loading = true;
		try {
			const items = await searchMarketplaceSkills(
				{
					query: nextQuery,
					limit: 24,
				},
				workspaceId ?? undefined,
			);
			if (generation !== searchGeneration) {
				return;
			}
			results = items;
			void hydrateSearchResults(items, generation);
			void backfillLegacyInstalledSkills(items, generation);

			if (items.length === 0) {
				selectedKey = null;
				selectedSkill = null;
				content = '';
				return;
			}

			const nextSelected =
				items.find((entry) => `${entry.provider}:${entry.externalId}` === selectedKey) ?? items[0];
			await openSkill(nextSelected);
		} catch (error) {
			if (generation === searchGeneration) {
				searchError = toErrorMessage(error, 'Failed to search marketplace');
			}
		} finally {
			if (generation === searchGeneration) {
				loading = false;
			}
		}
	};

	const hydrateSearchResults = async (
		items: MarketplaceSkill[],
		generation: number,
	): Promise<void> => {
		const needsMetadata = items
			.filter(
				(skill) =>
					(skill.auditSummaries?.length ?? 0) === 0 && (skill.listingUrl?.trim().length ?? 0) > 0,
			)
			.slice(0, 8);
		if (needsMetadata.length === 0) {
			return;
		}
		const hydrated = await Promise.allSettled(
			needsMetadata.map((skill) => getMarketplaceSkillMetadata(skill, workspaceId ?? undefined)),
		);
		if (generation !== searchGeneration) {
			return;
		}
		const metadataByKey: Record<string, MarketplaceSkill> = {};
		for (const result of hydrated) {
			if (result.status !== 'fulfilled') {
				continue;
			}
			metadataByKey[skillKey(result.value)] = result.value;
		}
		if (Object.keys(metadataByKey).length === 0) {
			return;
		}
		results = results.map((skill) => metadataByKey[skillKey(skill)] ?? skill);
		if (selectedSkill) {
			selectedSkill = metadataByKey[skillKey(selectedSkill)] ?? selectedSkill;
		}
	};

	const backfillLegacyInstalledSkills = async (
		items: MarketplaceSkill[],
		generation: number,
	): Promise<void> => {
		const legacyInstalled = installedSkills.filter((skill) => !skill.marketplace);
		for (const installed of legacyInstalled) {
			const candidates = items
				.filter((item) => slugifyDirName(item) === installed.dirName)
				.slice(0, 4);
			if (candidates.length === 0) {
				continue;
			}

			let localContent: string;
			try {
				const local = await getSkill(
					installed.scope,
					installed.dirName,
					resolveInstalledTool(installed),
					workspaceId ?? undefined,
				);
				localContent = normalizeSkillContent(local.content);
			} catch {
				continue;
			}

			const matches: MarketplaceSkill[] = [];
			for (const candidate of candidates) {
				try {
					const remote = await getMarketplaceSkillContent(candidate, workspaceId ?? undefined);
					if (normalizeSkillContent(remote.content) === localContent) {
						matches.push(remote.skill);
					}
				} catch {
					// Ignore fetch failures during legacy provenance backfill.
				}
			}

			if (generation !== searchGeneration || matches.length !== 1) {
				continue;
			}

			const match = matches[0];
			try {
				await attachSkillMarketplaceSource(installed, match, workspaceId ?? undefined);
				const key = skillKey(match);
				const scopes = backfilledInstallScopes[key] ?? [];
				if (!scopes.includes(installed.scope as SkillScope)) {
					backfilledInstallScopes = {
						...backfilledInstallScopes,
						[key]: [...scopes, installed.scope as SkillScope].sort((left, right) =>
							left === right ? 0 : left === 'project' ? -1 : 1,
						),
					};
				}
			} catch {
				// Leave legacy installs unlabeled when provenance cannot be attached.
			}
		}
	};

	const scheduleSearch = (): void => {
		if (debouncedSearch) {
			clearTimeout(debouncedSearch);
		}
		const nextQuery = searchQuery.trim();
		if (nextQuery.length < 2) {
			if (nextQuery.length === 0) {
				void runSearch(DEFAULT_QUERY);
			}
			return;
		}
		debouncedSearch = setTimeout(() => {
			void runSearch(nextQuery);
		}, SEARCH_DEBOUNCE_MS);
	};

	const openSkill = async (skill: MarketplaceSkill): Promise<void> => {
		detailLoading = true;
		installError = null;
		selectedKey = skillKey(skill);
		try {
			const response = await getMarketplaceSkillContent(skill, workspaceId ?? undefined);
			selectedSkill = response.skill;
			results = results.map((entry) =>
				skillKey(entry) === skillKey(response.skill) ? response.skill : entry,
			);
			content = response.content;
			installDirName = slugifyDirName(response.skill);
		} catch (error) {
			installError = toErrorMessage(error, `Failed to load ${skill.name}`);
		} finally {
			detailLoading = false;
		}
	};

	const toggleInstallTool = (toolId: string): void => {
		if (installTools.has(toolId)) {
			installTools.delete(toolId);
		} else {
			installTools.add(toolId);
		}
	};

	const installSelectedSkill = async (): Promise<void> => {
		if (!selectedSkill || !canInstall) return;
		installing = true;
		installError = null;
		try {
			const installedSkill = await installMarketplaceSkill(
				{
					skill: selectedSkill,
					scope: installScope,
					dirName: installDirName.trim(),
					tools: [...installTools],
				},
				workspaceId ?? undefined,
			);
			await onInstalled({
				installedSkill,
				message: `Installed ${installedSkill.name} to ${installedSkill.scope === 'project' ? 'workset' : installedSkill.scope}.`,
			});
		} catch (error) {
			installError = toErrorMessage(error, `Failed to install ${selectedSkill.name}`);
		} finally {
			installing = false;
		}
	};

	onMount(() => {
		void runSearch(DEFAULT_QUERY);
		return () => {
			if (debouncedSearch) {
				clearTimeout(debouncedSearch);
			}
		};
	});
</script>

<div class="marketplace-shell">
	<div class="marketplace-toolbar">
		<label class="marketplace-search">
			<Search size={16} />
			<input
				type="text"
				bind:value={searchQuery}
				placeholder="Search Vercel skills.sh..."
				autocapitalize="off"
				autocorrect="off"
				spellcheck="false"
				oninput={scheduleSearch}
				onkeydown={(event) => {
					if (event.key === 'Enter') {
						void runSearch();
					}
				}}
			/>
		</label>
		<button type="button" class="btn-primary" onclick={() => runSearch()} disabled={loading}>
			{#if loading}
				<LoaderCircle size={16} class="spin" />
			{:else}
				<Search size={16} />
			{/if}
			Search
		</button>
	</div>

	<div class="marketplace-note">
		<ShieldCheck size={14} />
		<span>
			Powered by <strong>Vercel skills.sh</strong>. Security signals below are metadata-based
			provenance checks, not a full code audit by Workset.
		</span>
	</div>

	{#if searchError}
		<div class="banner error">{searchError}</div>
	{/if}
	{#if installError}
		<div class="banner error">{installError}</div>
	{/if}

	{#if !searchQuery.trim() && results.length === 0}
		<div class="marketplace-empty">
			<div class="marketplace-empty-card">
				<Sparkles size={18} />
				<h3>Search Vercel skills.sh</h3>
				<p>
					Workset imports remote skills into your local registry so you keep ownership after
					install.
				</p>
				<div class="quick-searches">
					{#each QUICK_SEARCHES as suggestion (suggestion)}
						<button type="button" class="quick-search" onclick={() => runSearch(suggestion)}>
							{suggestion}
						</button>
					{/each}
				</div>
			</div>
		</div>
	{:else}
		<div class="marketplace-content">
			<div class="marketplace-results">
				{#if loading && results.length === 0}
					<div class="loading-state">
						<LoaderCircle size={16} class="spin" />
						Searching marketplace...
					</div>
				{:else if results.length === 0}
					<div class="empty-state ws-empty-state">No marketplace skills matched that query.</div>
				{:else}
					{#each results as skill (`${skill.provider}:${skill.externalId}`)}
						<button
							type="button"
							class="marketplace-card"
							class:active={selectedKey === `${skill.provider}:${skill.externalId}`}
							onclick={() => openSkill(skill)}
						>
							<div class="marketplace-card-header">
								<div>
									<h3>{skill.name}</h3>
									{#if skill.description}
										<p>{skill.description}</p>
									{/if}
								</div>
								<div class="card-badges">
									{#if isInstalled(skill)}
										{#each getInstalledScopes(skill) as scope (scope)}
											<span class="installed-badge">{formatScopeLabel(scope)} installed</span>
										{/each}
									{/if}
									<span class="provider-badge">skills.sh</span>
								</div>
							</div>
							<div class="marketplace-security-row">
								<span class={`security-pill ${getSecuritySignal(skill).tone}`}
									>{getSecuritySignal(skill).label}</span
								>
								<span class="security-detail">{getSecuritySignal(skill).detail}</span>
							</div>
							{#if hasAuditSummaries(skill)}
								<div class="audit-badges">
									{#each skill.auditSummaries ?? [] as audit (audit.provider)}
										<span class={`audit-chip ${auditStatusTone(audit.status)}`}>
											<strong>{auditLabel(audit.provider)}</strong>
											{audit.status}
										</span>
									{/each}
								</div>
							{/if}
							<div class="marketplace-card-footer">
								<span class="footer-source">{skill.sourceRepo || skill.sourceUrl}</span>
								<div class="footer-metrics">
									<span>{formatInstallCount(skill.installCount)}</span>
									{#if skill.weeklyInstalls != null}
										<span class="footer-metric">
											<TrendingUp size={11} />
											{Intl.NumberFormat('en-US', { notation: 'compact' }).format(
												skill.weeklyInstalls,
											)}/wk
										</span>
									{/if}
									{#if skill.githubStars != null}
										<span class="footer-metric">
											<Star size={11} />
											{Intl.NumberFormat('en-US', { notation: 'compact' }).format(
												skill.githubStars,
											)}
										</span>
									{/if}
								</div>
							</div>
						</button>
					{/each}
				{/if}
			</div>

			<div class="marketplace-detail">
				{#if detailLoading}
					<div class="detail-scroll">
						<div class="loading-state">
							<LoaderCircle size={16} class="spin" />
							Loading skill preview...
						</div>
					</div>
				{:else if !selectedResult || !content}
					<div class="detail-scroll">
						<div class="empty-state ws-empty-state">
							Pick a marketplace skill to inspect and install it.
						</div>
					</div>
				{:else}
					<div class="detail-scroll">
						<div class="marketplace-detail-card">
							<div class="marketplace-detail-header">
								<div>
									<div class="detail-title-row">
										<h3>{selectedResult.name}</h3>
										{#if isInstalled(selectedResult)}
											{#each getInstalledScopes(selectedResult) as scope (scope)}
												<span class="installed-badge">{formatScopeLabel(scope)} installed</span>
											{/each}
										{/if}
										<span class="provider-badge">skills.sh</span>
									</div>
									{#if selectedResult.description}
										<p>{selectedResult.description}</p>
									{/if}
								</div>
								<a href={selectedResult.sourceUrl} target="_blank" rel="noreferrer">
									<ExternalLink size={14} />
									Source
								</a>
							</div>

							<div class="marketplace-metadata">
								<span>{formatInstallCount(selectedResult.installCount)}</span>
								{#if selectedResult.weeklyInstalls != null}
									<span>{formatInstallCount(selectedResult.weeklyInstalls)} weekly</span>
								{/if}
								{#if selectedResult.githubStars != null}
									<span
										>{Intl.NumberFormat('en-US', { notation: 'compact' }).format(
											selectedResult.githubStars,
										)} GitHub stars</span
									>
								{/if}
								{#if selectedResult.firstSeen}
									<span>First seen {selectedResult.firstSeen}</span>
								{/if}
								{#if selectedResult.repoVerified}
									<span class="verified-pill">
										<ShieldCheck size={12} />
										Verified org
									</span>
								{/if}
								{#if selectedResult.verified}
									<span class="verified-pill">
										<ShieldCheck size={12} />
										Verified
									</span>
								{/if}
								{#if selectedResult.trustScore != null}
									<span>Trust {selectedResult.trustScore.toFixed(1)}</span>
								{/if}
							</div>

							<div class="security-panel">
								<div class="security-panel-header">
									<div>
										<h4>Security Audit Signals</h4>
										<p>Provider metadata and provenance, not a Workset code audit.</p>
									</div>
									<span class={`security-pill ${getSecuritySignal(selectedResult).tone}`}
										>{getSecuritySignal(selectedResult).label}</span
									>
								</div>
								<div class="security-summary">
									<LockKeyhole size={14} />
									<span>{getSecuritySignal(selectedResult).detail}</span>
								</div>
								{#if hasAuditSummaries(selectedResult)}
									<div class="audit-grid">
										{#each selectedResult.auditSummaries ?? [] as audit (audit.provider)}
											<a
												class={`audit-card ${auditStatusTone(audit.status)}`}
												href={audit.detailUrl ||
													selectedResult.listingUrl ||
													selectedResult.sourceUrl}
												target="_blank"
												rel="noreferrer"
											>
												<span class="audit-card-label">{audit.provider}</span>
												<strong>{audit.status}</strong>
											</a>
										{/each}
									</div>
								{/if}
								<div class="security-facts">
									<span>Source repo: {selectedResult.sourceRepo || 'Unknown'}</span>
									<span>Raw host: {getSourceHost(selectedResult.rawSkillUrl)}</span>
									{#if selectedResult.trustScore != null}
										<span>Trust score: {selectedResult.trustScore.toFixed(1)}</span>
									{/if}
									{#if selectedResult.benchmarkScore != null}
										<span>Benchmark score: {selectedResult.benchmarkScore}</span>
									{/if}
									<span>Installs: {formatInstallCount(selectedResult.installCount)}</span>
								</div>
							</div>

							<div class="marketplace-preview">
								<!-- eslint-disable-next-line svelte/no-at-html-tags -->
								{@html renderedMarkdown}
							</div>
						</div>
					</div>

					<!-- Sticky install bar -->
					<div class="install-bar-wrapper">
						{#if installConfigOpen}
							<div class="install-config">
								<div class="install-config-row">
									<label class="install-config-field">
										<span>Directory name</span>
										<input
											type="text"
											bind:value={installDirName}
											autocapitalize="off"
											autocorrect="off"
											spellcheck="false"
										/>
									</label>
								</div>
								<div class="install-config-row">
									<span class="install-config-label">Target tools</span>
									<div class="install-tools">
										{#each availableToolOptions as option (option.id)}
											<button
												type="button"
												class="tool-pill"
												class:active={installTools.has(option.id)}
												onclick={() => toggleInstallTool(option.id)}
											>
												{option.label}
											</button>
										{/each}
									</div>
								</div>
							</div>
						{/if}
						<div class="install-bar">
							<div class="install-bar-left">
								<div class="scope-toggle">
									<button
										type="button"
										class="scope-btn"
										class:active={installScope === 'global'}
										onclick={() => {
											installScope = 'global';
										}}>Global</button
									>
									<button
										type="button"
										class="scope-btn"
										class:active={installScope === 'project'}
										disabled={!workspaceId}
										onclick={() => {
											installScope = 'project';
										}}>Workset</button
									>
								</div>
								<button
									type="button"
									class="install-config-toggle"
									class:active={installConfigOpen}
									onclick={() => {
										installConfigOpen = !installConfigOpen;
									}}
									title="Install options"
								>
									<Settings size={14} />
									{#if installConfigOpen}
										<ChevronDown size={12} />
									{:else}
										<ChevronUp size={12} />
									{/if}
								</button>
							</div>
							<button
								type="button"
								class="btn-primary"
								onclick={installSelectedSkill}
								disabled={!canInstall || installing}
							>
								{#if installing}
									<LoaderCircle size={16} class="spin" />
									Installing...
								{:else}
									<Download size={16} />
									Install
								{/if}
							</button>
						</div>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style src="./SkillMarketplacePanel.css"></style>
