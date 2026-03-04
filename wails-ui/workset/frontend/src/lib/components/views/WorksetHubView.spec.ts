import { afterEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import WorksetHubView from './WorksetHubView.svelte';
import type { WorksetSummary } from '../../view-models/worksetViewModel';

const buildWorkset = (overrides: Partial<WorksetSummary> = {}): WorksetSummary => ({
	id: 'ws-1',
	label: 'thread-one',
	description: 'workspace',
	workset: 'Platform Core',
	repos: ['repo-one'],
	branch: 'main',
	repoCount: 1,
	dirtyCount: 0,
	openPrs: 0,
	mergedPrs: 0,
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
	onRemoveWorkspace: vi.fn(),
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

	test('aggregates threads under one workset card', () => {
		const { getByRole, queryByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				worksets: [
					buildWorkset({ id: 'ws-1', label: 'oauth', workset: 'Platform Core' }),
					buildWorkset({ id: 'ws-2', label: 'billing', workset: 'Platform Core' }),
				],
			},
		});

		expect(getByRole('heading', { level: 3, name: 'Platform Core' })).toBeTruthy();
		expect(getByRole('button', { name: /Workset actions/i })).toBeTruthy();
		expect(queryByRole('heading', { level: 3, name: 'oauth' })).toBeNull();
		expect(queryByRole('heading', { level: 3, name: 'billing' })).toBeNull();
	});

	test('opens add-repo action from grid menu using primary thread', async () => {
		const onAddRepo = vi.fn<(workspaceId: string) => void>();
		const { getByRole } = render(WorksetHubView, {
			props: {
				...baseProps(onAddRepo),
				worksets: [
					buildWorkset({
						id: 'ws-recent',
						label: 'latest-thread',
						lastActiveTs: Date.now(),
					}),
					buildWorkset({
						id: 'ws-older',
						label: 'older-thread',
						lastActiveTs: Date.now() - 1000,
					}),
				],
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Workset actions' }));
		await fireEvent.click(getByRole('button', { name: 'Add repo' }));

		expect(onAddRepo).toHaveBeenCalledWith('ws-recent');
	});

	test('opens remove-workspace action from grid menu', async () => {
		const onRemoveWorkspace = vi.fn<(workspaceId: string) => void>();
		const { getByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				onRemoveWorkspace,
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Workset actions' }));
		await fireEvent.click(getByRole('button', { name: 'Delete workset' }));

		expect(onRemoveWorkspace).toHaveBeenCalledWith('ws-1');
	});

	test('starts in list layout when list layout prop is provided', () => {
		const { getByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				layoutMode: 'list',
			},
		});

		const listButton = getByRole('button', { name: 'List layout' });
		const gridButton = getByRole('button', { name: 'Grid layout' });

		expect(listButton).toHaveClass('active');
		expect(gridButton).not.toHaveClass('active');
	});

	test('invokes layout mode change callback when layout button is clicked', async () => {
		const onLayoutModeChange = vi.fn<(layoutMode: 'grid' | 'list') => void>();
		const { getByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				onLayoutModeChange,
			},
		});

		await fireEvent.click(getByRole('button', { name: 'List layout' }));

		expect(onLayoutModeChange).toHaveBeenCalledWith('list');
	});

	test('invokes group mode callback when group button is clicked', async () => {
		const onGroupModeChange = vi.fn<(groupMode: 'all' | 'repo' | 'active') => void>();
		const { getByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				onGroupModeChange,
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Repo' }));

		expect(onGroupModeChange).toHaveBeenCalledWith('repo');
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

	test('groups worksets with no repos under No Repos', async () => {
		const { getByRole, getAllByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				worksets: [
					buildWorkset({
						id: 'ws-empty',
						label: 'empty-workspace',
						workset: 'Infra',
						repos: [],
					}),
					buildWorkset({
						id: 'ws-linked',
						label: 'linked-workspace',
						workset: 'Platform',
						repos: ['repo-a'],
					}),
				],
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Repo' }));
		const groupHeaders = getAllByRole('heading', { level: 2 }).map((node) => node.textContent);
		expect(groupHeaders).toContain('No Repos');
		expect(getByRole('heading', { level: 3, name: 'Infra' })).toBeTruthy();
	});
});
