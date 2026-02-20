<script lang="ts">
	import { onMount } from 'svelte';
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
		Sparkles,
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

	// No props needed — this view is self-contained.

	type ToolOption = {
		id: string;
		label: string;
		hint?: string; // subtitle shown below label
		globalOnly?: boolean; // false = no global dir (project-only)
	};

	const TOOL_OPTIONS: ToolOption[] = [
		{
			id: 'agents',
			label: 'Agents',
			hint: 'Universal standard — Amp, Factory, Crush, Pi, Cline, Windsurf',
		},
		{ id: 'claude', label: 'Claude' },
		{ id: 'codex', label: 'Codex' },
		{ id: 'copilot', label: 'Copilot', hint: 'Project-scoped only', globalOnly: false },
		{ id: 'cursor', label: 'Cursor' },
		{ id: 'opencode', label: 'OpenCode' },
	];

	type ScopeFilter = 'all' | 'global' | 'project';
	type SkillScope = 'global' | 'project';
	type DetailTab = 'rendered' | 'raw';

	const INITIAL_SKILL = `---
name: example-skill
description: one sentence about what this skill does
---

# Example Skill

Add task-specific guidance here.
`;

	// Color palette for skill card icons
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

	const getIconColor = (name: string): string => {
		let hash = 0;
		for (let i = 0; i < name.length; i++) {
			hash = (hash << 5) - hash + name.charCodeAt(i);
			hash |= 0;
		}
		return ICON_COLORS[Math.abs(hash) % ICON_COLORS.length];
	};

	const skillKey = (skill: Pick<SkillInfo, 'scope' | 'dirName'>): string =>
		`${skill.scope}:${skill.dirName}`;

	const toErrorMessage = (error: unknown, fallback: string): string =>
		error instanceof Error ? error.message : fallback;

	let loading = $state(true);
	let detailLoading = $state(false);
	let saving = $state(false);
	let deleting = $state(false);
	let creating = $state(false);

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
	let newTools = $state<Set<string>>(new Set(['agents']));
	let newContent = $state(INITIAL_SKILL);

	let copySuccess = $state(false);

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

	type ViewMode = 'grid' | 'detail' | 'create' | 'loading';

	const viewMode = $derived.by<ViewMode>(() => {
		if (creating) return 'create';
		if (selectedSkill && !detailLoading) return 'detail';
		if (detailLoading) return 'loading';
		return 'grid';
	});

	/** Strip YAML frontmatter (--- delimited block) before rendering. */
	const stripFrontmatter = (raw: string): string => {
		const trimmed = raw.trimStart();
		if (!trimmed.startsWith('---')) return raw;
		const end = trimmed.indexOf('---', 3);
		if (end === -1) return raw;
		return trimmed.slice(end + 3).trimStart();
	};

	const renderedMarkdown = $derived.by(() => {
		if (!editorContent) return '';
		try {
			const body = stripFrontmatter(editorContent);
			const rendered = marked.parse(body, { async: false }) as string;
			return DOMPurify.sanitize(rendered);
		} catch {
			return '<p>Failed to render markdown.</p>';
		}
	});

	const rawLines = $derived.by(() => {
		if (!editorContent) return [];
		return editorContent.split('\n');
	});

	const hasUnsavedChanges = $derived(
		!creating && selectedSkill !== null && editorContent !== originalContent,
	);

	const canCreate = $derived(
		creating &&
			newDirName.trim().length > 0 &&
			/^[a-z0-9_-]+$/.test(newDirName.trim()) &&
			newTools.size > 0 &&
			newContent.trim().length > 0,
	);

	const resetCreateForm = (): void => {
		newDirName = '';
		newScope = 'global';
		newTools = new Set(['agents']);
		newContent = INITIAL_SKILL;
	};

	const clearDetail = (): void => {
		selectedKey = null;
		editorContent = '';
		originalContent = '';
		editorTool = 'codex';
		detailTab = 'rendered';
	};

	const openSkillDetail = async (skill: SkillInfo): Promise<void> => {
		detailLoading = true;
		error = null;
		success = null;
		selectedKey = skillKey(skill);
		creating = false;
		detailTab = 'rendered';
		try {
			const content = await loadSkillContent(skill);
			editorTool = resolvePreferredTool(skill);
			editorContent = content.content;
			originalContent = content.content;
		} catch (loadError) {
			error = toErrorMessage(loadError, 'Failed to load skill content');
			editorContent = '';
			originalContent = '';
		} finally {
			detailLoading = false;
		}
	};

	const goBackToGrid = (): void => {
		clearDetail();
		creating = false;
	};

	const MIN_SPIN_MS = 600;

	const refreshSkills = async (targetKey?: string | null): Promise<void> => {
		loading = true;
		error = null;
		success = null;
		const spinStart = Date.now();
		try {
			const state = await loadSkillsState();
			skills = state.items;
			if (state.error) {
				error = state.error;
			}
			const desiredKey = targetKey ?? selectedKey;
			if (desiredKey) {
				const selected = state.items.find((entry) => skillKey(entry) === desiredKey);
				if (selected) {
					await openSkillDetail(selected);
				}
			}
		} catch (loadError) {
			error = toErrorMessage(loadError, 'Failed to load skills');
		} finally {
			const elapsed = Date.now() - spinStart;
			if (elapsed < MIN_SPIN_MS) {
				await new Promise((resolve) => setTimeout(resolve, MIN_SPIN_MS - elapsed));
			}
			loading = false;
		}
	};

	const startCreate = (): void => {
		creating = true;
		error = null;
		success = null;
		clearDetail();
		resetCreateForm();
	};

	const cancelCreate = async (): Promise<void> => {
		creating = false;
		resetCreateForm();
		await refreshSkills(selectedKey);
	};

	const saveSelected = async (): Promise<void> => {
		if (!selectedSkill) return;
		saving = true;
		error = null;
		success = null;
		try {
			await saveSkillContent(selectedSkill.scope, selectedSkill.dirName, editorTool, editorContent);
			originalContent = editorContent;
			success = `Saved ${selectedSkill.name}.`;
			await refreshSkills(skillKey(selectedSkill));
		} catch (saveError) {
			error = toErrorMessage(saveError, `Failed to save ${selectedSkill.name}`);
		} finally {
			saving = false;
		}
	};

	const toggleTool = (toolId: string): void => {
		const next = new Set(newTools);
		if (next.has(toolId)) {
			next.delete(toolId);
		} else {
			next.add(toolId);
		}
		newTools = next;
	};

	/** Derive the visible tool options – filter out project-only tools when scope is global. */
	const availableToolOptions = $derived.by(() =>
		TOOL_OPTIONS.filter((opt) => !(newScope === 'global' && opt.globalOnly === false)),
	);

	const createSkill = async (): Promise<void> => {
		if (!canCreate) return;
		saving = true;
		error = null;
		success = null;
		const dirName = newDirName.trim();
		const tools = [...newTools];
		try {
			// Save to each selected tool directory
			for (const tool of tools) {
				await saveSkillContent(newScope, dirName, tool, newContent);
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
			await removeSkill(selectedSkill);
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
			// Fallback: ignore clipboard errors
		}
	};

	onMount(() => {
		void refreshSkills();
	});
</script>

<div class="registry-shell">
	<!-- Header -->
	<header class="reg-header">
		<div>
			<h1>Skill Registry</h1>
			<p>Markdown-defined capabilities that shape how your agent works.</p>
		</div>
		<div class="header-actions">
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
			<button type="button" class="btn-marketplace">
				<Download size={16} />
				Marketplace
			</button>
		</div>
	</header>

	{#if error}
		<div class="banner error">{error}</div>
	{/if}
	{#if success}
		<div class="banner success">{success}</div>
	{/if}

	<!-- Main container -->
	<div class="main-container">
		{#if viewMode === 'create'}
			<!-- Create skill form -->
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
									<option value="project">Project</option>
								</select>
							</div>
						</div>
						<div class="input-group">
							<p class="input-label">
								Target tools
								<span class="field-hint"
									>— skill will be written to each selected tool's directory</span
								>
							</p>
							<div class="tool-chips">
								{#each availableToolOptions as opt (opt.id)}
									<button
										type="button"
										class="tool-chip"
										class:selected={newTools.has(opt.id)}
										onclick={() => toggleTool(opt.id)}
									>
										<span class="chip-check">{newTools.has(opt.id) ? '✓' : ''}</span>
										<span class="chip-label">{opt.label}</span>
									</button>
								{/each}
							</div>
							{#if newTools.has('agents')}
								<p class="tool-hint">
									<Sparkles size={11} />
									<span
										><strong>Agents</strong> is the universal standard — covers Amp, Factory AI, Crush,
										Pi, Cline, Windsurf, and more.</span
									>
								</p>
							{/if}
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
						<button type="button" class="btn-ghost" onclick={cancelCreate} disabled={saving}
							>Cancel</button
						>
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
		{:else if viewMode === 'detail' && selectedSkill}
			<!-- Detail view wrapper for transition -->
			<div class="detail-wrapper" in:fly={{ x: 30, duration: 420, easing: cubicOut }}>
				<div class="detail-header-bar">
					<button type="button" class="back-link" onclick={goBackToGrid}>
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
								<span class="scope-badge">{selectedSkill.scope}</span>
								<span class="detail-path">
									<FolderOpen size={9} />
									{selectedSkill.path}
								</span>
							</div>
						</div>
					</div>

					<!-- View mode toggle -->
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

					<!-- Actions -->
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

				<!-- Content area -->
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
								{#each rawLines as line, i (i)}
									<div class="raw-line">
										<span class="line-no">{i + 1}</span>
										<span class="line-text">{line}</span>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</div>

				<!-- Save bar (only when there are changes) -->
				{#if hasUnsavedChanges}
					<div class="save-bar">
						<span class="save-hint">You have unsaved changes</span>
						<button
							type="button"
							class="btn-ghost"
							onclick={() => (editorContent = originalContent)}
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
		{:else if viewMode === 'loading'}
			<div class="loading-state" in:fade={{ duration: 120 }}>
				<LoaderCircle size={16} class="spin" /> Loading skill content...
			</div>
		{:else}
			<!-- Grid wrapper for transition -->
			<div class="grid-wrapper" in:fly={{ x: -30, duration: 420, easing: cubicOut }}>
				<!-- Toolbar -->
				<div class="toolbar">
					<label class="search-input">
						<Search size={16} />
						<input
							type="text"
							bind:value={searchQuery}
							placeholder="Search skills..."
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</label>
					<div class="scope-select">
						<select bind:value={scopeFilter}>
							<option value="all">All Scopes</option>
							<option value="global">Global</option>
							<option value="project">Project</option>
						</select>
					</div>
				</div>

				<!-- Grid content -->
				<div class="grid-content">
					{#if loading}
						<div class="loading-state">
							<LoaderCircle size={16} class="spin" /> Loading skills...
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
										<span class="scope-badge">{skill.scope}</span>
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
