import { afterEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import CommandCenterView from './CommandCenterView.svelte';
import type { Workspace } from '../../types';

const buildWorkspace = (overrides: Partial<Workspace> = {}): Workspace => ({
	id: 'ws-1',
	name: 'workspace-one',
	path: '/tmp/workspace-one',
	archived: false,
	repos: [],
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: new Date().toISOString(),
	...overrides,
});

describe('CommandCenterView', () => {
	afterEach(() => {
		cleanup();
	});

	test('shows empty state with Create Workspace button when no workspace selected', () => {
		const onCreateWorkspace = vi.fn();
		const { container } = render(CommandCenterView, {
			props: {
				workspaces: [],
				activeWorkspaceId: null,
				onCreateWorkspace,
			},
		});

		expect(container.querySelector('h2')?.textContent).toBe('No workspace selected');
		const ctaButton = container.querySelector('.empty-card .cta');
		expect(ctaButton).toBeInTheDocument();
		expect(ctaButton?.textContent).toContain('Create Workspace');
	});

	test('shows empty state with Add Repo button when workspace has no repos', () => {
		const onAddRepo = vi.fn();
		const workspace = buildWorkspace({ repos: [] });
		const { container } = render(CommandCenterView, {
			props: {
				workspaces: [workspace],
				activeWorkspaceId: 'ws-1',
				onAddRepo,
			},
		});

		expect(container.querySelector('h2')?.textContent).toBe('No repos linked');
		const ctaButton = container.querySelector('.empty-card .cta');
		expect(ctaButton).toBeInTheDocument();
		expect(ctaButton?.textContent).toContain('Add Repo');
	});

	test('calls onAddRepo when Add Repo button is clicked in empty state', async () => {
		const onAddRepo = vi.fn();
		const workspace = buildWorkspace({ repos: [] });
		const { container } = render(CommandCenterView, {
			props: {
				workspaces: [workspace],
				activeWorkspaceId: 'ws-1',
				onAddRepo,
			},
		});

		const addButton = container.querySelector('.empty-card .cta');
		await fireEvent.click(addButton!);

		expect(onAddRepo).toHaveBeenCalledWith('ws-1');
	});

	test('shows repo panel with Add Repo button in toolbar when workspace has repos', () => {
		const onAddRepo = vi.fn();
		const workspace = buildWorkspace({
			repos: [
				{
					id: 'repo-1',
					name: 'test-repo',
					path: '/tmp/test-repo',
					dirty: false,
					missing: false,
					currentBranch: 'main',
					defaultBranch: 'main',
					files: [],
					diff: { added: 0, removed: 0 },
					trackedPullRequest: undefined,
				},
			],
		});
		const { container } = render(CommandCenterView, {
			props: {
				workspaces: [workspace],
				activeWorkspaceId: 'ws-1',
				onAddRepo,
			},
		});

		expect(container.querySelector('.repo-panel')).toBeInTheDocument();
		const toolbarButton = container.querySelector('.panel-toolbar .add-repo-btn');
		expect(toolbarButton).toBeInTheDocument();
		expect(toolbarButton?.textContent).toContain('Add Repo');
	});

	test('calls onAddRepo when toolbar Add Repo button is clicked', async () => {
		const onAddRepo = vi.fn();
		const workspace = buildWorkspace({
			repos: [
				{
					id: 'repo-1',
					name: 'test-repo',
					path: '/tmp/test-repo',
					dirty: false,
					missing: false,
					currentBranch: 'main',
					defaultBranch: 'main',
					files: [],
					diff: { added: 0, removed: 0 },
					trackedPullRequest: undefined,
				},
			],
		});
		const { container } = render(CommandCenterView, {
			props: {
				workspaces: [workspace],
				activeWorkspaceId: 'ws-1',
				onAddRepo,
			},
		});

		const addButton = container.querySelector('.panel-toolbar .add-repo-btn');
		await fireEvent.click(addButton!);

		expect(onAddRepo).toHaveBeenCalledWith('ws-1');
	});

	test('shows merged PR stat and merged cleanup badge', () => {
		const workspace = buildWorkspace({
			repos: [
				{
					id: 'repo-merged',
					name: 'merged-repo',
					path: '/tmp/merged-repo',
					dirty: false,
					missing: false,
					currentBranch: 'main',
					defaultBranch: 'main',
					files: [],
					diff: { added: 0, removed: 0 },
					trackedPullRequest: {
						repo: 'merged-repo',
						number: 12,
						url: 'https://github.com/example/merged-repo/pull/12',
						title: 'Merged PR',
						state: 'closed',
						draft: false,
						merged: true,
						baseRepo: 'example/merged-repo',
						baseBranch: 'main',
						headRepo: 'example/merged-repo',
						headBranch: 'feature/merged',
					},
				},
			],
		});
		const { getByText } = render(CommandCenterView, {
			props: {
				workspaces: [workspace],
				activeWorkspaceId: 'ws-1',
			},
		});

		expect(getByText('Merged PRs')).toBeInTheDocument();
		expect(getByText('Merged PR')).toBeInTheDocument();
	});

	test('sorts repos with dirty/ahead first, then merged cleanup candidates, then clean', () => {
		const workspace = buildWorkspace({
			repos: [
				{
					id: 'repo-clean',
					name: 'clean-repo',
					path: '/tmp/clean-repo',
					dirty: false,
					missing: false,
					currentBranch: 'main',
					defaultBranch: 'main',
					ahead: 0,
					files: [],
					diff: { added: 0, removed: 0 },
				},
				{
					id: 'repo-merged',
					name: 'merged-repo',
					path: '/tmp/merged-repo',
					dirty: false,
					missing: false,
					currentBranch: 'main',
					defaultBranch: 'main',
					ahead: 0,
					files: [],
					diff: { added: 0, removed: 0 },
					trackedPullRequest: {
						repo: 'merged-repo',
						number: 22,
						url: 'https://github.com/example/merged-repo/pull/22',
						title: 'Merged PR',
						state: 'closed',
						draft: false,
						merged: true,
						baseRepo: 'example/merged-repo',
						baseBranch: 'main',
						headRepo: 'example/merged-repo',
						headBranch: 'feature/merged',
					},
				},
				{
					id: 'repo-dirty',
					name: 'dirty-repo',
					path: '/tmp/dirty-repo',
					dirty: true,
					missing: false,
					currentBranch: 'main',
					defaultBranch: 'main',
					ahead: 0,
					files: [{ path: 'README.md', added: 1, removed: 0, hunks: [] }],
					diff: { added: 1, removed: 0 },
				},
			],
		});
		const { container } = render(CommandCenterView, {
			props: {
				workspaces: [workspace],
				activeWorkspaceId: 'ws-1',
			},
		});

		const headings = Array.from(container.querySelectorAll('.repo-card h3')).map((node) =>
			node.textContent?.trim(),
		);
		expect(headings).toEqual(['dirty-repo', 'merged-repo', 'clean-repo']);
	});
});
