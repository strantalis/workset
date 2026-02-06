<script lang="ts">
	import { listSkills, getSkill, saveSkill, deleteSkill, syncSkill } from '../../../api';
	import type { SkillInfo } from '../../../api';
	import { activeWorkspace } from '../../../state';
	import SettingsSection from '../SettingsSection.svelte';
	import Button from '../../ui/Button.svelte';

	interface Props {
		onSkillCountChange: (count: number) => void;
	}

	const { onSkillCountChange }: Props = $props();

	const ALL_TOOLS = ['claude', 'codex', 'copilot', 'agents'] as const;
	const TOOL_LABELS: Record<string, string> = {
		claude: 'Claude Code',
		codex: 'Codex',
		copilot: 'Copilot',
		agents: 'Agents',
	};

	let skills: SkillInfo[] = $state([]);
	let selectedSkill: SkillInfo | null = $state(null);
	let isNew = $state(false);
	let editing = $state(false);
	let loading = $state(false);
	let error: string | null = $state(null);
	let success: string | null = $state(null);

	let formDirName = $state('');
	let formContent = $state('');
	let formScope = $state<'global' | 'project'>('global');
	let formTool = $state('claude');
	let syncTargets = $state<Record<string, boolean>>({});

	let autoSync = $state(
		typeof localStorage !== 'undefined' && localStorage.getItem('workset:skills-auto-sync') === '1',
	);

	const availableToolsForSync = (skill: SkillInfo): string[] => {
		return ALL_TOOLS.filter((tool) => {
			if (skill.scope === 'global' && tool === 'copilot') return false;
			return !skill.tools.includes(tool);
		});
	};

	const globalSkills = $derived(skills.filter((s) => s.scope === 'global'));
	const projectSkills = $derived(skills.filter((s) => s.scope === 'project'));
	const totalUnsyncedCount = $derived(
		skills.reduce((count, skill) => count + availableToolsForSync(skill).length, 0),
	);

	const getWorkspaceId = (): string | undefined => $activeWorkspace?.id;

	const formatError = (err: unknown): string => {
		if (err instanceof Error) return err.message;
		return 'An error occurred.';
	};

	const loadSkills = async (): Promise<void> => {
		try {
			skills = await listSkills(getWorkspaceId());
			onSkillCountChange(skills.length);
		} catch (err) {
			error = formatError(err);
		}
	};

	const selectSkill = async (skill: SkillInfo): Promise<void> => {
		selectedSkill = skill;
		isNew = false;
		editing = false;
		error = null;
		success = null;
		// Initialize sync targets
		const targets: Record<string, boolean> = {};
		for (const tool of ALL_TOOLS) {
			targets[tool] = skill.tools.includes(tool);
		}
		syncTargets = targets;
		// Load content from the first available tool
		try {
			const result = await getSkill(skill.scope, skill.dirName, skill.tools[0], getWorkspaceId());
			formContent = result.content;
		} catch (err) {
			formContent = '';
			error = formatError(err);
		}
	};

	const startNew = (): void => {
		selectedSkill = null;
		isNew = true;
		editing = true;
		formDirName = '';
		formContent = '---\nname: \ndescription: \n---\n\n';
		formScope = 'global';
		formTool = 'claude';
		syncTargets = { claude: true, codex: false, copilot: false, agents: false };
		error = null;
		success = null;
	};

	const closeDetail = (): void => {
		selectedSkill = null;
		isNew = false;
		editing = false;
		error = null;
		success = null;
	};

	const cancelEdit = (): void => {
		if (isNew) {
			selectedSkill = null;
			isNew = false;
			editing = false;
		} else {
			editing = false;
		}
		error = null;
		success = null;
	};

	const handleSave = async (): Promise<void> => {
		if (isNew) {
			const dirName = formDirName.trim();
			if (!dirName) {
				error = 'Skill directory name is required.';
				return;
			}
			if (!/^[a-z0-9_-]+$/.test(dirName)) {
				error = 'Directory name must be lowercase alphanumeric with hyphens/underscores.';
				return;
			}
			loading = true;
			error = null;
			success = null;
			try {
				await saveSkill(formScope, dirName, formTool, formContent, getWorkspaceId());
				// Auto-sync to all other tools if enabled
				if (autoSync) {
					const otherTools = ALL_TOOLS.filter((t) => {
						if (t === formTool) return false;
						if (formScope === 'global' && t === 'copilot') return false;
						return true;
					});
					if (otherTools.length > 0) {
						await syncSkill(formScope, dirName, formTool, otherTools, getWorkspaceId());
					}
				}
				success = `Created skill "${dirName}"${autoSync ? ' (synced to all tools)' : ''}.`;
				await loadSkills();
				const created = skills.find((s) => s.dirName === dirName && s.scope === formScope);
				if (created) {
					await selectSkill(created);
				} else {
					isNew = false;
					editing = false;
				}
			} catch (err) {
				error = formatError(err);
			} finally {
				loading = false;
			}
		} else if (selectedSkill) {
			loading = true;
			error = null;
			success = null;
			try {
				await saveSkill(
					selectedSkill.scope,
					selectedSkill.dirName,
					selectedSkill.tools[0],
					formContent,
					getWorkspaceId(),
				);
				success = `Saved "${selectedSkill.name}".`;
				editing = false;
				await loadSkills();
				const updated = skills.find(
					(s) => s.dirName === selectedSkill!.dirName && s.scope === selectedSkill!.scope,
				);
				if (updated) {
					selectedSkill = updated;
				}
			} catch (err) {
				error = formatError(err);
			} finally {
				loading = false;
			}
		}
	};

	const handleDelete = async (): Promise<void> => {
		if (!selectedSkill) return;
		const name = selectedSkill.name;
		const confirmed = window.confirm(`Delete skill "${name}" from all tool directories?`);
		if (!confirmed) return;

		loading = true;
		error = null;
		success = null;
		try {
			for (const tool of selectedSkill.tools) {
				await deleteSkill(selectedSkill.scope, selectedSkill.dirName, tool, getWorkspaceId());
			}
			success = `Deleted "${name}".`;
			selectedSkill = null;
			isNew = false;
			editing = false;
			await loadSkills();
		} catch (err) {
			error = formatError(err);
		} finally {
			loading = false;
		}
	};

	const handleSyncAll = async (): Promise<void> => {
		if (!selectedSkill) return;
		const toTools = availableToolsForSync(selectedSkill);
		if (toTools.length === 0) {
			success = 'Already synced to all tool directories.';
			return;
		}
		// Check all targets then run sync
		for (const tool of toTools) {
			syncTargets[tool] = true;
		}
		await doSync(toTools);
	};

	const handleSync = async (): Promise<void> => {
		if (!selectedSkill) return;
		const toTools = ALL_TOOLS.filter(
			(tool) => syncTargets[tool] && !selectedSkill!.tools.includes(tool),
		);
		if (toTools.length === 0) {
			error = 'No new tools selected for sync.';
			return;
		}
		await doSync(toTools);
	};

	const doSync = async (toTools: string[]): Promise<void> => {
		if (!selectedSkill) return;
		loading = true;
		error = null;
		success = null;
		try {
			await syncSkill(
				selectedSkill.scope,
				selectedSkill.dirName,
				selectedSkill.tools[0],
				toTools,
				getWorkspaceId(),
			);
			success = `Synced to ${toTools.join(', ')}.`;
			await loadSkills();
			const updated = skills.find(
				(s) => s.dirName === selectedSkill!.dirName && s.scope === selectedSkill!.scope,
			);
			if (updated) {
				await selectSkill(updated);
			}
		} catch (err) {
			error = formatError(err);
		} finally {
			loading = false;
		}
	};

	const toggleAutoSync = (): void => {
		autoSync = !autoSync;
		try {
			localStorage.setItem('workset:skills-auto-sync', autoSync ? '1' : '0');
		} catch {
			// ignore storage failures
		}
	};

	const handleSyncAllSkills = async (): Promise<void> => {
		loading = true;
		error = null;
		success = null;
		let synced = 0;
		try {
			for (const skill of skills) {
				const toTools = availableToolsForSync(skill);
				if (toTools.length === 0) continue;
				await syncSkill(skill.scope, skill.dirName, skill.tools[0], toTools, getWorkspaceId());
				synced++;
			}
			if (synced === 0) {
				success = 'All skills already synced to all tools.';
			} else {
				success = `Synced ${synced} skill${synced === 1 ? '' : 's'} to all tool directories.`;
			}
			await loadSkills();
			if (selectedSkill) {
				const updated = skills.find(
					(s) => s.dirName === selectedSkill!.dirName && s.scope === selectedSkill!.scope,
				);
				if (updated) {
					await selectSkill(updated);
				}
			}
		} catch (err) {
			error = formatError(err);
		} finally {
			loading = false;
		}
	};

	const truncateDesc = (desc: string): string => {
		if (desc.length > 60) return desc.substring(0, 57) + '...';
		return desc;
	};

	// Reload skills when workspace changes
	$effect(() => {
		// Access $activeWorkspace to create dependency
		void $activeWorkspace;
		void loadSkills();
	});
