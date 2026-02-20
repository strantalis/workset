<script lang="ts">
	import { listSkills, getSkill, saveSkill, deleteSkill, syncSkill } from '../../../api/skills';
	import type { SkillInfo } from '../../../api/skills';
	import { activeWorkspace } from '../../../state';
	import { toErrorMessage } from '../../../errors';
	import SettingsSection from '../SettingsSection.svelte';
	import Button from '../../ui/Button.svelte';
	import SkillDetailPanel from './SkillDetailPanel.svelte';

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

	const loadSkills = async (): Promise<void> => {
		try {
			skills = await listSkills(getWorkspaceId());
			onSkillCountChange(skills.length);
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
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
			error = toErrorMessage(err, 'An error occurred.');
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
				error = toErrorMessage(err, 'An error occurred.');
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
				error = toErrorMessage(err, 'An error occurred.');
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
			error = toErrorMessage(err, 'An error occurred.');
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
			error = toErrorMessage(err, 'An error occurred.');
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
			error = toErrorMessage(err, 'An error occurred.');
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
			<div class="empty ws-empty-state">
				<p class="ws-empty-state-copy">No skills found.</p>
				<Button variant="ghost" onclick={startNew}>Create your first skill</Button>
			</div>
		{/if}

		{#if error}
			<div class="message error ws-message ws-message-error">{error}</div>
		{:else if success}
			<div class="message success ws-message ws-message-success">{success}</div>
		{/if}

		<SkillDetailPanel
			skillsCount={skills.length}
			{isNew}
			{selectedSkill}
			{editing}
			{loading}
			{formDirName}
			{formContent}
			{formScope}
			{formTool}
			{syncTargets}
			allTools={ALL_TOOLS}
			onCloseDetail={closeDetail}
			onCancelEdit={cancelEdit}
			onSave={handleSave}
			onDelete={handleDelete}
			onSync={handleSync}
			onSyncAll={handleSyncAll}
			onStartEdit={() => (editing = true)}
			onFormDirNameChange={(value) => (formDirName = value)}
			onFormContentChange={(value) => (formContent = value)}
			onFormScopeChange={(value) => (formScope = value)}
			onFormToolChange={(value) => (formTool = value)}
			onSyncTargetChange={(tool, checked) => {
				syncTargets = { ...syncTargets, [tool]: checked };
			}}
			availableToolsForSyncCount={selectedSkill ? availableToolsForSync(selectedSkill).length : 0}
		/>
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
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.auto-sync-toggle {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-sm);
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
		font-size: var(--text-xs);
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
		font-size: var(--text-base);
		font-family: inherit;
		text-align: left;
		border-radius: var(--radius-sm);
		cursor: pointer;
		transition: background var(--transition-fast);
	}

	.list-item:hover {
		background: color-mix(in srgb, var(--text) 4%, transparent);
	}

	.list-item.active {
		background: color-mix(in srgb, var(--text) 8%, transparent);
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
		font-size: var(--text-sm);
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
		font-size: var(--text-xs);
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
		font-size: var(--text-xs);
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

	.message.success {
		background: rgba(74, 222, 128, 0.1);
	}

	.empty {
		padding: 32px;
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}
</style>
