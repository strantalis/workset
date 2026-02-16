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
		const onGroupModeChange = vi.fn<(groupMode: 'all' | 'template' | 'repo' | 'active') => void>();
		const { getByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				onGroupModeChange,
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Template' }));

		expect(onGroupModeChange).toHaveBeenCalledWith('template');
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

	test('keeps all-mode order stable instead of activity-sorting', async () => {
		const { container, getByRole, getAllByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				worksets: [
					buildWorkset({
						id: 'ws-new',
						label: 'A-workspace',
						lastActiveTs: Date.now() - 1000,
					}),
					buildWorkset({
						id: 'ws-old',
						label: 'B-workspace',
						lastActiveTs: Date.now(),
					}),
				],
			},
		});

		await fireEvent.click(getByRole('button', { name: 'All' }));
		const allModeTitles = getAllByRole('heading', { level: 3 }).map((node) => node.textContent);
		expect(allModeTitles).toEqual(['A-workspace', 'B-workspace']);

		await fireEvent.click(getByRole('button', { name: 'Active' }));
		expect(container.querySelectorAll('.group')[0]?.textContent).toContain('Today');
		const activeModeTitles = getAllByRole('heading', { level: 3 }).map((node) => node.textContent);
		expect(activeModeTitles).toEqual(['B-workspace', 'A-workspace']);
	});

	test('renders pinned worksets under a Pinned group on all mode', async () => {
		const { getByRole, getAllByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				worksets: [
					buildWorkset({
						id: 'ws-pinned',
						label: 'pinned-workspace',
						pinned: true,
						lastActiveTs: Date.now() - 1000,
					}),
					buildWorkset({
						id: 'ws-unpinned',
						label: 'regular-workspace',
						pinned: false,
						lastActiveTs: Date.now() - 2000,
					}),
				],
			},
		});

		await fireEvent.click(getByRole('button', { name: 'All' }));

		const groupHeaders = getAllByRole('heading', { level: 2 }).map((node) => node.textContent);
		expect(groupHeaders).toContain('Pinned');
		expect(groupHeaders).toContain('Unpinned');

		const allModeTitles = getAllByRole('heading', { level: 3 }).map((node) => node.textContent);
		expect(allModeTitles).toEqual(['pinned-workspace', 'regular-workspace']);
	});

	test('reacts to prop updates after a workspace becomes pinned', async () => {
		const initialWorkset = buildWorkset({
			id: 'ws-pin',
			label: 'pin-me',
			pinned: false,
		});
		const { getByRole, queryByRole, rerender } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				worksets: [initialWorkset],
			},
		});

		await fireEvent.click(getByRole('button', { name: 'All' }));
		expect(queryByRole('heading', { level: 2, name: 'Pinned' })).toBeNull();

		await rerender({
			...baseProps(vi.fn()),
			worksets: [{ ...initialWorkset, pinned: true }],
		});

		expect(getByRole('heading', { level: 2, name: 'Pinned' })).toBeTruthy();
	});

	test('sorts template groups alphabetically without heuristic labels', async () => {
		const { getByRole, getAllByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				worksets: [
					buildWorkset({
						id: 'ws-zeta',
						label: 'zeta',
						template: 'Unassigned',
					}),
					buildWorkset({
						id: 'ws-alpha',
						label: 'alpha',
						template: 'Unassigned',
					}),
				],
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Template' }));
		getByRole('heading', { level: 2, name: /Unassigned/i });
		const templateModeTitles = getAllByRole('heading', { level: 3 }).map(
			(node) => node.textContent,
		);
		expect(templateModeTitles).toEqual(['alpha', 'zeta']);
	});

	test('groups worksets with no repos under No Repos', async () => {
		const { getByRole, getAllByRole } = render(WorksetHubView, {
			props: {
				...baseProps(vi.fn()),
				worksets: [
					buildWorkset({
						id: 'ws-empty',
						label: 'empty-workspace',
						repos: [],
					}),
					buildWorkset({
						id: 'ws-linked',
						label: 'linked-workspace',
						repos: ['repo-a'],
					}),
				],
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Repo' }));
		const groupHeaders = getAllByRole('heading', { level: 2 }).map((node) => node.textContent);
		expect(groupHeaders).toContain('No Repos');
		const groupItems = getAllByRole('heading', { level: 3 }).map((node) => node.textContent);
		expect(groupItems).toEqual(expect.arrayContaining(['linked-workspace', 'empty-workspace']));
		expect(getByRole('heading', { level: 3, name: 'empty-workspace' })).toBeTruthy();
	});
});
