import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';
import ExplorerPanel from './ExplorerPanel.svelte';
import type { Workspace } from '../../types';
import {
	buildShortcutMap,
	mapWorkspacesToExplorerWorksets,
} from '../../view-models/worksetViewModel';

const buildWorkspace = (overrides: Partial<Workspace> = {}): Workspace => ({
	id: 'ws-1',
	name: 'Thread One',
	path: '/tmp/ws-1',
	workset: 'Alpha',
	worksetKey: 'workset:alpha',
	worksetLabel: 'Alpha',
	placeholder: false,
	archived: false,
	repos: [],
	pinned: false,
	pinOrder: 0,
	expanded: true,
	lastUsed: new Date().toISOString(),
	...overrides,
});

const buildGroupedWorksets = (workspaces: Workspace[]) =>
	mapWorkspacesToExplorerWorksets(workspaces, buildShortcutMap(workspaces));

const renderExplorerPanel = (workspaces: Workspace[], props: Record<string, unknown>) =>
	render(ExplorerPanel, {
		props: {
			activeWorkspaceId: null,
			groupedWorksets: buildGroupedWorksets(workspaces),
			onSelectWorkspace: vi.fn<(workspaceId: string) => void>(),
			...props,
		},
	});

describe('ExplorerPanel', () => {
	beforeEach(() => {
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
			configurable: true,
			value: vi.fn(),
		});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	test('lets users select an empty workset and create a thread', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();
		const onCreateThread = vi.fn<(worksetId: string) => void>();

		const workspaces = [
			buildWorkspace({
				id: 'thread-alpha',
				name: 'Alpha Thread',
				workset: 'Alpha',
				worksetKey: 'workset:alpha',
				worksetLabel: 'Alpha',
			}),
			buildWorkspace({
				id: 'placeholder-beta',
				name: 'Beta Placeholder',
				workset: 'Beta',
				worksetKey: 'workset:beta',
				worksetLabel: 'Beta',
				placeholder: true,
			}),
		];

		const { getByRole } = renderExplorerPanel(workspaces, {
			activeWorkspaceId: 'thread-alpha',
			onSelectWorkspace,
			onCreateThread,
		});

		await fireEvent.click(getByRole('button', { name: 'Switch workset' }));
		await fireEvent.click(getByRole('button', { name: /Beta/i }));

		expect(onSelectWorkspace).not.toHaveBeenCalled();

		const createThreadButton = getByRole('button', { name: 'Create thread in Beta' });
		await fireEvent.click(createThreadButton);
		expect(onCreateThread).toHaveBeenCalledWith('workset:beta');
	});

	test('keeps selection on the same workset when active thread is removed', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();

		const initialWorkspaces = [
			buildWorkspace({
				id: 'thread-alpha',
				name: 'Alpha Thread',
				workset: 'Alpha',
				worksetKey: 'workset:alpha',
				worksetLabel: 'Alpha',
			}),
			buildWorkspace({
				id: 'thread-beta-1',
				name: 'Beta Thread One',
				workset: 'Beta',
				worksetKey: 'workset:beta',
				worksetLabel: 'Beta',
			}),
			buildWorkspace({
				id: 'thread-beta-2',
				name: 'Beta Thread Two',
				workset: 'Beta',
				worksetKey: 'workset:beta',
				worksetLabel: 'Beta',
			}),
		];

		const { rerender } = renderExplorerPanel(initialWorkspaces, {
			activeWorkspaceId: 'thread-beta-2',
			onSelectWorkspace,
		});

		const nextWorkspaces = [
			buildWorkspace({
				id: 'thread-alpha',
				name: 'Alpha Thread',
				workset: 'Alpha',
				worksetKey: 'workset:alpha',
				worksetLabel: 'Alpha',
			}),
			buildWorkspace({
				id: 'thread-beta-1',
				name: 'Beta Thread One',
				workset: 'Beta',
				worksetKey: 'workset:beta',
				worksetLabel: 'Beta',
			}),
		];

		await rerender({
			groupedWorksets: buildGroupedWorksets(nextWorkspaces),
			activeWorkspaceId: null,
			onSelectWorkspace,
		});

		await waitFor(() => expect(onSelectWorkspace).toHaveBeenCalledWith('thread-beta-1'));
	});

	test('does not auto-switch to another workset when selected workset is empty', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();

		const initialWorkspaces = [
			buildWorkspace({
				id: 'thread-alpha',
				name: 'Alpha Thread',
				workset: 'Alpha',
				worksetKey: 'workset:alpha',
				worksetLabel: 'Alpha',
			}),
			buildWorkspace({
				id: 'thread-beta',
				name: 'Beta Thread',
				workset: 'Beta',
				worksetKey: 'workset:beta',
				worksetLabel: 'Beta',
			}),
		];

		const { rerender } = renderExplorerPanel(initialWorkspaces, {
			activeWorkspaceId: 'thread-beta',
			onSelectWorkspace,
		});

		const nextWorkspaces = [
			buildWorkspace({
				id: 'thread-alpha',
				name: 'Alpha Thread',
				workset: 'Alpha',
				worksetKey: 'workset:alpha',
				worksetLabel: 'Alpha',
			}),
			buildWorkspace({
				id: 'placeholder-beta',
				name: 'Beta Placeholder',
				workset: 'Beta',
				worksetKey: 'workset:beta',
				worksetLabel: 'Beta',
				placeholder: true,
			}),
		];

		await rerender({
			groupedWorksets: buildGroupedWorksets(nextWorkspaces),
			activeWorkspaceId: null,
			onSelectWorkspace,
		});

		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(onSelectWorkspace).not.toHaveBeenCalled();
	});

	test('does not switch worksets while selected workset is temporarily missing', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();

		const initialWorkspaces = [
			buildWorkspace({
				id: 'thread-alpha',
				name: 'Alpha Thread',
				workset: 'Alpha',
				worksetKey: 'workset:alpha',
				worksetLabel: 'Alpha',
			}),
			buildWorkspace({
				id: 'thread-beta',
				name: 'Beta Thread',
				workset: 'Beta',
				worksetKey: 'workset:beta',
				worksetLabel: 'Beta',
			}),
		];

		const { rerender } = renderExplorerPanel(initialWorkspaces, {
			activeWorkspaceId: 'thread-beta',
			onSelectWorkspace,
		});

		const nextWorkspaces = [
			buildWorkspace({
				id: 'thread-alpha',
				name: 'Alpha Thread',
				workset: 'Alpha',
				worksetKey: 'workset:alpha',
				worksetLabel: 'Alpha',
			}),
		];

		await rerender({
			groupedWorksets: buildGroupedWorksets(nextWorkspaces),
			activeWorkspaceId: null,
			onSelectWorkspace,
		});

		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(onSelectWorkspace).not.toHaveBeenCalled();
	});

	test('allows add repo for worksets without threads', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();
		const onAddRepo = vi.fn<(worksetId: string) => void>();

		const workspaces = [
			buildWorkspace({
				id: 'thread-alpha',
				name: 'Alpha Thread',
				workset: 'Alpha',
				worksetKey: 'workset:alpha',
				worksetLabel: 'Alpha',
			}),
			buildWorkspace({
				id: 'placeholder-beta',
				name: 'Beta Placeholder',
				workset: 'Beta',
				worksetKey: 'workset:beta',
				worksetLabel: 'Beta',
				placeholder: true,
			}),
		];

		const { getByRole } = renderExplorerPanel(workspaces, {
			activeWorkspaceId: 'thread-alpha',
			onSelectWorkspace,
			onAddRepo,
		});

		await fireEvent.click(getByRole('button', { name: 'Switch workset' }));
		await fireEvent.click(getByRole('button', { name: /Beta/i }));

		const addRepoButton = getByRole('button', { name: 'Add repo to Beta' });
		await fireEvent.click(addRepoButton);
		expect(onAddRepo).toHaveBeenCalledWith('workset:beta');
	});

	test('reveals the remove button only for the hovered thread', async () => {
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const beta = buildWorkspace({
			id: 'thread-beta',
			name: 'beta',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const { getByRole } = renderExplorerPanel([alpha, beta], {
			activeWorkspaceId: alpha.id,
			onSelectWorkspace: vi.fn(),
		});

		const alphaRow = getByRole('button', { name: 'alpha' }).closest('.thread-row');
		const betaRow = getByRole('button', { name: 'beta' }).closest('.thread-row');
		const alphaRemove = getByRole('button', { name: 'Remove thread alpha' });
		const betaRemove = getByRole('button', { name: 'Remove thread beta' });
		expect(alphaRow).not.toBeNull();
		expect(betaRow).not.toBeNull();
		expect(alphaRemove).not.toHaveClass('visible');
		expect(betaRemove).not.toHaveClass('visible');

		await fireEvent.mouseEnter(alphaRow!);
		expect(alphaRemove).toHaveClass('visible');
		expect(betaRemove).not.toHaveClass('visible');

		await fireEvent.mouseLeave(alphaRow!);
		await fireEvent.mouseEnter(betaRow!);
		expect(alphaRemove).not.toHaveClass('visible');
		expect(betaRemove).toHaveClass('visible');
	});

	test('clears the hovered remove button when switching threads', async () => {
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const beta = buildWorkspace({
			id: 'thread-beta',
			name: 'beta',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();
		const { getByRole } = renderExplorerPanel([alpha, beta], {
			activeWorkspaceId: alpha.id,
			onSelectWorkspace,
		});

		const alphaRemove = getByRole('button', { name: 'Remove thread alpha' });
		const alphaRow = getByRole('button', { name: 'alpha' }).closest('.thread-row');
		expect(alphaRow).not.toBeNull();

		await fireEvent.mouseEnter(alphaRow!);
		expect(alphaRemove).toHaveClass('visible');

		await fireEvent.click(getByRole('button', { name: 'beta' }));
		expect(onSelectWorkspace).toHaveBeenCalledWith(beta.id);
		expect(alphaRemove).not.toHaveClass('visible');
	});

	test('keeps remove button keyboard reachable without hover', async () => {
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const { getByRole } = renderExplorerPanel([alpha], {
			activeWorkspaceId: alpha.id,
			onSelectWorkspace: vi.fn(),
		});

		const removeButton = getByRole('button', { name: 'Remove thread alpha' });
		expect(removeButton.tabIndex).toBe(0);
		removeButton.focus();
		expect(removeButton).toHaveFocus();
	});

	test('renders animated work badge for threads with active terminal IO', () => {
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const beta = buildWorkspace({
			id: 'thread-beta',
			name: 'beta',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const { container, getByText } = renderExplorerPanel([alpha, beta], {
			activeWorkspaceId: alpha.id,
			activeTerminalWorkspaceIds: [beta.id],
			onSelectWorkspace: vi.fn(),
		});

		const workBadge = container.querySelector<HTMLElement>('.thread-live-indicator');
		expect(workBadge).not.toBeNull();
		expect(workBadge).toHaveAttribute('title', 'Work in progress');
		expect(getByText('beta').parentElement).toContainElement(workBadge);
	});

	test('does not render Live badge when no thread has active terminal IO', () => {
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const beta = buildWorkspace({
			id: 'thread-beta',
			name: 'beta',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const { queryByText } = renderExplorerPanel([alpha, beta], {
			activeWorkspaceId: alpha.id,
			activeTerminalWorkspaceIds: [],
			onSelectWorkspace: vi.fn(),
		});

		expect(queryByText('Live')).toBeNull();
	});

	test('opens files from the footer nav and marks the icon active', async () => {
		const onOpenFiles = vi.fn();
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});

		const { getByRole } = renderExplorerPanel([alpha], {
			activeWorkspaceId: alpha.id,
			filesActive: true,
			onSelectWorkspace: vi.fn(),
			onOpenFiles,
		});

		const filesButton = getByRole('button', { name: 'Toggle code pane' });
		expect(filesButton).toHaveClass('active');

		await fireEvent.click(filesButton);
		expect(onOpenFiles).toHaveBeenCalledTimes(1);
	});

	test('opens skills from the footer pane toggles and marks the control active', async () => {
		const onOpenSkills = vi.fn();
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});

		const { getByRole } = renderExplorerPanel([alpha], {
			activeWorkspaceId: alpha.id,
			activeView: 'skill-registry',
			onSelectWorkspace: vi.fn(),
			onOpenSkills,
		});

		const skillsButton = getByRole('button', { name: 'Toggle skills view' });
		expect(skillsButton).toHaveClass('active');

		await fireEvent.click(skillsButton);
		expect(onOpenSkills).toHaveBeenCalledTimes(1);
	});

	test('renders review feedback badge for threads with tracked PR feedback', () => {
		const workspace = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
			repos: [
				{
					id: 'repo-1',
					name: 'repo-one',
					path: '/tmp/ws-1/repo-one',
					defaultBranch: 'main',
					dirty: false,
					missing: false,
					diff: { added: 0, removed: 0 },
					files: [],
					trackedPullRequest: {
						repo: 'repo-one',
						number: 198,
						url: 'https://github.com/strantalis/workset/pull/198',
						title: 'fix hover delete',
						state: 'open',
						draft: false,
						baseRepo: 'strantalis/workset',
						baseBranch: 'main',
						headRepo: 'strantalis/workset',
						headBranch: 'fix-buggy-hover-delete',
						commentsCount: 1,
						reviewCommentsCount: 1,
					},
				},
			],
		});
		const { container } = renderExplorerPanel([workspace], {
			activeWorkspaceId: workspace.id,
			onSelectWorkspace: vi.fn(),
		});

		const badge = container.querySelector('.thread-feedback-indicator');
		expect(badge).not.toBeNull();
		expect(badge?.textContent).toContain('1');
	});
});
