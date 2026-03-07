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

	const { workspaceId = null }: { workspaceId?: string | null } = $props();

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

	const availableToolOptions = $derived.by(() =>
		TOOL_OPTIONS.filter((option) => !(newScope === 'global' && option.globalOnly === false)),
	);

	$effect(() => {
		if (!workspaceId && newScope === 'project') {
			newScope = 'global';
		}
	});

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
				await saveSkillContent(newScope, dirName, tool, newContent, workspaceId ?? undefined);
			}
			creating = false;
			success = `Created ${dirName} for ${tools.join(', ')}.`;
			await refreshSkills(`${newScope}:${dirName}`);
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

<div class="registry-shell">
	<header class="reg-header">
		<div>
			<h1>Skill Registry</h1>
			<p>Markdown-defined capabilities that shape how your agent works.</p>
		</div>
		<div class="header-actions">
			<button
				type="button"
				class="btn-marketplace"
				class:active={surfaceTab === 'installed'}
				onclick={showInstalledSurface}
			>
				<FileCode2 size={16} />
				Installed
			</button>
			<button
				type="button"
				class="btn-marketplace"
				class:active={surfaceTab === 'marketplace'}
				onclick={showMarketplaceSurface}
			>
				<Download size={16} />
				Marketplace
			</button>
			{#if surfaceTab === 'installed'}
				<button
					type="button"
					class="refresh-btn"
					class:refreshing={loading}
					onclick={() => refreshSkills(selectedKey)}
					disabled={loading}
					title="Refresh skills"
				>
					<RefreshCw size={14} />
				</button>
				<button type="button" class="btn-primary" onclick={startCreate} disabled={loading}>
					<Plus size={16} />
					New Skill
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
		{#if surfaceTab === 'marketplace'}
			<SkillMarketplacePanel {workspaceId} installedSkills={skills} onInstalled={handleMarketplaceInstalled} />
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
							<div class="select-wrap">
								<select id="create-skill-scope" bind:value={newScope}>
									<option value="global">Global</option>
									<option value="project" disabled={!workspaceId}>Workset</option>
								</select>
							</div>
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
						<button
							type="button"
							class="btn-primary"
							onclick={createSkill}
							disabled={!canCreate || saving}
						>
							{saving ? 'Creating...' : 'Create Skill'}
						</button>
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
						<button type="button" class="btn-primary" onclick={saveSelected} disabled={saving}>
							{saving ? 'Saving...' : 'Save Changes'}
						</button>
					</div>
				{/if}
			</div>
		{:else}
			<div class="grid-wrapper" in:fly={{ x: -30, duration: 420, easing: cubicOut }}>
				<div class="toolbar">
					<label class="search-input">
						<Search size={16} />
						<input
							type="text"
							bind:value={searchQuery}
							placeholder="Search installed skills..."
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</label>
					<div class="scope-select">
						<select bind:value={scopeFilter}>
							<option value="all">All Scopes</option>
							<option value="global">Global</option>
							<option value="project">Workset</option>
						</select>
					</div>
				</div>
				<div class="grid-content">
					{#if loading}
						<div class="loading-state">
							<LoaderCircle size={16} class="spin" />
							Loading skills...
						</div>
					{:else if filteredSkills.length === 0}
						<div class="empty-state ws-empty-state">No skills matched your current filters.</div>
					{:else}
						<div class="card-grid">
							{#each filteredSkills as skill (skillKey(skill))}
								<button type="button" class="skill-card" onclick={() => openSkillDetail(skill)}>
									<div class="card-top">
										<div class="card-icon" style="color: {getIconColor(skill.name)};">
											<FileCode2 size={20} />
										</div>
									</div>
									<div class="card-body">
										<h3>{skill.name}</h3>
										<p>{skill.description || 'No description'}</p>
									</div>
									<div class="card-footer">
										<span class="scope-badge">{scopeLabel(skill.scope as SkillScope)}</span>
										<span class="skill-md-label">
											<FileText size={10} />
											SKILL.md
										</span>
									</div>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</div>
</div>

<style src="./SkillRegistryView.css"></style>
