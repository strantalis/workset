import { afterEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import TerminalCockpitView from './TerminalCockpitView.svelte';
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

describe('TerminalCockpitView', () => {
	afterEach(() => {
		cleanup();
	});

	test('routes file-system add button to onAddRepo', async () => {
		const onAddRepo = vi.fn<(workspaceId: string) => void>();
		const { container } = render(TerminalCockpitView, {
			props: {
				workspace: buildWorkspace(),
				onAddRepo,
			},
		});

		const addButton = container.querySelector('button.section-action');
		expect(addButton).toBeInTheDocument();
		await fireEvent.click(addButton!);

		expect(onAddRepo).toHaveBeenCalledWith('ws-1');
	});
});
