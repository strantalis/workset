import { afterEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import WorksetHubView from './WorksetHubView.svelte';
import type { WorksetSummary } from '../../view-models/worksetViewModel';

const buildWorkset = (overrides: Partial<WorksetSummary> = {}): WorksetSummary => ({
	id: 'ws-1',
	label: 'workspace-one',
	description: 'workspace',
	template: 'Library',
	repos: ['repo-one'],
	branch: 'main',
	repoCount: 1,
	dirtyCount: 0,
	openPrs: 0,
	linesAdded: 0,
	linesRemoved: 0,
	lastActive: 'just now',
	lastActiveTs: Date.now(),
	health: ['clean'],
	pinned: false,
	archived: false,
	...overrides,
});

const baseProps = (onAddRepo: (workspaceId: string) => void) => ({
	worksets: [buildWorkset()],
	shortcutMap: new Map<string, number>(),
	activeWorkspaceId: null,
	onSelectWorkspace: vi.fn(),
	onCreateWorkspace: vi.fn(),
	onAddRepo,
	onTogglePin: vi.fn(),
	onToggleArchived: vi.fn(),
	onOpenPopout: vi.fn(),
	onClosePopout: vi.fn(),
	isWorkspacePoppedOut: vi.fn(() => false),
});

describe('WorksetHubView', () => {
	afterEach(() => {
		cleanup();
	});

	test('opens add-repo action from grid menu', async () => {
		const onAddRepo = vi.fn<(workspaceId: string) => void>();
		const { getByRole } = render(WorksetHubView, {
			props: baseProps(onAddRepo),
		});

		await fireEvent.click(getByRole('button', { name: 'Workset actions' }));
		await fireEvent.click(getByRole('button', { name: 'Add repo' }));

		expect(onAddRepo).toHaveBeenCalledWith('ws-1');
	});

	test('opens add-repo action from list menu', async () => {
		const onAddRepo = vi.fn<(workspaceId: string) => void>();
		const { getByRole } = render(WorksetHubView, {
			props: baseProps(onAddRepo),
		});

		await fireEvent.click(getByRole('button', { name: 'List layout' }));
		await fireEvent.click(getByRole('button', { name: 'Workset actions' }));
		await fireEvent.click(getByRole('button', { name: 'Add repo' }));

		expect(onAddRepo).toHaveBeenCalledWith('ws-1');
	});
});
