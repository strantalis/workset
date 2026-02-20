<script lang="ts">
	import type { SkillInfo } from '../../../api/skills';
	import Button from '../../ui/Button.svelte';

	interface Props {
		skillsCount: number;
		isNew: boolean;
		selectedSkill: SkillInfo | null;
		editing: boolean;
		loading: boolean;
		formDirName: string;
		formContent: string;
		formScope: 'global' | 'project';
		formTool: string;
		syncTargets: Record<string, boolean>;
		allTools: readonly string[];
		onCloseDetail: () => void;
		onCancelEdit: () => void;
		onSave: () => void;
		onDelete: () => void;
		onSync: () => void;
		onSyncAll: () => void;
		onStartEdit: () => void;
		onFormDirNameChange: (value: string) => void;
		onFormContentChange: (value: string) => void;
		onFormScopeChange: (value: 'global' | 'project') => void;
		onFormToolChange: (value: string) => void;
		onSyncTargetChange: (tool: string, checked: boolean) => void;
		availableToolsForSyncCount: number;
	}

	const {
		skillsCount,
		isNew,
		selectedSkill,
		editing,
		loading,
		formDirName,
		formContent,
		formScope,
		formTool,
		syncTargets,
		allTools,
		onCloseDetail,
		onCancelEdit,
		onSave,
		onDelete,
		onSync,
		onSyncAll,
		onStartEdit,
		onFormDirNameChange,
		onFormContentChange,
		onFormScopeChange,
		onFormToolChange,
		onSyncTargetChange,
		availableToolsForSyncCount,
	}: Props = $props();
</script>

{#if isNew}
	<div class="detail">
		<div class="detail-header">
			<span>New Skill</span>
			<button class="close-btn" type="button" onclick={onCloseDetail} title="Close">
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
			<label class="ws-field">
				<span>Directory name</span>
				<input
					class="ws-field-input"
					type="text"
					value={formDirName}
					oninput={(event) => onFormDirNameChange((event.currentTarget as HTMLInputElement).value)}
					placeholder="my-skill"
					autocapitalize="off"
					autocorrect="off"
					spellcheck="false"
				/>
			</label>
			<label class="ws-field">
				<span>Scope</span>
				<select
					class="ws-field-select"
					value={formScope}
					onchange={(event) =>
						onFormScopeChange(
							(event.currentTarget as HTMLSelectElement).value as 'global' | 'project',
						)}
				>
					<option value="global">Global</option>
					<option value="project">Project</option>
				</select>
			</label>
			<label class="ws-field">
				<span>Tool</span>
				<select
					class="ws-field-select"
					value={formTool}
					onchange={(event) => onFormToolChange((event.currentTarget as HTMLSelectElement).value)}
				>
					{#each allTools as tool (tool)}
						{#if formScope === 'global' && tool === 'copilot'}
							<!-- copilot has no global dir -->
						{:else}
							<option value={tool}>{tool}</option>
						{/if}
					{/each}
				</select>
			</label>
			<label class="ws-field">
				<span>SKILL.md content</span>
				<textarea
					value={formContent}
					oninput={(event) =>
						onFormContentChange((event.currentTarget as HTMLTextAreaElement).value)}
					rows="10"
					spellcheck="false"
					class="content-editor"
				></textarea>
			</label>
		</div>
		<div class="actions">
			<div class="spacer"></div>
			<Button variant="ghost" onclick={onCancelEdit} disabled={loading}>Cancel</Button>
			<Button variant="primary" onclick={onSave} disabled={loading}>
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
			<button class="close-btn" type="button" onclick={onCloseDetail} title="Close">
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
				{#each allTools as tool (tool)}
					{#if selectedSkill.scope === 'global' && tool === 'copilot'}
						<!-- copilot has no global dir -->
					{:else}
						<label class="sync-tool">
							<input
								type="checkbox"
								checked={syncTargets[tool]}
								disabled={selectedSkill.tools.includes(tool)}
								onchange={(event) =>
									onSyncTargetChange(tool, (event.currentTarget as HTMLInputElement).checked)}
							/>
							<span class:synced={selectedSkill.tools.includes(tool)}>{tool}</span>
						</label>
					{/if}
				{/each}
			</div>
			<Button variant="ghost" size="sm" onclick={onSync} disabled={loading}>Sync</Button>
			<Button
				variant="primary"
				size="sm"
				onclick={onSyncAll}
				disabled={loading || availableToolsForSyncCount === 0}
			>
				Sync All
			</Button>
		</div>

		{#if editing}
			<label class="ws-field">
				<span>SKILL.md content</span>
				<textarea
					value={formContent}
					oninput={(event) =>
						onFormContentChange((event.currentTarget as HTMLTextAreaElement).value)}
					rows="12"
					spellcheck="false"
					class="content-editor"
				></textarea>
			</label>
			<div class="actions">
				<Button variant="danger" onclick={onDelete} disabled={loading}>Delete</Button>
				<div class="spacer"></div>
				<Button variant="ghost" onclick={onCancelEdit} disabled={loading}>Cancel</Button>
				<Button variant="primary" onclick={onSave} disabled={loading}>
					{loading ? 'Saving...' : 'Save'}
				</Button>
			</div>
		{:else}
			<div class="content-preview">{formContent}</div>
			<div class="actions">
				<Button variant="danger" onclick={onDelete} disabled={loading}>Delete</Button>
				<div class="spacer"></div>
				<Button variant="ghost" onclick={onStartEdit}>Edit</Button>
			</div>
		{/if}
	</div>
{:else if skillsCount > 0}
	<div class="hint ws-hint">Select a skill to view, or click "+ New" to create one.</div>
{/if}

<style>
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
		font-size: var(--text-md);
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
		font-size: var(--text-sm);
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
		background: color-mix(in srgb, var(--text) 8%, transparent);
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
		font-size: var(--text-sm);
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
		font-size: var(--text-sm);
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

	.ws-field-input,
	.ws-field-select {
		background: var(--panel-strong);
		padding: 10px var(--space-3);
		font-family: inherit;
	}

	.content-editor {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 10px var(--space-3);
		font-size: var(--text-mono-base);
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
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 10px var(--space-3);
		font-size: var(--text-mono-base);
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

	.hint {
		padding: var(--space-4);
		text-align: center;
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}
</style>