</script>

<SettingsSection
	title="Skills"
	description="Manage agent skills (SKILL.md) across Claude Code, Codex, Copilot, and Agent directories."
>
	<div class="manager">
		<div class="list-header">
			<span class="list-count">{skills.length} skill{skills.length === 1 ? '' : 's'}</span>
			<div class="list-actions">
				<Button
					variant="primary"
					size="sm"
					onclick={handleSyncAllSkills}
					disabled={loading || totalUnsyncedCount === 0}
				>
					{loading
						? 'Syncing...'
						: `Sync All${totalUnsyncedCount > 0 ? ` (${totalUnsyncedCount})` : ''}`}
				</Button>
				<Button variant="ghost" size="sm" onclick={startNew}>+ New</Button>
			</div>
		</div>
		<label class="auto-sync-toggle">
			<input type="checkbox" checked={autoSync} onchange={toggleAutoSync} />
			<span>Auto-sync new skills to all tools</span>
		</label>

		{#if skills.length > 0 || isNew}
			<div class="list">
				{#if globalSkills.length > 0}
					<div class="scope-header">Global</div>
					{#each globalSkills as skill (skill.dirName + skill.scope)}
						<button
							class="list-item"
							class:active={selectedSkill?.dirName === skill.dirName &&
								selectedSkill?.scope === skill.scope &&
								!isNew}
							type="button"
							onclick={() => selectSkill(skill)}
						>
							<div class="item-left">
								<span class="item-name">{skill.name}</span>
								{#if skill.description}
									<span class="item-desc">{truncateDesc(skill.description)}</span>
								{/if}
							</div>
							<div class="item-badges">
								{#each ALL_TOOLS as tool (tool)}
									<span
										class="tool-badge"
										class:active={skill.tools.includes(tool)}
										data-tooltip={TOOL_LABELS[tool] ?? tool}
									>
										{tool.charAt(0).toUpperCase()}
									</span>
								{/each}
							</div>
						</button>
					{/each}
				{/if}
				{#if projectSkills.length > 0}
					<div class="scope-header">Project</div>
					{#each projectSkills as skill (skill.dirName + skill.scope)}
						<button
							class="list-item"
							class:active={selectedSkill?.dirName === skill.dirName &&
								selectedSkill?.scope === skill.scope &&
								!isNew}
							type="button"
							onclick={() => selectSkill(skill)}
						>
							<div class="item-left">
								<span class="item-name">{skill.name}</span>
								{#if skill.description}
									<span class="item-desc">{truncateDesc(skill.description)}</span>
								{/if}
							</div>
							<div class="item-badges">
								{#each ALL_TOOLS as tool (tool)}
									<span
										class="tool-badge"
										class:active={skill.tools.includes(tool)}
										data-tooltip={TOOL_LABELS[tool] ?? tool}
									>
										{tool.charAt(0).toUpperCase()}
									</span>
								{/each}
							</div>
						</button>
					{/each}
				{/if}
				{#if isNew}
					<button class="list-item active" type="button">
						<span class="item-name new">New skill</span>
					</button>
				{/if}
			</div>
		{:else}
			<div class="empty">
				<p>No skills found.</p>
				<Button variant="ghost" onclick={startNew}>Create your first skill</Button>
			</div>
		{/if}

		{#if error}
			<div class="message error">{error}</div>
		{:else if success}
			<div class="message success">{success}</div>
		{/if}

		{#if isNew}
			<div class="detail">
				<div class="detail-header">
					<span>New Skill</span>
					<button class="close-btn" type="button" onclick={closeDetail} title="Close">
						<svg
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
						>
							<path d="M18 6L6 18M6 6l12 12" />
						</svg>
					</button>
				</div>
				<div class="form">
					<label class="field">
						<span>Directory name</span>
						<input
							type="text"
							bind:value={formDirName}
							placeholder="my-skill"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</label>
					<label class="field">
						<span>Scope</span>
						<select bind:value={formScope}>
							<option value="global">Global</option>
							<option value="project">Project</option>
						</select>
					</label>
					<label class="field">
						<span>Tool</span>
						<select bind:value={formTool}>
							{#each ALL_TOOLS as tool (tool)}
								{#if formScope === 'global' && tool === 'copilot'}
									<!-- copilot has no global dir -->
								{:else}
									<option value={tool}>{tool}</option>
								{/if}
							{/each}
						</select>
					</label>
					<label class="field">
						<span>SKILL.md content</span>
						<textarea bind:value={formContent} rows="10" spellcheck="false" class="content-editor"
						></textarea>
					</label>
				</div>
				<div class="actions">
					<div class="spacer"></div>
					<Button variant="ghost" onclick={cancelEdit} disabled={loading}>Cancel</Button>
					<Button variant="primary" onclick={handleSave} disabled={loading}>
						{loading ? 'Creating...' : 'Create'}
					</Button>
				</div>
			</div>
		{:else if selectedSkill}
			<div class="detail">
				<div class="detail-header">
					<div class="detail-title">
						<span>{selectedSkill.name}</span>
						{#if selectedSkill.description}
							<span class="detail-desc">{selectedSkill.description}</span>
						{/if}
					</div>
					<button class="close-btn" type="button" onclick={closeDetail} title="Close">
						<svg
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
						>
							<path d="M18 6L6 18M6 6l12 12" />
						</svg>
					</button>
				</div>

				<div class="sync-section">
					<span class="sync-label">Tool directories:</span>
					<div class="sync-tools">
						{#each ALL_TOOLS as tool (tool)}
							{#if selectedSkill.scope === 'global' && tool === 'copilot'}
								<!-- copilot has no global dir -->
							{:else}
								<label class="sync-tool">
									<input
										type="checkbox"
										bind:checked={syncTargets[tool]}
										disabled={selectedSkill.tools.includes(tool)}
									/>
									<span class:synced={selectedSkill.tools.includes(tool)}>{tool}</span>
								</label>
							{/if}
						{/each}
					</div>
					<Button variant="ghost" size="sm" onclick={handleSync} disabled={loading}>Sync</Button>
					<Button
						variant="primary"
						size="sm"
						onclick={handleSyncAll}
						disabled={loading || availableToolsForSync(selectedSkill).length === 0}
					>
						Sync All
					</Button>
				</div>

				{#if editing}
					<label class="field">
						<span>SKILL.md content</span>
						<textarea bind:value={formContent} rows="12" spellcheck="false" class="content-editor"
						></textarea>
					</label>
					<div class="actions">
						<Button variant="danger" onclick={handleDelete} disabled={loading}>Delete</Button>
						<div class="spacer"></div>
						<Button variant="ghost" onclick={cancelEdit} disabled={loading}>Cancel</Button>
						<Button variant="primary" onclick={handleSave} disabled={loading}>
							{loading ? 'Saving...' : 'Save'}
						</Button>
					</div>
				{:else}
					<div class="content-preview">{formContent}</div>
					<div class="actions">
						<Button variant="danger" onclick={handleDelete} disabled={loading}>Delete</Button>
						<div class="spacer"></div>
						<Button variant="ghost" onclick={() => (editing = true)}>Edit</Button>
					</div>
				{/if}
			</div>
		{:else if skills.length > 0}
			<div class="hint">Select a skill to view, or click "+ New" to create one.</div>
		{/if}
	</div>
</SettingsSection>

<style>
	.manager {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.list-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
	}

	.list-actions {
		display: flex;
		gap: var(--space-2);
		align-items: center;
	}

	.list-count {
		font-size: 12px;
		color: var(--muted);
	}

	.auto-sync-toggle {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--muted);
		cursor: pointer;
		padding: 2px 0;
	}

	.auto-sync-toggle input[type='checkbox'] {
		accent-color: var(--accent);
	}

	.auto-sync-toggle span {
		user-select: none;
	}

	.list {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		max-height: 240px;
		overflow-y: auto;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: var(--space-1);
		background: var(--panel);
	}

	.scope-header {
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--muted);
		padding: 6px var(--space-3) 2px;
	}

	.list-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
		padding: 10px var(--space-3);
		border: none;
		background: transparent;
		color: var(--text);
		font-size: 13px;
		font-family: inherit;
		text-align: left;
		border-radius: var(--radius-sm);
		cursor: pointer;
		transition: background var(--transition-fast);
	}

	.list-item:hover {
		background: rgba(255, 255, 255, 0.04);
	}

	.list-item.active {
		background: rgba(255, 255, 255, 0.08);
	}

	.item-left {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
		flex: 1;
	}

	.item-name {
		font-weight: 500;
	}

	.item-name.new {
		font-style: italic;
		color: var(--accent);
	}

	.item-desc {
		font-size: 12px;
		color: var(--muted);
		text-overflow: ellipsis;
		overflow: hidden;
		white-space: nowrap;
	}

	.item-badges {
		display: flex;
		gap: 3px;
		flex-shrink: 0;
	}

	.tool-badge {
		position: relative;
		width: 20px;
		height: 20px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 10px;
		font-weight: 600;
		border-radius: var(--radius-sm);
		border: 1px solid var(--border);
		color: var(--muted);
		background: transparent;
	}

	.tool-badge.active {
		background: var(--accent-soft);
		color: var(--accent);
		border-color: var(--accent);
	}

	.tool-badge::after {
		content: attr(data-tooltip);
		position: absolute;
		bottom: calc(100% + 4px);
		left: 50%;
		transform: translateX(-50%);
		padding: 3px 8px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: 11px;
		font-weight: 400;
		color: var(--text);
		white-space: nowrap;
		pointer-events: none;
		opacity: 0;
		transition: opacity 0.15s ease;
		z-index: 10;
	}

	.tool-badge:hover::after {
		opacity: 1;
	}

	.detail {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		padding: var(--space-4);
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}

	.detail-header {
		font-size: 14px;
		font-weight: 600;
		color: var(--text);
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: var(--space-2);
	}

	.detail-title {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.detail-desc {
		font-size: 12px;
		font-weight: 400;
		color: var(--muted);
	}

	.close-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: none;
		border-radius: var(--radius-sm);
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		flex-shrink: 0;
		transition:
			background var(--transition-fast),
			color var(--transition-fast);
	}

	.close-btn:hover {
		background: rgba(255, 255, 255, 0.08);
		color: var(--text);
	}

	.sync-section {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) 0;
		flex-wrap: wrap;
	}

	.sync-label {
		font-size: 12px;
		color: var(--muted);
	}

	.sync-tools {
		display: flex;
		gap: var(--space-3);
	}

	.sync-tool {
		display: flex;
		align-items: center;
		gap: 4px;
		font-size: 12px;
		color: var(--text);
		cursor: pointer;
	}

	.sync-tool input[type='checkbox'] {
		accent-color: var(--accent);
	}

	.sync-tool .synced {
		color: var(--accent);
		font-weight: 500;
	}

	.form {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 12px;
		color: var(--muted);
	}

	.field input,
	.field select {
		background: var(--panel-strong);
		border: 1px solid rgba(255, 255, 255, 0.08);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 10px var(--space-3);
		font-size: 13px;
		font-family: inherit;
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.field input:focus,
	.field select:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 2px var(--accent-soft);
	}

	.content-editor {
		background: var(--panel-strong);
		border: 1px solid rgba(255, 255, 255, 0.08);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 10px var(--space-3);
		font-size: 13px;
		font-family: var(--font-mono);
		resize: vertical;
		min-height: 120px;
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.content-editor:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 2px var(--accent-soft);
	}

	.content-preview {
		background: var(--panel-strong);
		border: 1px solid rgba(255, 255, 255, 0.08);
		border-radius: var(--radius-md);
		padding: 10px var(--space-3);
		font-size: 13px;
		font-family: var(--font-mono);
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 200px;
		overflow-y: auto;
		color: var(--text);
	}

	.actions {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding-top: var(--space-2);
		border-top: 1px solid var(--border);
	}

	.spacer {
		flex: 1;
	}

	.message {
		font-size: 13px;
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius-md);
	}

	.message.error {
		background: var(--danger-subtle);
		color: var(--danger);
	}

	.message.success {
		background: rgba(74, 222, 128, 0.1);
		color: var(--success);
	}

	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-3);
		padding: 32px;
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		text-align: center;
	}

	.empty p {
		margin: 0;
		color: var(--muted);
		font-size: 14px;
	}

	.hint {
		font-size: 13px;
		color: var(--muted);
		padding: var(--space-4);
		text-align: center;
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}
</style>
