<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { fly, fade } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import {
		ArrowLeft,
		Check,
		Code2,
		Copy,
		Download,
		Eye,
		FileCode2,
		FileText,
		FolderOpen,
		LoaderCircle,
		Plus,
		RefreshCw,
		Search,
		Trash2,
		X,
	} from '@lucide/svelte';
	import DOMPurify from 'dompurify';
	import { marked } from 'marked';
	import type { SkillInfo } from '../../api/skills';
	import {
		loadSkillContent,
		loadSkillsState,
		removeSkill,
		resolvePreferredTool,
		saveSkillContent,
	} from '../../view-models/skillsViewModel';
	import SkillMarketplacePanel from './SkillMarketplacePanel.svelte';
	import Button from '../../components/ui/Button.svelte';
	import Select from '../../components/ui/Select.svelte';

	type ToolOption = {
		id: string;
		label: string;
		globalOnly?: boolean;
	};

	type ScopeFilter = 'all' | 'global' | 'project';
	type SkillScope = 'global' | 'project';
	type DetailTab = 'rendered' | 'raw';
	type SurfaceTab = 'installed' | 'marketplace';

	const TOOL_OPTIONS: ToolOption[] = [
		{ id: 'agents', label: 'Agents' },
		{ id: 'claude', label: 'Claude' },
		{ id: 'codex', label: 'Codex' },
		{ id: 'copilot', label: 'Copilot', globalOnly: false },
		{ id: 'cursor', label: 'Cursor' },
		{ id: 'opencode', label: 'OpenCode' },
	];

	const INITIAL_SKILL = `---
name: example-skill
description: one sentence about what this skill does
---

# Example Skill

Add task-specific guidance here.
`;

	const ICON_COLORS = [
		'#5E6AD2',
		'#EF4444',
		'#2D8CFF',
		'#86C442',
		'#F28C28',
		'#06B6D4',
		'#EC4899',
		'#8B5CF6',
	];

	const { workspaceId = null, onClose }: { workspaceId?: string | null; onClose?: () => void } =
		$props();

	const getIconColor = (name: string): string => {
		let hash = 0;
		for (let index = 0; index < name.length; index += 1) {
			hash = (hash << 5) - hash + name.charCodeAt(index);
			hash |= 0;
		}
		return ICON_COLORS[Math.abs(hash) % ICON_COLORS.length];
	};

	const skillKey = (skill: Pick<SkillInfo, 'scope' | 'dirName'>): string =>
		`${skill.scope}:${skill.dirName}`;
	const scopeLabel = (scope: SkillScope): string => (scope === 'project' ? 'workset' : 'global');

	const toErrorMessage = (error: unknown, fallback: string): string =>
		error instanceof Error ? error.message : fallback;

	const stripFrontmatter = (raw: string): string => {
		const trimmed = raw.trimStart();
		if (!trimmed.startsWith('---')) return raw;
		const end = trimmed.indexOf('---', 3);
		if (end === -1) return raw;
		return trimmed.slice(end + 3).trimStart();
	};

	let loading = $state(true);
	let detailLoading = $state(false);
	let saving = $state(false);
	let deleting = $state(false);
	let creating = $state(false);
	let surfaceTab = $state<SurfaceTab>('installed');

	let error = $state<string | null>(null);
	let success = $state<string | null>(null);

	let searchQuery = $state('');
	let marketplaceSearchQuery = $state('');
	let scopeFilter = $state<ScopeFilter>('all');
	let skills = $state<SkillInfo[]>([]);
	let selectedKey = $state<string | null>(null);

	let editorContent = $state('');
	let originalContent = $state('');
	let editorTool = $state<string>('codex');
	let detailTab = $state<DetailTab>('rendered');

	let newDirName = $state('');
	let newScope = $state<SkillScope>('global');
	const newTools = new SvelteSet<string>(['agents']);
	let newContent = $state(INITIAL_SKILL);
	let copySuccess = $state(false);
	let lastLoadedWorkspaceId: string | null = null;
	let searchInputEl = $state<HTMLInputElement | null>(null);

	const sortedSkills = $derived.by<SkillInfo[]>(() => {
		const priority = { global: 0, project: 1 };
		return [...skills].sort((left, right) => {
			const scopeDiff =
				(priority[left.scope as SkillScope] ?? 2) - (priority[right.scope as SkillScope] ?? 2);
			if (scopeDiff !== 0) return scopeDiff;
			return left.name.localeCompare(right.name);
		});
	});

	const filteredSkills = $derived.by<SkillInfo[]>(() => {
		const normalized = searchQuery.trim().toLowerCase();
		return sortedSkills.filter((skill) => {
			if (scopeFilter !== 'all' && skill.scope !== scopeFilter) return false;
			if (!normalized) return true;
			return `${skill.name} ${skill.description} ${skill.dirName} ${skill.path}`
				.toLowerCase()
				.includes(normalized);
		});
	});

	const selectedSkill = $derived.by<SkillInfo | null>(
		() => skills.find((entry) => skillKey(entry) === selectedKey) ?? null,
	);

	const renderedMarkdown = $derived.by(() => {
		if (!editorContent) return '';
		try {
			const rendered = marked.parse(stripFrontmatter(editorContent), { async: false }) as string;
			return DOMPurify.sanitize(rendered);
		} catch {
			return '<p>Failed to render markdown.</p>';
		}
	});

	const rawLines = $derived.by(() => editorContent.split('\n'));

	const hasUnsavedChanges = $derived(
		!creating && selectedSkill !== null && editorContent !== originalContent,
	);

	const canCreate = $derived(
		newDirName.trim().length > 0 &&
			/^[a-z0-9_-]+$/.test(newDirName.trim()) &&
			newTools.size > 0 &&
			newContent.trim().length > 0,
	);

	const effectiveNewScope = $derived<SkillScope>(
		!workspaceId && newScope === 'project' ? 'global' : newScope,
	);

	const availableToolOptions = $derived.by(() =>
		TOOL_OPTIONS.filter(
			(option) => !(effectiveNewScope === 'global' && option.globalOnly === false),
		),
	);

	const resetCreateForm = (): void => {
		newDirName = '';
		newScope = workspaceId ? 'project' : 'global';
		newTools.clear();
		newTools.add('agents');
		newContent = INITIAL_SKILL;
	};

	const clearDetail = (): void => {
		selectedKey = null;
		editorContent = '';
		originalContent = '';
		editorTool = 'codex';
		detailTab = 'rendered';
	};

	const refreshSkills = async (targetKey?: string | null): Promise<void> => {
		loading = true;
		error = null;
		const desiredKey = targetKey ?? selectedKey;
		try {
			const state = await loadSkillsState(workspaceId ?? undefined);
			skills = state.items;
			if (state.error) {
				error = state.error;
			}

			if (desiredKey) {
				const selected = state.items.find((entry) => skillKey(entry) === desiredKey);
				if (selected) {
					await openSkillDetail(selected);
				}
			}
		} catch (refreshError) {
			error = toErrorMessage(refreshError, 'Failed to load skills');
		} finally {
			loading = false;
		}
	};

	const openSkillDetail = async (skill: SkillInfo): Promise<void> => {
		detailLoading = true;
		error = null;
		success = null;
		surfaceTab = 'installed';
		creating = false;
		selectedKey = skillKey(skill);
		detailTab = 'rendered';
		try {
			const content = await loadSkillContent(skill, workspaceId ?? undefined);
			editorTool = resolvePreferredTool(skill);
			editorContent = content.content;
			originalContent = content.content;
		} catch (loadError) {
			error = toErrorMessage(loadError, `Failed to load ${skill.name}`);
			editorContent = '';
			originalContent = '';
		} finally {
			detailLoading = false;
		}
	};

	const showInstalledSurface = (): void => {
		surfaceTab = 'installed';
	};

	const showMarketplaceSurface = (): void => {
		surfaceTab = 'marketplace';
		creating = false;
		clearDetail();
		error = null;
		success = null;
	};

	const startCreate = (): void => {
		surfaceTab = 'installed';
		creating = true;
		error = null;
		success = null;
		clearDetail();
		resetCreateForm();
	};

	const cancelCreate = async (): Promise<void> => {
		creating = false;
		resetCreateForm();
		await refreshSkills();
	};

	const toggleTool = (toolId: string): void => {
		if (newTools.has(toolId)) {
			newTools.delete(toolId);
		} else {
			newTools.add(toolId);
		}
	};

	const saveSelected = async (): Promise<void> => {
		if (!selectedSkill) return;
		saving = true;
		error = null;
		success = null;
		try {
			await saveSkillContent(
				selectedSkill.scope,
				selectedSkill.dirName,
				editorTool,
				editorContent,
				workspaceId ?? undefined,
			);
			originalContent = editorContent;
			success = `Saved ${selectedSkill.name}.`;
			await refreshSkills(skillKey(selectedSkill));
		} catch (saveError) {
			error = toErrorMessage(saveError, `Failed to save ${selectedSkill.name}`);
		} finally {
			saving = false;
		}
	};

	const createSkill = async (): Promise<void> => {
		if (!canCreate) return;
		saving = true;
		error = null;
		success = null;
		const dirName = newDirName.trim();
		const tools = [...newTools];
		try {
			for (const tool of tools) {
				await saveSkillContent(
					effectiveNewScope,
					dirName,
					tool,
					newContent,
					workspaceId ?? undefined,
				);
			}
			creating = false;
			success = `Created ${dirName} for ${tools.join(', ')}.`;
			await refreshSkills(`${effectiveNewScope}:${dirName}`);
		} catch (createError) {
			error = toErrorMessage(createError, `Failed to create ${dirName}`);
		} finally {
			saving = false;
		}
	};

	const deleteSelected = async (): Promise<void> => {
		if (!selectedSkill) return;
		const confirmed = window.confirm(`Delete "${selectedSkill.name}"?`);
		if (!confirmed) return;
		deleting = true;
		error = null;
		success = null;
		const deletedName = selectedSkill.name;
		try {
			await removeSkill(selectedSkill, workspaceId ?? undefined);
			success = `Deleted ${deletedName}.`;
			clearDetail();
			await refreshSkills();
		} catch (deleteError) {
			error = toErrorMessage(deleteError, `Failed to delete ${deletedName}`);
		} finally {
			deleting = false;
		}
	};

	const copyContent = async (): Promise<void> => {
		if (!editorContent) return;
		try {
			await navigator.clipboard.writeText(editorContent);
			copySuccess = true;
			setTimeout(() => (copySuccess = false), 2000);
		} catch {
			// Clipboard failure should not block editing.
		}
	};

	const handleMarketplaceInstalled = async ({ message }: { message: string }): Promise<void> => {
		success = message;
		surfaceTab = 'installed';
		await refreshSkills();
	};

	const handleShellKeydown = (event: KeyboardEvent): void => {
		if (event.key === '/' && !event.metaKey && !event.ctrlKey) {
			const tag = (event.target as HTMLElement)?.tagName;
			if (tag === 'INPUT' || tag === 'TEXTAREA') return;
			event.preventDefault();
			searchInputEl?.focus();
		}
		if (event.key === 'Escape' && onClose && !selectedSkill && !creating) {
			event.preventDefault();
			onClose();
		}
	};

	onMount(() => {
		lastLoadedWorkspaceId = workspaceId;
		void refreshSkills();
	});

	$effect(() => {
		if (lastLoadedWorkspaceId === workspaceId) return;
		lastLoadedWorkspaceId = workspaceId;
		resetCreateForm();
		clearDetail();
		void refreshSkills();
	});
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="registry-shell" onkeydown={handleShellKeydown}>
	<header class="reg-header">
		<h1>Skill Registry</h1>
		<div class="header-actions">
			<Button variant="primary" onclick={startCreate} disabled={loading}>
				<Plus size={16} />
				New Skill
			</Button>
			{#if onClose}
				<button type="button" class="close-btn" onclick={onClose} aria-label="Close skill registry">
					<X size={16} />
				</button>
			{/if}
		</div>
	</header>

	{#if error}
		<div class="banner error">{error}</div>
	{/if}
	{#if success}
		<div class="banner success">{success}</div>
	{/if}

	<div class="main-container">
		<div class="toolbar">
			<div class="toolbar-tabs">
				<button
					type="button"
					class="btn-tab"
					class:active={surfaceTab === 'installed'}
					onclick={showInstalledSurface}
				>
					Installed
				</button>
				<button
					type="button"
					class="btn-tab"
					class:active={surfaceTab === 'marketplace'}
					onclick={showMarketplaceSurface}
				>
					Marketplace
				</button>
			</div>
			<div class="toolbar-sep"></div>
			{#if !creating && !selectedSkill}
				<label class="search-input">
					<Search size={16} />
					<input
						class="ws-field-input ws-field-input--ghost"
						type="text"
						bind:this={searchInputEl}
						value={surfaceTab === 'installed' ? searchQuery : marketplaceSearchQuery}
						oninput={(e) => {
							const val = e.currentTarget.value;
							if (surfaceTab === 'installed') {
								searchQuery = val;
							} else {
								marketplaceSearchQuery = val;
							}
						}}
						placeholder={surfaceTab === 'installed' ? 'Search skills…' : 'Search marketplace…'}
						autocapitalize="off"
						autocorrect="off"
						spellcheck="false"
					/>
				</label>
				{#if surfaceTab === 'installed'}
					<div class="scope-select">
						<Select
							value={scopeFilter}
							options={[
								{ label: 'All Scopes', value: 'all' },
								{ label: 'Global', value: 'global' },
								{ label: 'Workset', value: 'project' },
							]}
							onchange={(val) => (scopeFilter = val as ScopeFilter)}
						/>
					</div>
				{/if}
			{/if}
			<button
				type="button"
				class="ws-icon-action-btn"
				class:refreshing={loading}
				onclick={() => refreshSkills(selectedKey)}
				disabled={loading}
				data-hover-label="Refresh"
			>
				<RefreshCw size={14} />
			</button>
		</div>
		{#if surfaceTab === 'marketplace'}
			<SkillMarketplacePanel
				{workspaceId}
				installedSkills={skills}
				onInstalled={handleMarketplaceInstalled}
				externalSearchQuery={marketplaceSearchQuery}
				onSearchQueryChange={(q) => (marketplaceSearchQuery = q)}
			/>
		{:else if creating}
			<div class="create-view" in:fly={{ y: 20, duration: 420, easing: cubicOut }}>
				<div class="create-card">
					<div class="create-head">
						<div>
							<h3>Create New Skill</h3>
							<p>Define capabilities for your workset agent.</p>
						</div>
					</div>
					<div class="create-fields">
						<div class="input-group">
							<label for="create-skill-dir">Directory name</label>
							<input
								class="ws-field-input"
								id="create-skill-dir"
								type="text"
								bind:value={newDirName}
								placeholder="my-skill"
								autocapitalize="off"
								autocorrect="off"
								spellcheck="false"
							/>
						</div>
						<div class="input-group">
							<label for="create-skill-scope">Scope</label>
							<Select
								id="create-skill-scope"
								value={newScope}
								options={[
									{ label: 'Global', value: 'global' },
									{ label: 'Workset', value: 'project' },
								]}
								onchange={(val) => (newScope = val as SkillScope)}
							/>
						</div>
						<div class="input-group">
							<p class="input-label">Target tools</p>
							<div class="tool-chips">
								{#each availableToolOptions as option (option.id)}
									<button
										type="button"
										class="tool-chip"
										class:selected={newTools.has(option.id)}
										onclick={() => toggleTool(option.id)}
									>
										<span class="chip-check">{newTools.has(option.id) ? '✓' : ''}</span>
										<span class="chip-label">{option.label}</span>
									</button>
								{/each}
							</div>
						</div>
						<div class="input-group">
							<label for="create-skill-content">SKILL.md content</label>
							<div class="textarea-wrap">
								<textarea
									class="ws-field-textarea ws-field-input--ghost ws-field-input--mono"
									id="create-skill-content"
									bind:value={newContent}
									rows="16"
									spellcheck="false"
								></textarea>
							</div>
						</div>
					</div>
					<div class="create-actions">
						<button type="button" class="btn-ghost" onclick={cancelCreate} disabled={saving}>
							Cancel
						</button>
						<Button variant="primary" onclick={createSkill} disabled={!canCreate || saving}>
							{saving ? 'Creating...' : 'Create Skill'}
						</Button>
					</div>
				</div>
			</div>
		{:else if detailLoading}
			<div class="loading-state" in:fade={{ duration: 120 }}>
				<LoaderCircle size={16} class="spin" />
				Loading skill content...
			</div>
		{:else if selectedSkill}
			<div class="detail-wrapper" in:fly={{ x: 30, duration: 420, easing: cubicOut }}>
				<div class="detail-header-bar">
					<button
						type="button"
						class="back-link"
						onclick={() => {
							clearDetail();
						}}
					>
						<ArrowLeft size={16} />
						Back
					</button>
					<div class="header-sep"></div>
					<div class="detail-identity">
						<div class="detail-icon" style="color: {getIconColor(selectedSkill.name)};">
							<FileCode2 size={18} />
						</div>
						<div class="detail-name-block">
							<span class="detail-name">{selectedSkill.name}</span>
							<div class="detail-meta-inline">
								<span class="scope-badge">{scopeLabel(selectedSkill.scope as SkillScope)}</span>
								<span class="detail-path">
									<FolderOpen size={9} />
									{selectedSkill.path}
								</span>
							</div>
						</div>
					</div>
					<div class="view-toggle">
						<button
							type="button"
							class="vtog"
							class:active={detailTab === 'rendered'}
							onclick={() => (detailTab = 'rendered')}
						>
							<Eye size={12} />
							Rendered
						</button>
						<button
							type="button"
							class="vtog vtog-raw"
							class:active={detailTab === 'raw'}
							onclick={() => (detailTab = 'raw')}
						>
							<Code2 size={12} />
							Raw
						</button>
					</div>
					<div class="detail-header-actions">
						<button type="button" class="hdr-action" onclick={copyContent}>
							{#if copySuccess}
								<Check size={12} class="text-success" /> Copied
							{:else}
								<Copy size={12} /> Copy
							{/if}
						</button>
						<button
							type="button"
							class="hdr-action danger"
							onclick={deleteSelected}
							disabled={deleting}
						>
							<Trash2 size={12} />
							{deleting ? 'Removing...' : 'Remove'}
						</button>
					</div>
				</div>

				<div class="detail-content-area">
					{#if detailTab === 'rendered'}
						<div class="rendered-wrap" in:fade={{ duration: 250 }}>
							<div class="rendered-content">
								<!-- eslint-disable-next-line svelte/no-at-html-tags -->
								{@html renderedMarkdown}
							</div>
						</div>
					{:else}
						<div class="raw-wrap" in:fade={{ duration: 250 }}>
							<div class="raw-file-tab">
								<FileText size={13} />
								<span class="raw-file-name">SKILL.md</span>
								<span class="raw-line-count">{rawLines.length} lines</span>
							</div>
							<div class="raw-source">
								{#each rawLines as line, index (index)}
									<div class="raw-line">
										<span class="line-no">{index + 1}</span>
										<span class="line-text">{line}</span>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</div>

				{#if hasUnsavedChanges}
					<div class="save-bar">
						<span class="save-hint">You have unsaved changes</span>
						<button
							type="button"
							class="btn-ghost"
							onclick={() => {
								editorContent = originalContent;
							}}
							disabled={saving}
						>
							Reset
						</button>
						<Button variant="primary" onclick={saveSelected} disabled={saving}>
							{saving ? 'Saving...' : 'Save Changes'}
						</Button>
					</div>
				{/if}
			</div>
		{:else}
			<div class="grid-wrapper" in:fly={{ x: -30, duration: 420, easing: cubicOut }}>
				<div class="grid-content">
					{#if loading}
						<div class="loading-state">
							<LoaderCircle size={16} class="spin" />
							Loading skills...
						</div>
					{:else if skills.length === 0}
						<div class="empty-state empty-first-use">
							<div class="empty-icon">
								<Download size={24} />
							</div>
							<p class="empty-headline">No skills installed yet</p>
							<p>
								Skills teach your agent how to handle specific tasks. Install one from the
								Marketplace to get started.
							</p>
							<button
								type="button"
								class="empty-browse-link empty-primary"
								onclick={showMarketplaceSurface}
							>
								<Download size={14} />
								Browse Marketplace
							</button>
						</div>
					{:else if filteredSkills.length === 0}
						<div class="empty-state">
							<p>No skills matched your filters.</p>
							<button
								type="button"
								class="empty-browse-link"
								onclick={() => {
									searchQuery = '';
									scopeFilter = 'all';
								}}
							>
								Clear filters
							</button>
						</div>
					{:else}
						{@const globalSkills = filteredSkills.filter((s) => s.scope === 'global')}
						{@const projectSkills = filteredSkills.filter((s) => s.scope === 'project')}
						<div class="skill-list">
							{#if globalSkills.length > 0 && scopeFilter !== 'project'}
								<div class="scope-group-header">Global</div>
								{#each globalSkills as skill (skillKey(skill))}
									<button type="button" class="skill-row" onclick={() => openSkillDetail(skill)}>
										<span class="row-name">{skill.name}</span>
										{#if skill.description}
											<span class="row-desc">{skill.description}</span>
										{/if}
									</button>
								{/each}
							{/if}
							{#if projectSkills.length > 0 && scopeFilter !== 'global'}
								<div class="scope-group-header">Project</div>
								{#each projectSkills as skill (skillKey(skill))}
									<button type="button" class="skill-row" onclick={() => openSkillDetail(skill)}>
										<span class="row-name">{skill.name}</span>
										{#if skill.description}
											<span class="row-desc">{skill.description}</span>
										{/if}
									</button>
								{/each}
							{/if}
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</div>
</div>

<style src="./SkillRegistryView.css"></style>
