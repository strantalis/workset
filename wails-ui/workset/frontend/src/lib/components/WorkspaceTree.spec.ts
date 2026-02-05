import { describe, expect, it, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import { deriveSidebarLabelLimits, ellipsisMiddle } from '../names';
import type { Workspace } from '../types';
import WorkspaceItem from './WorkspaceItem.svelte';

const noop = vi.fn();

const baseProps = {
	isActive: false,
	isPinned: false,
	draggable: false,
	onSelectWorkspace: noop,
	onSelectRepo: noop,
	onAddRepo: noop,
	onManageWorkspace: noop,
	onManageRepo: noop,
	onTogglePin: noop,
	onSetColor: noop,
	onDragStart: noop,
	onDragEnd: noop,
	onDrop: noop,
	onToggleExpanded: noop,
};

describe('WorkspaceItem long label layout', () => {
	it('renders middle-truncated workspace and repo labels with full titles', () => {
		const limits = deriveSidebarLabelLimits(280);
		const longWorkspaceName = 'data-security-platform-thunder-byte';
		const longRepoName = 'data-security-platform';
		const longRef = 'upstream-superlongremote/super-long-branch-name-for-tests';
		const workspace: Workspace = {
			id: 'ws-1',
			name: longWorkspaceName,
			path: '/tmp/ws-1',
			archived: false,
			pinned: false,
			pinOrder: 0,
			expanded: true,
			lastUsed: new Date().toISOString(),
			repos: [
				{
					id: 'repo-1',
					name: longRepoName,
					path: '/tmp/ws-1/data-security-platform',
					remote: 'upstream-superlongremote',
					defaultBranch: 'super-long-branch-name-for-tests',
					dirty: false,
					missing: false,
					diff: { added: 0, removed: 0 },
					files: [],
				},
			],
		};

		const { container } = render(WorkspaceItem, {
			props: {
				...baseProps,
				workspace,
			},
		});

		const workspaceButton = container.querySelector('.workspace-info');
		const repoButton = container.querySelector('.repo-info-single');
		const branch = container.querySelector('.branch');

		expect(workspaceButton).toHaveAttribute('title', longWorkspaceName);
		expect(workspaceButton).toHaveTextContent(ellipsisMiddle(longWorkspaceName, limits.workspace));
		expect(repoButton).toHaveAttribute('title', longRepoName);
		expect(repoButton).toHaveTextContent(ellipsisMiddle(longRepoName, limits.repo));
		expect(branch).toHaveAttribute('title', longRef);
		expect(branch).toHaveTextContent(ellipsisMiddle(longRef, limits.ref));
	});

	it('keeps short names unchanged', () => {
		const workspace: Workspace = {
			id: 'ws-2',
			name: 'acme',
			path: '/tmp/ws-2',
			archived: false,
			pinned: false,
			pinOrder: 0,
			expanded: true,
			lastUsed: new Date().toISOString(),
			repos: [
				{
					id: 'repo-2',
					name: 'backend',
					path: '/tmp/ws-2/backend',
					remote: 'origin',
					defaultBranch: 'main',
					dirty: false,
					missing: false,
					diff: { added: 0, removed: 0 },
					files: [],
				},
			],
		};

		const { container } = render(WorkspaceItem, {
			props: {
				...baseProps,
				workspace,
			},
		});

		const workspaceButton = container.querySelector('.workspace-info');
		const repoButton = container.querySelector('.repo-info-single');
		const branch = container.querySelector('.branch');

		expect(workspaceButton).toHaveTextContent('acme');
		expect(repoButton).toHaveTextContent('backend');
		expect(branch).toHaveTextContent('origin/main');
	});
});
