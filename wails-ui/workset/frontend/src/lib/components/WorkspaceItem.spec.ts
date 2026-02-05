import { describe, test, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import WorkspaceItem from './WorkspaceItem.svelte';
import type { Workspace, Repo } from '../types';

const buildRepo = (name: string, overrides: Partial<Repo> = {}): Repo => ({
	id: `ws::${name}`,
	name,
	path: `/tmp/${name}`,
	dirty: false,
	missing: false,
	diff: { added: 0, removed: 0 },
	files: [],
	...overrides,
});

const buildWorkspace = (overrides: Partial<Workspace> = {}): Workspace => ({
	id: 'ws',
	name: 'ws',
	path: '/tmp/ws',
	archived: false,
	repos: [buildRepo('repo')],
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: new Date().toISOString(),
	...overrides,
});

const baseProps = (
	workspace: Workspace,
	onManageRepo: (repoId: string, action: 'remove') => void,
) => ({
	workspace,
	isActive: false,
	isPinned: false,
	onSelectWorkspace: vi.fn(),
	onSelectRepo: vi.fn(),
	onAddRepo: vi.fn(),
	onManageWorkspace: vi.fn(),
	onManageRepo,
	onTogglePin: vi.fn(),
	onSetColor: vi.fn(),
	onDragStart: vi.fn(),
	onDragEnd: vi.fn(),
	onDrop: vi.fn(),
	onToggleExpanded: vi.fn(),
});

describe('WorkspaceItem', () => {
	afterEach(() => {
		cleanup();
	});

	test('single repo shows repo actions and removal uses repo name', async () => {
		const onManageRepo = vi.fn<(repoId: string, action: 'remove') => void>();
		const repo = buildRepo('platform', { id: 'ws::platform' });
		const workspace = buildWorkspace({ repos: [repo] });

		const { container } = render(WorkspaceItem, {
			props: baseProps(workspace, onManageRepo),
		});

		const trigger = container.querySelector('.menu-trigger-small');
		expect(trigger).toBeInTheDocument();
		await fireEvent.click(trigger!);

		const removeButton = document.body.querySelector('.dropdown-menu button.danger');
		expect(removeButton).toBeInTheDocument();
		await fireEvent.click(removeButton!);

		expect(onManageRepo).toHaveBeenCalledWith('platform', 'remove');
	});

	test('multi repo removal uses repo name', async () => {
		const onManageRepo = vi.fn<(repoId: string, action: 'remove') => void>();
		const repo = buildRepo('workset', { id: 'ws::workset' });
		const workspace = buildWorkspace({
			repos: [repo, buildRepo('other')],
			expanded: true,
		});

		const { container } = render(WorkspaceItem, {
			props: baseProps(workspace, onManageRepo),
		});

		const trigger = container.querySelector('.menu-trigger-small');
		expect(trigger).toBeInTheDocument();
		await fireEvent.click(trigger!);

		const removeButton = document.body.querySelector('.dropdown-menu button.danger');
		expect(removeButton).toBeInTheDocument();
		await fireEvent.click(removeButton!);

		expect(onManageRepo).toHaveBeenCalledWith('workset', 'remove');
	});

	test('drop handler stops propagation', async () => {
		const onDrop = vi.fn();
		const workspace = buildWorkspace();
		const onManageRepo = vi.fn<(repoId: string, action: 'remove') => void>();
		const { container } = render(WorkspaceItem, {
			props: {
				...baseProps(workspace, onManageRepo),
				onDrop,
			},
		});

		const item = container.querySelector('.workspace-item');
		expect(item).toBeInTheDocument();

		const dropEvent = new Event('drop', { bubbles: true, cancelable: true });
		const stopSpy = vi.spyOn(dropEvent, 'stopPropagation');

		await fireEvent(item!, dropEvent);

		expect(stopSpy).toHaveBeenCalled();
		expect(onDrop).toHaveBeenCalledWith('ws');
	});
});
