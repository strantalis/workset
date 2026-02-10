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
						<div class="empty-state">No skills matched your current filters.</div>
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

<style>
	/* ─── Shell ─── */
	.registry-shell {
		display: flex;
		flex-direction: column;
		gap: 0;
		height: 100%;
		background: color-mix(in srgb, var(--bg) 90%, transparent);
		padding: 24px;
		overflow: hidden;
	}

	/* ─── Header ─── */
	.reg-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 28px;
	}

	.reg-header h1 {
		margin: 0;
		font-size: var(--text-3xl);
		font-weight: 600;
		color: var(--text);
		letter-spacing: -0.01em;
	}

	.reg-header p {
		margin: 4px 0 0;
		font-size: var(--text-base);
		color: var(--muted);
	}

	.header-actions {
		display: flex;
		gap: 10px;
		align-items: center;
	}

	.refresh-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: rgba(255, 255, 255, 0.02);
		color: var(--text);
		cursor: pointer;
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast),
			color var(--transition-fast);
	}

	.refresh-btn:hover:not(:disabled) {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
		color: var(--accent);
	}

	.refresh-btn:active:not(:disabled) {
		transform: scale(0.92);
	}

	.refresh-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.refresh-btn :global(svg) {
		transition: transform 0.3s ease;
	}

	.refresh-btn.refreshing :global(svg) {
		animation: spin 0.8s linear infinite;
	}

	.btn-primary {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 8px 16px;
		border-radius: 8px;
		border: none;
		background: var(--accent);
		color: white;
		font-size: var(--text-base);
		font-weight: 500;
		cursor: pointer;
		box-shadow: 0 4px 16px color-mix(in srgb, var(--accent) 20%, transparent);
		transition: background var(--transition-fast);
	}

	.btn-primary:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 90%, white);
	}

	.btn-primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-marketplace {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 8px 16px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--accent) 20%, var(--border));
		background: var(--panel-strong);
		color: var(--accent);
		font-size: var(--text-base);
		font-weight: 500;
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.btn-marketplace:hover {
		background: color-mix(in srgb, var(--accent) 10%, var(--panel-strong));
	}

	.btn-ghost {
		padding: 8px 14px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: var(--muted);
		font-size: var(--text-base);
		font-weight: 500;
		cursor: pointer;
	}

	.btn-ghost:hover {
		color: var(--text);
		background: var(--panel-strong);
	}

	/* ─── Banners ─── */
	.banner {
		padding: 8px 12px;
		border-radius: 8px;
		font-size: var(--text-sm);
		margin-bottom: 12px;
	}

	.banner.error {
		background: color-mix(in srgb, var(--status-error) 10%, transparent);
		border: 1px solid color-mix(in srgb, var(--status-error) 30%, var(--border));
		color: color-mix(in srgb, var(--status-error) 74%, white);
	}

	.banner.success {
		background: color-mix(in srgb, var(--success) 20%, transparent);
		border: 1px solid color-mix(in srgb, var(--success) 30%, var(--border));
		color: color-mix(in srgb, var(--success) 70%, white);
	}

	/* ─── Main Container ─── */
	.main-container {
		flex: 1;
		min-height: 0;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
		position: relative;
	}

	/* Transition wrappers */
	.detail-wrapper,
	.grid-wrapper {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
	}

	/* ─── Toolbar ─── */
	.toolbar {
		padding: 14px 16px;
		border-bottom: 1px solid var(--border);
		background: var(--panel-soft);
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.search-input {
		position: relative;
		display: inline-flex;
		align-items: center;
		width: 280px;
	}

	.search-input :global(svg) {
		position: absolute;
		left: 12px;
		color: var(--muted);
		pointer-events: none;
	}

	.search-input input {
		width: 100%;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		padding: 8px 12px 8px 36px;
		color: var(--text);
		font-size: var(--text-base);
		transition: border-color var(--transition-fast);
	}

	.search-input input::placeholder {
		color: color-mix(in srgb, var(--muted) 50%, transparent);
	}

	.search-input input:focus {
		outline: none;
		border-color: color-mix(in srgb, var(--accent) 50%, var(--border));
	}

	.scope-select select {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 8px 32px 8px 12px;
		color: var(--muted);
		font-size: var(--text-base);
		appearance: auto;
	}

	.scope-select select:focus {
		outline: none;
	}

	/* ─── Grid ─── */
	.grid-content {
		flex: 1;
		overflow-y: auto;
		padding: 20px;
	}

	.card-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 14px;
	}

	.skill-card {
		display: flex;
		flex-direction: column;
		padding: 18px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		color: inherit;
		text-align: left;
		cursor: pointer;
		transition: all 200ms ease;
		animation: cardAppear 0.4s cubic-bezier(0.22, 1, 0.36, 1) both;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}

	.skill-card:hover {
		border-color: color-mix(in srgb, var(--accent) 30%, var(--border));
	}

	.card-top {
		margin-bottom: 14px;
	}

	.card-icon {
		width: 48px;
		height: 48px;
		border-radius: 12px;
		display: grid;
		place-items: center;
		background: var(--bg);
		border: 1px solid var(--border);
	}

	.card-body {
		flex: 1;
	}

	.card-body h3 {
		margin: 0 0 4px;
		font-size: var(--text-md);
		font-weight: 600;
		color: var(--text);
	}

	.card-body p {
		margin: 0;
		font-size: var(--text-base);
		color: var(--muted);
		line-height: 1.5;
		line-clamp: 2;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.card-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-top: 14px;
		padding-top: 14px;
		border-top: 1px solid var(--border);
		font-size: var(--text-xs);
	}

	.scope-badge {
		padding: 2px 8px;
		border-radius: 4px;
		font-size: var(--text-xs);
		font-weight: 500;
		background: var(--border);
		color: var(--muted);
		text-transform: capitalize;
	}

	.skill-md-label {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: color-mix(in srgb, var(--muted) 50%, transparent);
	}

	/* ─── Detail Header Bar (Figma parity) ─── */
	.detail-header-bar {
		padding: 12px 20px;
		border-bottom: 1px solid var(--border);
		background: var(--panel-soft);
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.back-link {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 0;
		border: none;
		background: transparent;
		color: var(--muted);
		font-size: var(--text-base);
		cursor: pointer;
		white-space: nowrap;
		transition: color var(--transition-fast);
	}

	.back-link:hover {
		color: var(--text);
	}

	.header-sep {
		width: 1px;
		height: 20px;
		background: var(--border);
		flex-shrink: 0;
	}

	.detail-identity {
		display: flex;
		align-items: center;
		gap: 10px;
		flex: 1;
		min-width: 0;
	}

	.detail-icon {
		width: 36px;
		height: 36px;
		border-radius: 8px;
		display: grid;
		place-items: center;
		background: var(--bg);
		border: 1px solid var(--border);
		flex-shrink: 0;
	}

	.detail-name-block {
		min-width: 0;
	}

	.detail-name {
		display: block;
		font-size: var(--text-base);
		font-weight: 600;
		color: var(--text);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.detail-meta-inline {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 2px;
		font-size: var(--text-xs);
	}

	.detail-path {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: var(--subtle);
	}

	/* View mode toggle */
	.view-toggle {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 2px;
		flex-shrink: 0;
	}

	.vtog {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 5px 10px;
		border-radius: 6px;
		border: 1px solid transparent;
		background: transparent;
		color: var(--muted);
		font-size: var(--text-xs);
		font-weight: 500;
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.vtog:hover {
		color: var(--text);
	}

	.vtog.active {
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		border-color: color-mix(in srgb, var(--accent) 30%, transparent);
	}

	.vtog-raw.active {
		background: color-mix(in srgb, var(--purple) 15%, transparent);
		color: var(--purple);
		border-color: color-mix(in srgb, var(--purple) 30%, transparent);
	}

	/* Header actions */
	.detail-header-actions {
		display: flex;
		gap: 6px;
		flex-shrink: 0;
	}

	.hdr-action {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 5px 10px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		color: var(--muted);
		font-size: var(--text-xs);
		font-weight: 500;
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.hdr-action:hover {
		color: var(--text);
		border-color: color-mix(in srgb, var(--accent) 30%, var(--border));
	}

	.hdr-action.danger {
		color: var(--status-error);
	}

	.hdr-action.danger:hover {
		background: color-mix(in srgb, var(--status-error) 10%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--status-error) 30%, var(--border));
	}

	.hdr-action :global(.text-success) {
		color: var(--success);
	}

	/* ─── Detail Content ─── */
	.detail-content-area {
		flex: 1;
		overflow-y: auto;
	}

	/* Rendered markdown */
	.rendered-wrap {
		padding: 32px;
		max-width: 960px;
		margin: 0 auto;
	}

	.rendered-content :global(h1) {
		font-size: var(--text-3xl);
		font-weight: 600;
		margin: 0 0 14px;
		padding-bottom: 12px;
		border-bottom: 1px solid var(--border);
		color: var(--text);
	}

	.rendered-content :global(h2) {
		font-size: var(--text-xl);
		font-weight: 600;
		margin: 28px 0 10px;
		color: var(--text);
	}

	.rendered-content :global(h3) {
		font-size: var(--text-md);
		font-weight: 600;
		margin: 20px 0 8px;
		color: var(--text);
	}

	.rendered-content :global(p) {
		font-size: var(--text-base);
		color: var(--muted);
		line-height: 1.65;
		margin: 0 0 14px;
	}

	.rendered-content :global(ul) {
		font-size: var(--text-base);
		color: var(--muted);
		margin: 0 0 14px;
		padding-left: 6px;
		list-style: none;
	}

	.rendered-content :global(ol) {
		font-size: var(--text-base);
		color: var(--muted);
		margin: 0 0 14px;
		padding-left: 6px;
		list-style: decimal inside;
	}

	.rendered-content :global(li) {
		margin-bottom: 5px;
		line-height: 1.6;
	}

	.rendered-content :global(ul > li::before) {
		content: '-';
		color: var(--accent);
		margin-right: 8px;
		font-weight: 500;
	}

	.rendered-content :global(strong) {
		color: var(--text);
		font-weight: 600;
	}

	.rendered-content :global(em) {
		color: var(--purple);
		font-style: normal;
	}

	.rendered-content :global(code) {
		background: var(--border);
		color: var(--text);
		padding: 1px 6px;
		border-radius: 4px;
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
	}

	.rendered-content :global(pre) {
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 14px 16px;
		margin: 0 0 14px;
		overflow-x: auto;
	}

	.rendered-content :global(pre code) {
		background: transparent;
		padding: 0;
		font-size: var(--text-mono-sm);
		color: var(--muted);
	}

	.rendered-content :global(blockquote) {
		border-left: 2px solid color-mix(in srgb, var(--accent) 50%, transparent);
		padding-left: 14px;
		margin: 0 0 14px;
		color: var(--muted);
		font-size: var(--text-base);
	}

	.rendered-content :global(table) {
		width: 100%;
		border-collapse: collapse;
		margin: 0 0 14px;
		font-size: var(--text-base);
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
	}

	.rendered-content :global(thead) {
		background: var(--bg);
	}

	.rendered-content :global(th) {
		padding: 8px 14px;
		text-align: left;
		font-size: var(--text-xs);
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--muted);
		border-bottom: 1px solid var(--border);
	}

	.rendered-content :global(td) {
		padding: 8px 14px;
		font-size: var(--text-mono-sm);
		color: var(--muted);
		border-bottom: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
		font-family: var(--font-mono);
	}

	.rendered-content :global(hr) {
		border: none;
		border-top: 1px solid var(--border);
		margin: 20px 0;
	}

	.rendered-content :global(a) {
		color: var(--accent);
		text-decoration: none;
	}

	.rendered-content :global(a:hover) {
		text-decoration: underline;
	}

	/* ─── Raw source view ─── */
	.raw-wrap {
		padding: 20px;
		max-width: 960px;
		margin: 0 auto;
	}

	.raw-file-tab {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 14px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-bottom: none;
		border-radius: 8px 8px 0 0;
		color: var(--muted);
	}

	.raw-file-tab :global(svg) {
		flex-shrink: 0;
	}

	.raw-file-name {
		font-family: var(--font-mono);
		font-size: var(--text-mono-sm);
		color: var(--text);
	}

	.raw-line-count {
		margin-left: auto;
		font-size: var(--text-xs);
		color: var(--subtle);
	}

	.raw-source {
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 0 0 8px 8px;
		overflow-x: auto;
		font-family: var(--font-mono);
		font-size: var(--text-mono-sm);
		line-height: 1.7;
	}

	.raw-line {
		display: flex;
		transition: background 80ms;
	}

	.raw-line:hover {
		background: color-mix(in srgb, var(--text) 1.5%, transparent);
	}

	.line-no {
		width: 44px;
		flex-shrink: 0;
		text-align: right;
		padding-right: 12px;
		user-select: none;
		font-size: var(--text-mono-xs);
		color: color-mix(in srgb, var(--subtle) 40%, transparent);
		border-right: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
	}

	.line-text {
		padding-left: 14px;
		white-space: pre;
		color: var(--muted);
	}

	/* ─── Save bar ─── */
	.save-bar {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 10px 20px;
		border-top: 1px solid var(--border);
		background: var(--panel-soft);
	}

	.save-hint {
		font-size: var(--text-sm);
		color: var(--muted);
		margin-right: auto;
	}

	/* ─── Create form ─── */
	.create-view {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
	}

	.create-card {
		max-width: 720px;
		margin: 0 auto;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 28px;
		box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
	}

	.create-head {
		padding-bottom: 18px;
		margin-bottom: 24px;
		border-bottom: 1px solid var(--border);
	}

	.create-head h3 {
		margin: 0;
		font-size: var(--text-2xl);
		font-weight: 600;
		color: var(--text);
	}

	.create-head p {
		margin: 4px 0 0;
		font-size: var(--text-base);
		color: var(--muted);
	}

	.create-fields {
		display: grid;
		gap: 20px;
	}

	.tool-chips {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.tool-chip {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 14px;
		border-radius: 20px;
		border: 1px solid var(--border);
		background: var(--panel);
		color: var(--muted);
		font-size: var(--text-sm);
		font-weight: 500;
		cursor: pointer;
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast),
			color var(--transition-fast);
		user-select: none;
	}

	.tool-chip:hover {
		border-color: var(--accent);
		color: var(--text);
	}

	.tool-chip.selected {
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 14%, var(--panel));
		color: var(--text);
	}

	.chip-check {
		width: 12px;
		font-size: var(--text-xs);
		color: var(--accent);
	}

	.chip-label {
		line-height: 1;
	}

	.field-hint {
		font-weight: 400;
		text-transform: none;
		letter-spacing: 0;
		font-size: var(--text-xs);
		color: color-mix(in srgb, var(--muted) 60%, transparent);
	}

	.tool-hint {
		display: flex;
		align-items: flex-start;
		gap: 6px;
		margin: 0;
		padding: 8px 12px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--accent) 6%, transparent);
		font-size: var(--text-xs);
		line-height: 1.5;
		color: color-mix(in srgb, var(--muted) 80%, var(--accent));
	}

	.tool-hint :global(svg) {
		flex-shrink: 0;
		margin-top: 2px;
		color: var(--accent);
	}

	.input-group {
		display: grid;
		gap: 8px;
	}

	.input-group label,
	.input-group .input-label {
		font-size: var(--text-xs);
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--muted);
		margin: 0;
	}

	.input-group input {
		width: 100%;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 9px 12px;
		color: var(--text);
		font-size: var(--text-base);
		transition: border-color var(--transition-fast);
	}

	.input-group input::placeholder {
		color: color-mix(in srgb, var(--muted) 30%, transparent);
	}

	.input-group input:focus {
		outline: none;
		border-color: var(--accent);
	}

	.select-wrap select {
		width: 100%;
		appearance: auto;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 9px 12px;
		color: var(--text);
		font-size: var(--text-base);
	}

	.select-wrap select:focus {
		outline: none;
		border-color: var(--accent);
	}

	.textarea-wrap {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 4px;
	}

	.textarea-wrap textarea {
		width: 100%;
		background: transparent;
		border: none;
		color: var(--text);
		font-family: var(--font-mono);
		font-size: var(--text-mono-base);
		line-height: 1.5;
		padding: 8px;
		resize: none;
	}

	.textarea-wrap textarea:focus {
		outline: none;
	}

	.create-actions {
		display: flex;
		justify-content: flex-end;
		gap: 10px;
		margin-top: 24px;
		padding-top: 20px;
		border-top: 1px solid var(--border);
	}

	/* ─── States ─── */
	.loading-state,
	.empty-state {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 40px;
		color: var(--muted);
		font-size: var(--text-base);
	}

	:global(.spin) {
		animation: spin 0.9s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	@keyframes cardAppear {
		from {
			opacity: 0;
			transform: scale(0.95) translateY(6px);
		}
		to {
			opacity: 1;
			transform: scale(1) translateY(0);
		}
	}

	/* Stagger card animations */
	.skill-card:nth-child(1) {
		animation-delay: 0ms;
	}
	.skill-card:nth-child(2) {
		animation-delay: 50ms;
	}
	.skill-card:nth-child(3) {
		animation-delay: 100ms;
	}
	.skill-card:nth-child(4) {
		animation-delay: 150ms;
	}
	.skill-card:nth-child(5) {
		animation-delay: 200ms;
	}
	.skill-card:nth-child(6) {
		animation-delay: 250ms;
	}
	.skill-card:nth-child(7) {
		animation-delay: 300ms;
	}
	.skill-card:nth-child(8) {
		animation-delay: 350ms;
	}
	.skill-card:nth-child(9) {
		animation-delay: 400ms;
	}
	.skill-card:nth-child(10) {
		animation-delay: 450ms;
	}
	.skill-card:nth-child(11) {
		animation-delay: 500ms;
	}
	.skill-card:nth-child(12) {
		animation-delay: 550ms;
	}

	/* ─── Responsive ─── */
	@media (max-width: 1200px) {
		.card-grid {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	@media (max-width: 900px) {
		.reg-header {
			flex-direction: column;
			gap: 12px;
		}

		.header-actions {
			flex-wrap: wrap;
		}

		.detail-header-bar {
			flex-wrap: wrap;
		}
	}

	@media (max-width: 700px) {
		.card-grid {
			grid-template-columns: 1fr;
		}

		.toolbar {
			flex-direction: column;
			align-items: stretch;
		}

		.search-input {
			width: 100%;
		}
	}
</style>
